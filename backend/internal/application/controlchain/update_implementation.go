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

// UpdateImplementationInput is a partial edit. A nil field is left alone;
// in_place_since blank clears it; the ownership slots are tri-state; AppliesTo,
// when non-nil, REPLACES the target set (and must not be empty). The control an
// implementation belongs to cannot be changed. Like the create input it has no
// envelope field.
type UpdateImplementationInput struct {
	Description  *string
	Status       *string
	InPlaceSince *string
	Ownership    domain.OwnershipPatch
	AppliesTo    *[]TargetRef
}

// UpdateImplementationUseCase edits an implementation of the tenant.
type UpdateImplementationUseCase struct {
	repo   ImplementationRepository
	owners OwnerValidator
	now    Clock
}

// NewUpdateImplementationUseCase builds the use case.
func NewUpdateImplementationUseCase(repo ImplementationRepository, owners OwnerValidator, now Clock) *UpdateImplementationUseCase {
	return &UpdateImplementationUseCase{repo: repo, owners: owners, now: now}
}

// Execute applies the edit to the stored implementation, validates the result,
// and stores the row, the target changes and the audit entry in one transaction.
// Targets that are new are resolved in the caller's tenant (ErrNotFound for a
// foreign asset); targets that are gone have their applies_to edge hard-deleted,
// journaled by the entry's before and after target sets (A10.3).
func (u *UpdateImplementationUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID, in UpdateImplementationInput) (*ImplementationView, error) {
	if err := c.authorize(PermissionUpdate); err != nil {
		return nil, err
	}
	if err := checkLongText("description", in.Description); err != nil {
		return nil, err
	}
	var want []uuid.UUID
	if in.AppliesTo != nil {
		t, err := cleanTargets(*in.AppliesTo)
		if err != nil {
			return nil, err
		}
		want = t
	}
	if err := validateOwners(ctx, u.owners, c.TenantID, in.Ownership); err != nil {
		return nil, err
	}
	conf, err := userConfidence()
	if err != nil {
		return nil, validation(err)
	}
	now := u.now().UTC()

	out, err := u.repo.UpdateImplementation(ctx, c.TenantID, id, func(cur ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		fail := func(err error) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
			return cur.Implementation, nil, nil, nil, err
		}
		next := cur.Implementation
		if in.Description != nil {
			next.Description = *in.Description
		}
		if in.Status != nil {
			next.Status = canonical.ImplementationStatus(*in.Status)
		}
		if in.InPlaceSince != nil {
			next.InPlaceSince = nil
			if s := blankToNil(in.InPlaceSince); s != nil {
				d := canonical.Date(*s)
				next.InPlaceSince = &d
			}
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
		if err := validation(next.Validate()); err != nil {
			return fail(err)
		}

		targets := cur.AppliesTo
		stored := cur.AppliesTo
		var add []canonical.Edge
		var remove []uuid.UUID
		if want != nil {
			targets = want
			have := make(map[uuid.UUID]bool, len(cur.AppliesTo))
			for _, a := range cur.AppliesTo {
				have[a] = true
			}
			wanted := make(map[uuid.UUID]bool, len(want))
			for _, a := range want {
				wanted[a] = true
				if !have[a] {
					e, err := appliesToEdge(c, id, a, conf, now)
					if err != nil {
						return fail(err)
					}
					add = append(add, e)
				}
			}
			// Every stored edge not wanted goes, the ones to a deleted asset included:
			// a restored asset must not bring a dropped link back, and every hard
			// delete is journaled in the before set.
			stored = append(append([]uuid.UUID{}, cur.AppliesTo...), cur.StaleTargets...)
			sortIDs(stored)
			for _, a := range stored {
				if !wanted[a] {
					remove = append(remove, a)
				}
			}
		}

		before, after := implementationSnapshot(cur.Implementation, stored), implementationSnapshot(next, targets)
		changed := changedFields(before, after)
		if len(changed) == 0 {
			return fail(nil)
		}
		next.UpdatedAt = now
		ev := auditEventOf(auditImplementationType, c, domain.AuditActionUpdate, "control implementation updated", before, after, changed, id, now)
		return next, add, remove, ev, nil
	})
	if err != nil {
		return nil, fmt.Errorf("application.UpdateControlImplementation: %w", err)
	}
	v := ImplementationViewOf(*out)
	return &v, nil
}
