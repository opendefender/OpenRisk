// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormControlMappingRepository implements domain.ControlMappingRepository.
// Tenant-scoped on every query.
type GormControlMappingRepository struct {
	db *gorm.DB
}

func NewGormControlMappingRepository(db *gorm.DB) *GormControlMappingRepository {
	return &GormControlMappingRepository{db: db}
}

func (r *GormControlMappingRepository) Create(ctx context.Context, m *domain.ControlMapping) error {
	if m.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *GormControlMappingRepository) Exists(ctx context.Context, tenantID, a, b uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.ControlMapping{}).
		Where("tenant_id = ?", tenantID).
		Where("(source_control_id = ? AND target_control_id = ?) OR (source_control_id = ? AND target_control_id = ?)", a, b, b, a).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check mapping existence: %w", err)
	}
	return count > 0, nil
}

func (r *GormControlMappingRepository) List(ctx context.Context, tenantID uuid.UUID, controlID *uuid.UUID) ([]domain.ControlMapping, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if controlID != nil {
		q = q.Where("source_control_id = ? OR target_control_id = ?", *controlID, *controlID)
	}
	var mappings []domain.ControlMapping
	if err := q.Order("created_at DESC").Find(&mappings).Error; err != nil {
		return nil, fmt.Errorf("failed to list mappings: %w", err)
	}
	return mappings, nil
}

func (r *GormControlMappingRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ControlMapping{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete mapping: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("control mapping", id)
	}
	return nil
}
