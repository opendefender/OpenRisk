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

// GormTelemetryRepository persists the single instance-level telemetry consent
// row (never tenant-scoped — telemetry is a deployment-wide concern).
type GormTelemetryRepository struct {
	db *gorm.DB
}

func NewGormTelemetryRepository(db *gorm.DB) *GormTelemetryRepository {
	return &GormTelemetryRepository{db: db}
}

// GetOrCreate returns the consent row, creating it (disabled, with a fresh random
// InstanceID) on first access.
func (r *GormTelemetryRepository) GetOrCreate(ctx context.Context) (*domain.TelemetryConfig, error) {
	var cfg domain.TelemetryConfig
	err := r.db.WithContext(ctx).First(&cfg).Error
	if err == nil {
		return &cfg, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	cfg = domain.TelemetryConfig{InstanceID: uuid.New(), Enabled: false}
	if err := r.db.WithContext(ctx).Create(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SetEnabled flips consent and records who changed it (local audit only).
func (r *GormTelemetryRepository) SetEnabled(ctx context.Context, enabled bool, by uuid.UUID) (*domain.TelemetryConfig, error) {
	cfg, err := r.GetOrCreate(ctx)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{"enabled": enabled}
	if by != uuid.Nil {
		updates["updated_by"] = by
	}
	if err := r.db.WithContext(ctx).Model(&domain.TelemetryConfig{}).
		Where("id = ?", cfg.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	cfg.Enabled = enabled
	return cfg, nil
}
