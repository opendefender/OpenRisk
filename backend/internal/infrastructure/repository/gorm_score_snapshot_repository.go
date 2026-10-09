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
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormScoreSnapshotRepository stores the daily tenant score history (#901).
// Every query is keyed on tenant_id.
type GormScoreSnapshotRepository struct {
	db *gorm.DB
}

func NewGormScoreSnapshotRepository(db *gorm.DB) *GormScoreSnapshotRepository {
	return &GormScoreSnapshotRepository{db: db}
}

// Upsert writes the day's snapshot, replacing an earlier one for the same
// tenant and day.
func (r *GormScoreSnapshotRepository) Upsert(ctx context.Context, s *domain.TenantScoreSnapshot) error {
	if s.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant is required")
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.Day = domain.SnapshotDay(s.Day)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "day"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "band", "formula_version", "updated_at"}),
	}).Create(s).Error
}

// ListSince returns the tenant's snapshots from `since` (inclusive), oldest
// first.
func (r *GormScoreSnapshotRepository) ListSince(ctx context.Context, tenantID uuid.UUID, since time.Time) ([]domain.TenantScoreSnapshot, error) {
	var rows []domain.TenantScoreSnapshot
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND day >= ?", tenantID, domain.SnapshotDay(since)).
		Order("day ASC").
		Find(&rows).Error
	return rows, err
}

// ListTenantIDs returns every organization id, for the daily snapshot sweep.
// A deleted organization's row is gone (internal/infrastructure/orgdeletion),
// so it drops out on its own.
func (r *GormScoreSnapshotRepository) ListTenantIDs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Model(&domain.Organization{}).Pluck("id", &ids).Error
	return ids, err
}
