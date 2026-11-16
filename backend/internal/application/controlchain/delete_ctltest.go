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

// DeleteTestUseCase soft-deletes a test definition of the tenant.
type DeleteTestUseCase struct {
	repo TestRepository
	now  Clock
}

// NewDeleteTestUseCase builds the use case.
func NewDeleteTestUseCase(repo TestRepository, now Clock) *DeleteTestUseCase {
	return &DeleteTestUseCase{repo: repo, now: now}
}

// Execute soft-deletes the test (deleted_at, tenant-filtered). An absent,
// deleted or foreign id is ErrNotFound.
func (u *DeleteTestUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID) error {
	if err := c.authorize(PermissionDelete); err != nil {
		return err
	}
	now := u.now().UTC()
	err := u.repo.DeleteTest(ctx, c.TenantID, id, func(cur canonical.ControlTest) (*domain.AuditEvent, error) {
		before := testSnapshot(cur)
		return auditEventOf(auditTestType, c, domain.AuditActionDelete, "control test deleted", before, nil, changedFields(before, nil), id, now), nil
	})
	if err != nil {
		return fmt.Errorf("application.DeleteControlTest: %w", err)
	}
	return nil
}
