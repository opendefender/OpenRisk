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

// UpdateTestInput is a partial edit. A nil field is left alone; for procedure,
// collector and cadence a blank value clears them. The implementation a test
// belongs to cannot be changed. No envelope field.
type UpdateTestInput struct {
	Name         *string
	Checks       *string
	Method       *string
	Procedure    *string
	Collector    *string
	Cadence      *string
	PassCriteria *string
}

// UpdateTestUseCase edits a test definition of the tenant.
type UpdateTestUseCase struct {
	repo TestRepository
	now  Clock
}

// NewUpdateTestUseCase builds the use case.
func NewUpdateTestUseCase(repo TestRepository, now Clock) *UpdateTestUseCase {
	return &UpdateTestUseCase{repo: repo, now: now}
}

// Execute applies the edit to the stored test and validates the whole result
// (so a switch of method without the procedure or collector it needs is refused),
// then stores it with its audit entry in one transaction.
func (u *UpdateTestUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID, in UpdateTestInput) (*TestView, error) {
	if err := c.authorize(PermissionUpdate); err != nil {
		return nil, err
	}
	if err := checkTestTexts(in.Checks, in.Procedure, in.PassCriteria, in.Collector, in.Cadence); err != nil {
		return nil, err
	}
	now := u.now().UTC()
	out, err := u.repo.UpdateTest(ctx, c.TenantID, id, func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
		next := cur
		if in.Name != nil {
			next.Name = *in.Name
		}
		if in.Checks != nil {
			next.Checks = *in.Checks
		}
		if in.Method != nil {
			next.Method = canonical.TestMethod(*in.Method)
		}
		if in.Procedure != nil {
			next.Procedure = blankToNil(in.Procedure)
		}
		if in.Collector != nil {
			next.Collector = blankToNil(in.Collector)
		}
		if in.Cadence != nil {
			next.Cadence = blankToNil(in.Cadence)
		}
		if in.PassCriteria != nil {
			next.PassCriteria = *in.PassCriteria
		}
		if err := validation(next.Validate()); err != nil {
			return cur, nil, err
		}
		before, after := testSnapshot(cur), testSnapshot(next)
		changed := changedFields(before, after)
		if len(changed) == 0 {
			return cur, nil, nil
		}
		next.UpdatedAt = now
		return next, auditEventOf(auditTestType, c, domain.AuditActionUpdate, "control test updated", before, after, changed, id, now), nil
	})
	if err != nil {
		return nil, fmt.Errorf("application.UpdateControlTest: %w", err)
	}
	v := TestViewOf(*out)
	return &v, nil
}
