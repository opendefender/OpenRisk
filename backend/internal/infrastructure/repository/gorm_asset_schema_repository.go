// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormAssetTypeSchemaRepository persists per-tenant asset attribute schemas.
//
// ABSOLUTE RULE #2: every query below filters by tenant_id.
type GormAssetTypeSchemaRepository struct {
	db *gorm.DB
}

func NewGormAssetTypeSchemaRepository(db *gorm.DB) *GormAssetTypeSchemaRepository {
	return &GormAssetTypeSchemaRepository{db: db}
}

var _ domain.AssetTypeSchemaRepository = (*GormAssetTypeSchemaRepository)(nil)

func (r *GormAssetTypeSchemaRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.AssetTypeSchema, error) {
	var out []domain.AssetTypeSchema
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("category ASC").
		Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *GormAssetTypeSchemaRepository) GetByCategory(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory) (*domain.AssetTypeSchema, error) {
	var s domain.AssetTypeSchema
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND category = ?", tenantID, cat).
		First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Upsert writes the schema for (tenant, category), relying on the unique index
// so two concurrent first-reads seeding the same default cannot create two rows.
func (r *GormAssetTypeSchemaRepository) Upsert(ctx context.Context, s *domain.AssetTypeSchema) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "category"}},
		DoUpdates: clause.AssignmentColumns([]string{"label", "attributes", "customized", "version", "updated_at"}),
	}).Create(s).Error
}

func (r *GormAssetTypeSchemaRepository) Delete(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND category = ?", tenantID, cat).
		Delete(&domain.AssetTypeSchema{}).Error
}
