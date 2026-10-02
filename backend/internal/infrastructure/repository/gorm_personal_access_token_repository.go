// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormPersonalAccessTokenRepository implements PersonalAccessTokenRepository using GORM
type GormPersonalAccessTokenRepository struct {
	db *gorm.DB
}

// NewGormPersonalAccessTokenRepository creates a new GORM PAT repository
func NewGormPersonalAccessTokenRepository(db *gorm.DB) *GormPersonalAccessTokenRepository {
	return &GormPersonalAccessTokenRepository{db: db}
}

// Create creates a new personal access token
func (r *GormPersonalAccessTokenRepository) Create(ctx context.Context, token *domain.PersonalAccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetByTokenHash gets a PAT by its hash. This is the authentication lookup: it
// runs before any tenant is known, and the row it returns names the only tenant
// the token may act in. Every other method below filters on tenant_id.
func (r *GormPersonalAccessTokenRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.PersonalAccessToken, error) {
	var token domain.PersonalAccessToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", hash).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// ListByOwner lists the tokens userID minted in tenantID, newest first.
func (r *GormPersonalAccessTokenRepository) ListByOwner(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.PersonalAccessToken, error) {
	var tokens []*domain.PersonalAccessToken
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("created_at DESC").
		Find(&tokens).Error
	return tokens, err
}

// UpdateLastUsed stamps the token's last use.
func (r *GormPersonalAccessTokenRepository) UpdateLastUsed(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&domain.PersonalAccessToken{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"last_used_at": time.Now(),
			"updated_at":   time.Now(),
		}).Error
}

// DeleteByOwner deletes the token only when userID owns it in tenantID. A
// foreign id, user or tenant matches no row and reports false.
func (r *GormPersonalAccessTokenRepository) DeleteByOwner(ctx context.Context, tenantID, userID, id uuid.UUID) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND id = ?", tenantID, userID, id).
		Delete(&domain.PersonalAccessToken{})
	return res.RowsAffected > 0, res.Error
}
