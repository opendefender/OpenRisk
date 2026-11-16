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

// DeleteImplementationUseCase soft-deletes an implementation of the tenant.
type DeleteImplementationUseCase struct {
	repo ImplementationRepository
	now  Clock
}

// NewDeleteImplementationUseCase builds the use case.
func NewDeleteImplementationUseCase(repo ImplementationRepository, now Clock) *DeleteImplementationUseCase {
	return &DeleteImplementationUseCase{repo: repo, now: now}
}

// Execute soft-deletes the implementation and, in the same transaction, its live
// tests. The applies_to edges are kept: readers filter on the liveness of both
// ends (A10.3). The audit trail gets one entry for the implementation and one
// per test. An absent, deleted or foreign id is ErrNotFound.
func (u *DeleteImplementationUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID) error {
	if err := c.authorize(PermissionDelete); err != nil {
		return err
	}
	now := u.now().UTC()
	err := u.repo.DeleteImplementation(ctx, c.TenantID, id, func(cur ImplementationRecord, tests []canonical.ControlTest) ([]*domain.AuditEvent, error) {
		before := implementationSnapshot(cur.Implementation, cur.AppliesTo)
		events := []*domain.AuditEvent{
			auditEventOf(auditImplementationType, c, domain.AuditActionDelete, "control implementation deleted", before, nil,
				changedFields(before, nil), id, now),
		}
		for _, t := range tests {
			b := testSnapshot(t)
			events = append(events, auditEventOf(auditTestType, c, domain.AuditActionDelete,
				"control test deleted with its implementation", b, nil, changedFields(b, nil), t.ID, now))
		}
		return events, nil
	})
	if err != nil {
		return fmt.Errorf("application.DeleteControlImplementation: %w", err)
	}
	return nil
}
