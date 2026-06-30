// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
