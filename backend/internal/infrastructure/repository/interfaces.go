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
	GetByID(ctx context.Context, id uuid.UUID) (*domain.PersonalAccessToken, error)
	GetByTokenHash(ctx context.Context, hash string) (*domain.PersonalAccessToken, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.PersonalAccessToken, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}
