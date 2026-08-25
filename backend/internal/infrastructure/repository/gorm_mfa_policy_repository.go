// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormMFAPolicyRepository stores one MFA grace policy per tenant (OR26-03).
//
// Every method takes the tenant explicitly and filters on it (RULE #2). The
// unique index on tenant_id is what makes "read the tenant's policy" a
// single-row question; without it the same tenant could accumulate several
// answers and the one that wins would depend on row order.
type GormMFAPolicyRepository struct {
	db *gorm.DB
}

// NewGormMFAPolicyRepository builds the repository.
func NewGormMFAPolicyRepository(db *gorm.DB) *GormMFAPolicyRepository {
	return &GormMFAPolicyRepository{db: db}
}

// GetMFAPolicy returns the tenant's saved policy, or nil when it has never
// opened the setting.
//
// nil is not an error: the caller substitutes domain.DefaultMFAPolicy, so a
// tenant that never touched the screen behaves exactly like one that saved the
// defaults. Returning a synthesised row here instead would make "saved" and
// "never saved" indistinguishable, and the settings screen needs to tell them
// apart to show who last changed it.
func (r *GormMFAPolicyRepository) GetMFAPolicy(ctx context.Context, tenantID uuid.UUID) (*domain.MFAPolicy, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	var row domain.MFAPolicy
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// SaveMFAPolicy creates or updates the tenant's policy.
//
// Scoped on tenant_id for the update, not on the primary key: a client that
// guessed another tenant's policy id must not be able to steer the write. The
// id it supplies is ignored entirely — the tenant is the identity here.
func (r *GormMFAPolicyRepository) SaveMFAPolicy(ctx context.Context, p *domain.MFAPolicy) error {
	if p == nil || p.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant is required")
	}
	if err := p.Validate(); err != nil {
		return err
	}

	existing, err := r.GetMFAPolicy(ctx, p.TenantID)
	if err != nil {
		return err
	}

	now := time.Now()
	if existing == nil {
		p.ID = uuid.New()
		p.CreatedAt = now
		p.UpdatedAt = now
		return r.db.WithContext(ctx).Create(p).Error
	}

	existing.GraceDays = p.GraceDays
	existing.UpdatedByID = p.UpdatedByID
	existing.UpdatedAt = now
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", p.TenantID).
		Save(existing).Error; err != nil {
		return err
	}
	*p = *existing
	return nil
}
