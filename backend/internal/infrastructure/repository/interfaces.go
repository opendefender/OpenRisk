// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

type AuthAuditLogRepository interface {
	Create(ctx context.Context, log *domain.AuthAuditLog) error
	GetByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.AuthAuditLog, error)
	GetByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*domain.AuthAuditLog, error)
}

type PersonalAccessTokenRepository interface {
	Create(ctx context.Context, token *domain.PersonalAccessToken) error
	// GetByTokenHash is the authentication lookup and is deliberately not
	// tenant-filtered: no tenant is known until the token is found, and the row
	// it returns carries the one tenant the token may act in.
	GetByTokenHash(ctx context.Context, hash string) (*domain.PersonalAccessToken, error)
	ListByOwner(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.PersonalAccessToken, error)
	UpdateLastUsed(ctx context.Context, tenantID, id uuid.UUID) error
	// DeleteByOwner removes the token only when it belongs to userID in
	// tenantID, and reports whether a row was removed.
	DeleteByOwner(ctx context.Context, tenantID, userID, id uuid.UUID) (bool, error)
}
