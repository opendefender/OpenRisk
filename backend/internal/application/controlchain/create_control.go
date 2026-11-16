// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// CreateInput is the content of a new control. There is deliberately no field
// for the id, the tenant, the source, the confidence or the provenance: they are
// set here, and a request that carries them is decoded into a DTO that drops them.
type CreateInput struct {
	Name                       string
	Description                *string
	Objective                  *string
	OwnerID                    *uuid.UUID
	AssigneeID                 *uuid.UUID
	ReviewerID                 *uuid.UUID
	Applicability              string // empty: applicable
	ApplicabilityJustification *string
	DeclaredStatus             string // empty: not_implemented
	Tags                       []string
}

// CreateUseCase creates a control of the tenant's own library.
type CreateUseCase struct {
	repo   Repository
	owners OwnerValidator
	now    Clock
}

// NewCreateUseCase builds the use case. owners must not be nil: an owner that
// was never checked is a silent store of another tenant's user.
func NewCreateUseCase(repo Repository, owners OwnerValidator, now Clock) *CreateUseCase {
	return &CreateUseCase{repo: repo, owners: owners, now: now}
}

// Execute builds the control with a server-generated id and the user_entered
// envelope (manual_entry by the caller, default confidence), validates it, and
// stores it with its audit entry in one transaction.
//
// declared_status = implemented is refused with evidence_required: a control
// created now has no evidence, and in this slice no path attaches any to a
// user-created control (results and evidence are #1416), so it cannot be created
// as implemented. That is the rule (D-108 Q16) applied honestly, not a gap.
func (u *CreateUseCase) Execute(ctx context.Context, c Caller, in CreateInput) (*ControlView, error) {
	if err := c.authorize(PermissionCreate); err != nil {
		return nil, err
	}
	if err := checkLongTexts(in.Description, in.Objective, in.ApplicabilityJustification); err != nil {
		return nil, err
	}
	now := u.now().UTC()
	tags, err := cleanTags(in.Tags)
	if err != nil {
		return nil, err
	}
	conf, err := userConfidence()
	if err != nil {
		return nil, validation(err)
	}
	id := uuid.New()
	ctl := canonical.Control{
		Envelope: canonical.Envelope{
			ID: id, TenantID: c.TenantID, Source: canonical.SourceOpenRiskUser, SourceID: id.String(),
			CreatedAt: now, UpdatedAt: now, Confidence: conf, Provenance: userProvenance(c.UserID),
		},
		Name: in.Name, Description: blankToNil(in.Description), Objective: blankToNil(in.Objective),
		Ownership:                  canonical.Ownership{OwnerID: in.OwnerID, AssigneeID: in.AssigneeID, ReviewerID: in.ReviewerID},
		Applicability:              canonical.Applicability(defaultTo(in.Applicability, string(canonical.Applicable))),
		ApplicabilityJustification: blankToNil(in.ApplicabilityJustification),
		DeclaredStatus:             canonical.DeclaredStatus(defaultTo(in.DeclaredStatus, string(canonical.DeclaredNotImplemented))),
		Tags:                       tags,
	}
	if err := validation(ctl.Validate()); err != nil {
		return nil, err
	}
	if ctl.DeclaredStatus == canonical.DeclaredImplemented {
		return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlEvidenceRequired)
	}
	if err := validateOwners(ctx, u.owners, c.TenantID, ownershipPatchOf(ctl.Ownership)); err != nil {
		return nil, err
	}

	after := snapshot(ctl)
	ev := auditEvent(c, domain.AuditActionCreate, "control created", nil, after, changedFields(nil, after), id, now)
	if err := u.repo.CreateControl(ctx, ctl, ev); err != nil {
		return nil, fmt.Errorf("application.CreateInternalControl: %w", err)
	}
	v := ViewOf(ctl)
	return &v, nil
}

func ownershipPatchOf(o canonical.Ownership) domain.OwnershipPatch {
	var p domain.OwnershipPatch
	if o.OwnerID != nil {
		p.Owner = domain.Assign(*o.OwnerID)
	}
	if o.AssigneeID != nil {
		p.Assignee = domain.Assign(*o.AssigneeID)
	}
	if o.ReviewerID != nil {
		p.Reviewer = domain.Assign(*o.ReviewerID)
	}
	return p
}
