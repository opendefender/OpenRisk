// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlexception

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// RevokeUseCase ends an exception before its expiry.
type RevokeUseCase struct {
	repo Repository
	now  Clock
}

// NewRevokeUseCase builds the use case.
func NewRevokeUseCase(repo Repository, now Clock) *RevokeUseCase {
	return &RevokeUseCase{repo: repo, now: now}
}

// Execute revokes the exception of the caller's tenant. A missing or foreign id
// is ErrNotFound; one already revoked or already expired is ErrConflict.
func (u *RevokeUseCase) Execute(ctx context.Context, c Caller, exceptionID uuid.UUID, reason string) (*View, error) {
	if err := c.authorize(PermissionUpdate); err != nil {
		return nil, err
	}
	now := u.now()
	ex, err := u.repo.Revoke(ctx, c.TenantID, exceptionID, func(cur *domain.ControlException) (*domain.AuditEvent, error) {
		before := snapshot(*cur, now)
		if err := cur.Revoke(c.UserID, reason, now); err != nil {
			return nil, err
		}
		return auditEvent(c, domain.AuditActionUpdate, "control exception revoked", before, snapshot(*cur, now),
			domain.StringList{"revoked_at", "revoked_by", "revoke_reason", "status"}, cur.ID, now), nil
	})
	if err != nil {
		return nil, fmt.Errorf("application.RevokeControlException: %w", err)
	}
	v := ViewOf(*ex, now)
	return &v, nil
}
