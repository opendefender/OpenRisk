// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// UpdateInput is a partial edit. A nil field is left alone. For the three text
// fields a blank value clears them; the ownership slots are tri-state (absent,
// null to clear, an id to assign). Like CreateInput it has no envelope field.
type UpdateInput struct {
	Name                       *string
	Description                *string
	Objective                  *string
	Ownership                  domain.OwnershipPatch
	Applicability              *string
	ApplicabilityJustification *string
	DeclaredStatus             *string
	Tags                       *[]string
}

// UpdateUseCase edits a control of the tenant's library.
type UpdateUseCase struct {
	repo   Repository
	owners OwnerValidator
	now    Clock
}

// NewUpdateUseCase builds the use case.
func NewUpdateUseCase(repo Repository, owners OwnerValidator, now Clock) *UpdateUseCase {
	return &UpdateUseCase{repo: repo, owners: owners, now: now}
}

// Execute applies the edit to the stored control, validates the result, and
// stores it with its audit entry in one transaction. One row is edited, so every
// requirement the control addresses shows the change at once (AC2).
//
//   - not_applicable needs a justification (ErrValidation), except on a control
//     with applicability_review_pending (A10.1).
//   - applicability_review_pending is never writable. The server clears it in the
//     same write when the stored control has it and the edit supplies a
//     non-empty justification or makes the control applicable.
//   - Moving declared_status INTO implemented needs live, accepted, unexpired
//     evidence reachable by an evidenced_by edge (D-108 Q16); re-saving an
//     implemented control is never blocked. The refusal is evidence_required.
func (u *UpdateUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID, in UpdateInput) (*ControlView, error) {
	if err := c.authorize(PermissionUpdate); err != nil {
		return nil, err
	}
	if err := checkLongTexts(in.Description, in.Objective, in.ApplicabilityJustification); err != nil {
		return nil, err
	}
	var tags canonical.List[string]
	if in.Tags != nil {
		t, err := cleanTags(*in.Tags)
		if err != nil {
			return nil, err
		}
		tags = t
	}
	if err := validateOwners(ctx, u.owners, c.TenantID, in.Ownership); err != nil {
		return nil, err
	}
	now := u.now().UTC()

	out, err := u.repo.UpdateControl(ctx, c.TenantID, id, func(cur canonical.Control, evidenced func(asOf time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error) {
		next := cur
		if in.Name != nil {
			next.Name = *in.Name
		}
		if in.Description != nil {
			next.Description = blankToNil(in.Description)
		}
		if in.Objective != nil {
			next.Objective = blankToNil(in.Objective)
		}
		if in.Ownership.Owner.Present {
			next.OwnerID = in.Ownership.Owner.Value
		}
		if in.Ownership.Assignee.Present {
			next.AssigneeID = in.Ownership.Assignee.Value
		}
		if in.Ownership.Reviewer.Present {
			next.ReviewerID = in.Ownership.Reviewer.Value
		}
		if in.Applicability != nil {
			next.Applicability = canonical.Applicability(*in.Applicability)
		}
		if in.ApplicabilityJustification != nil {
			next.ApplicabilityJustification = blankToNil(in.ApplicabilityJustification)
		}
		if in.DeclaredStatus != nil {
			next.DeclaredStatus = canonical.DeclaredStatus(*in.DeclaredStatus)
		}
		if in.Tags != nil {
			next.Tags = tags
		}

		// A10.1: the first write that answers the review clears the flag.
		if cur.ApplicabilityReviewPending() &&
			(next.Applicability == canonical.Applicable ||
				(in.ApplicabilityJustification != nil && blankToNil(in.ApplicabilityJustification) != nil)) {
			next = canonical.WithApplicabilityReviewPending(next, false)
		}

		if err := validation(next.Validate()); err != nil {
			return cur, nil, err
		}
		if cur.DeclaredStatus != canonical.DeclaredImplemented && next.DeclaredStatus == canonical.DeclaredImplemented {
			ok, err := evidenced(now)
			if err != nil {
				return cur, nil, err
			}
			if !ok {
				return cur, nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlEvidenceRequired)
			}
		}

		before, after := snapshot(cur), snapshot(next)
		changed := changedFields(before, after)
		if len(changed) == 0 {
			return cur, nil, nil
		}
		next.UpdatedAt = now
		ev := auditEvent(c, domain.AuditActionUpdate, "control updated", before, after, changed, id, now)
		return next, ev, nil
	})
	if err != nil {
		return nil, fmt.Errorf("application.UpdateInternalControl: %w", err)
	}
	v := ViewOf(*out)
	return &v, nil
}
