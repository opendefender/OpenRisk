// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// RevokeVendorAssessmentUseCase withdraws an open questionnaire. Its link then
// answers 410 (ADR 0004 D4). A submitted or already revoked assessment cannot be
// revoked: the vendor's answers are a record.
type RevokeVendorAssessmentUseCase struct{ deps AssessmentDeps }

func NewRevokeVendorAssessmentUseCase(deps AssessmentDeps) *RevokeVendorAssessmentUseCase {
	return &RevokeVendorAssessmentUseCase{deps: deps}
}

func (uc *RevokeVendorAssessmentUseCase) Execute(ctx context.Context, tenantID, actorID, id uuid.UUID) (*domain.VendorAssessment, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	a, err := uc.deps.Assessments.GetAssessment(ctx, id, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if a == nil {
		return nil, domain.NewNotFoundError("vendor assessment", id)
	}

	now := uc.deps.now()
	if a.Status == domain.VendorAssessmentSubmitted || a.Status == domain.VendorAssessmentRevoked {
		return nil, conflict(fmt.Sprintf("this questionnaire is %s and can no longer be revoked", a.Status))
	}
	revoked, err := uc.deps.Assessments.RevokeAssessment(ctx, id, tenantID, actorID, now)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !revoked {
		return nil, conflict("this questionnaire changed state while it was being revoked — reload it")
	}

	a.Status = domain.VendorAssessmentRevoked
	a.RevokedAt = &now
	actor := actorID
	a.RevokedBy = &actor
	uc.deps.record(ctx, tenantID, actorID, domain.AuditActionRevoke, "vendor_assessment", id.String(),
		"Revoked the questionnaire link", domain.JSONMap{"status": string(domain.VendorAssessmentRevoked)})
	return a, nil
}
