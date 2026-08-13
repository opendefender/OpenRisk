// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormControlCrosswalkRepository implements domain.ControlCrosswalkRepository.
// Tenant-scoped on every query.
type GormControlCrosswalkRepository struct {
	db *gorm.DB
}

func NewGormControlCrosswalkRepository(db *gorm.DB) *GormControlCrosswalkRepository {
	return &GormControlCrosswalkRepository{db: db}
}

func (r *GormControlCrosswalkRepository) Create(ctx context.Context, m *domain.ControlCrosswalk) error {
	if m.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *GormControlCrosswalkRepository) Exists(ctx context.Context, tenantID, a, b uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.ControlCrosswalk{}).
		Where("tenant_id = ?", tenantID).
		Where("(source_control_id = ? AND target_control_id = ?) OR (source_control_id = ? AND target_control_id = ?)", a, b, b, a).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check crosswalk existence: %w", err)
	}
	return count > 0, nil
}

func (r *GormControlCrosswalkRepository) List(ctx context.Context, tenantID uuid.UUID, controlID *uuid.UUID) ([]domain.ControlCrosswalk, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if controlID != nil {
		q = q.Where("source_control_id = ? OR target_control_id = ?", *controlID, *controlID)
	}
	var crosswalks []domain.ControlCrosswalk
	if err := q.Order("created_at DESC").Find(&crosswalks).Error; err != nil {
		return nil, fmt.Errorf("failed to list crosswalks: %w", err)
	}
	return crosswalks, nil
}

// ListByFramework returns every crosswalk with at least one foot in the
// framework — the query behind inherited coverage.
//
// The subquery names the framework's controls once and both sides are tested
// against it in one pass; loading the framework's controls into Go first and
// sending an IN list would put a framework-sized array on the wire for a
// question the database can answer on its own.
func (r *GormControlCrosswalkRepository) ListByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID) ([]domain.ControlCrosswalk, error) {
	var crosswalks []domain.ControlCrosswalk
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Where(`source_control_id IN (SELECT id FROM compliance_controls WHERE tenant_id = ? AND framework_id = ? AND deleted_at IS NULL)
		    OR target_control_id IN (SELECT id FROM compliance_controls WHERE tenant_id = ? AND framework_id = ? AND deleted_at IS NULL)`,
			tenantID, frameworkID, tenantID, frameworkID).
		Find(&crosswalks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list crosswalks for framework: %w", err)
	}
	return crosswalks, nil
}

func (r *GormControlCrosswalkRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ControlCrosswalk{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete crosswalk: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("control crosswalk", id)
	}
	return nil
}
