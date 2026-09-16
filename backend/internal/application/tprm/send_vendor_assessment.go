// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// SendAssessmentInput is what the sender chooses. The contact and the language
// default to the vendor record and the questionnaire; the owner is always the
// sender, because accepting an arbitrary user id here would assign reminders to
// someone this use case cannot check belongs to the tenant.
type SendAssessmentInput struct {
	TemplateID      uuid.UUID
	DueAt           time.Time
	ContactEmail    string
	ContactLanguage string
}

// SendVendorAssessmentUseCase sends a questionnaire to a vendor (#670,
// ADR 0004 D3/D4): it snapshots the questions, mints the first token, writes all
// of it in one transaction, and mails the link.
type SendVendorAssessmentUseCase struct{ deps AssessmentDeps }

func NewSendVendorAssessmentUseCase(deps AssessmentDeps) *SendVendorAssessmentUseCase {
	return &SendVendorAssessmentUseCase{deps: deps}
}

func (uc *SendVendorAssessmentUseCase) Execute(ctx context.Context, tenantID, actorID, vendorID uuid.UUID, in SendAssessmentInput) (*AssessmentDelivery, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}

	vendor, err := uc.deps.Vendors.GetVendor(ctx, vendorID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if vendor == nil {
		return nil, domain.NewNotFoundError("vendor", vendorID)
	}

	tmpl, err := uc.deps.Templates.GetTemplate(ctx, in.TemplateID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if tmpl == nil {
		return nil, domain.NewNotFoundError("questionnaire template", in.TemplateID)
	}

	email := strings.TrimSpace(in.ContactEmail)
	if email == "" {
		email = domain.AssetAttrString(vendor.Attributes, "contact_email")
	}
	if strings.TrimSpace(email) == "" {
		return nil, domain.NewValidationError("contact_email is required: this vendor has no contact email on record")
	}
	language := strings.TrimSpace(in.ContactLanguage)
	if language == "" {
		language = tmpl.Language
	}

	a, tok, token, err := domain.NewVendorAssessment(domain.NewVendorAssessmentInput{
		TenantID:        tenantID,
		VendorAssetID:   vendor.ID,
		OwnerUserID:     actorID,
		SentBy:          actorID,
		Template:        *tmpl,
		ContactEmail:    email,
		ContactLanguage: language,
		DueAt:           in.DueAt,
	}, uc.deps.now())
	if err != nil {
		return nil, err
	}
	if err := uc.deps.Assessments.CreateAssessment(ctx, a, tok); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	// The trail records who was asked what and by when — never the token.
	uc.deps.record(ctx, tenantID, actorID, domain.AuditActionCreate, "vendor_assessment", a.ID.String(),
		fmt.Sprintf("Sent questionnaire %q to %s (%s)", tmpl.Name, vendor.Name, a.ContactEmail),
		domain.JSONMap{
			"vendor_id":        vendor.ID.String(),
			"template_id":      tmpl.ID.String(),
			"template_version": tmpl.Version,
			"contact_email":    a.ContactEmail,
			"due_at":           a.DueAt,
		})

	return uc.deps.deliver(ctx, a, token, domain.VendorTokenReasonSend), nil
}
