// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormRiskScoringWeightsRepository is the Postgres-backed store for the per-tenant
// smart-risk factor weights (spec §8). Exactly one row per tenant.
// ABSOLUTE RULE: every query filters by tenant_id.
type GormRiskScoringWeightsRepository struct {
	db *gorm.DB
}

func NewGormRiskScoringWeightsRepository(db *gorm.DB) *GormRiskScoringWeightsRepository {
	return &GormRiskScoringWeightsRepository{db: db}
}

var _ domain.RiskScoringWeightsRepository = (*GormRiskScoringWeightsRepository)(nil)

// GetByTenant returns the tenant's weights row, or (nil, nil) if it has never
// been customised (the use case then falls back to the engine defaults).
func (r *GormRiskScoringWeightsRepository) GetByTenant(ctx context.Context, tenantID uuid.UUID) (*domain.RiskScoringWeights, error) {
	var w domain.RiskScoringWeights
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&w).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// Upsert inserts or updates the tenant's single weights row (matched on tenant_id).
func (r *GormRiskScoringWeightsRepository) Upsert(ctx context.Context, w *domain.RiskScoringWeights) error {
	var existing domain.RiskScoringWeights
	err := r.db.WithContext(ctx).Where("tenant_id = ?", w.TenantID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(w).Error
	}
	if err != nil {
		return err
	}
	w.ID = existing.ID
	w.CreatedAt = existing.CreatedAt
	return r.db.WithContext(ctx).Model(&existing).Select(
		"business_criticality", "internet_exposure", "vulnerabilities",
		"control_maturity", "incident_history", "exploitability",
		"financial_value", "threat_intel", "updated_by", "updated_at",
	).Updates(w).Error
}
