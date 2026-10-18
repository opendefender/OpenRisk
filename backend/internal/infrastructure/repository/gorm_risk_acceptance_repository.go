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

	riskapp "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
)

// GormRiskAcceptanceRepository backs the acceptance expiry sweep (#811).
type GormRiskAcceptanceRepository struct {
	db *gorm.DB
}

func NewGormRiskAcceptanceRepository(db *gorm.DB) *GormRiskAcceptanceRepository {
	return &GormRiskAcceptanceRepository{db: db}
}

var _ riskapp.AcceptanceStore = (*GormRiskAcceptanceRepository)(nil)

// ListAcceptancesDue reads accepted risks whose end date falls on or before
// `until`.
//
// Cross-tenant by design — it feeds the background sweep, the same shape as
// the evidence expiry and risk review workers. Every write that follows
// carries the row's own tenant_id.
func (r *GormRiskAcceptanceRepository) ListAcceptancesDue(ctx context.Context, until time.Time, limit int) ([]domain.Risk, error) {
	if limit <= 0 {
		limit = 500
	}
	var out []domain.Risk
	err := r.db.WithContext(ctx).
		Where("lifecycle_state = ?", domain.StateResidualAccepted).
		Where("acceptance_expires_at IS NOT NULL AND acceptance_expires_at <= ?", until).
		Order("acceptance_expires_at ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

// MarkAcceptanceReminded stamps one reminder column, only if it is still
// empty and the risk is still accepted. RowsAffected tells the caller whether
// it won the stamp.
func (r *GormRiskAcceptanceRepository) MarkAcceptanceReminded(ctx context.Context, tenantID, riskID uuid.UUID, stage riskapp.AcceptanceReminderStage, at time.Time) (bool, error) {
	var col string
	switch stage {
	case riskapp.AcceptanceReminder30:
		col = "acceptance_reminder30_at"
	case riskapp.AcceptanceReminder7:
		col = "acceptance_reminder7_at"
	default:
		return false, domain.NewValidationError("unknown acceptance reminder stage")
	}
	res := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("id = ? AND tenant_id = ?", riskID, tenantID).
		Where("lifecycle_state = ?", domain.StateResidualAccepted).
		Where(col+" IS NULL").
		UpdateColumn(col, at)
	return res.RowsAffected == 1, res.Error
}

// ExpireAcceptance is the system transition RESIDUAL_ACCEPTED → ASSESSED.
// Status and phase are derived from the state exactly as Risk.SetState does,
// so the three columns cannot disagree.
func (r *GormRiskAcceptanceRepository) ExpireAcceptance(ctx context.Context, tenantID, riskID uuid.UUID, at time.Time) (bool, error) {
	target := domain.StateAssessed
	res := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("id = ? AND tenant_id = ?", riskID, tenantID).
		Where("lifecycle_state = ?", domain.StateResidualAccepted).
		Where("acceptance_expires_at IS NOT NULL AND acceptance_expires_at <= ?", at).
		UpdateColumns(map[string]interface{}{
			"lifecycle_state":       target,
			"status":                target.DerivedStatus(),
			"lifecycle_phase":       target.DerivedPhase(),
			"acceptance_expired_at": at,
			"updated_at":            at,
		})
	return res.RowsAffected == 1, res.Error
}

// MaxAcceptanceMonths reads the tenant's maximum acceptance length from the
// organization settings. The organization IS the tenant, matched on id.
// Unset or out of range → the default.
func (r *GormOrganizationRepository) MaxAcceptanceMonths(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var org domain.Organization
	// organizations has no tenant_id: the row IS the tenant, matched on id = tenantID.
	if err := r.db.WithContext(ctx).Where("id = ?", tenantID).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DefaultAcceptanceMonths, nil
		}
		return 0, err
	}
	if v, ok := org.GetSettings()[domain.AcceptanceSettingKey].(float64); ok {
		return domain.NormalizeAcceptanceMax(int(v)), nil
	}
	return domain.DefaultAcceptanceMonths, nil
}

// SetMaxAcceptanceMonths stores the tenant's maximum. Load-merge-save, like
// the currency and the risk appetite beside it.
func (r *GormOrganizationRepository) SetMaxAcceptanceMonths(ctx context.Context, tenantID uuid.UUID, months int) error {
	var org domain.Organization
	// organizations has no tenant_id: the row IS the tenant, matched on id = tenantID.
	if err := r.db.WithContext(ctx).Where("id = ?", tenantID).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.NewNotFoundError("organization", tenantID)
		}
		return err
	}
	settings := org.GetSettings()
	settings[domain.AcceptanceSettingKey] = months
	if err := org.SetSettings(settings); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&domain.Organization{}).
		Where("id = ?", tenantID).
		Update("settings", org.Settings).Error
}
