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

// GormAssetDependencyRepository implements domain.AssetDependencyRepository.
// ABSOLUTE RULE: Filter by tenant_id on EVERY query. Cross-tenant reads return
// (nil, nil) / empty, never 403.
type GormAssetDependencyRepository struct {
	db *gorm.DB
}

// NewGormAssetDependencyRepository creates a new GORM-backed dependency repository.
func NewGormAssetDependencyRepository(db *gorm.DB) *GormAssetDependencyRepository {
	return &GormAssetDependencyRepository{db: db}
}

func (r *GormAssetDependencyRepository) Create(ctx context.Context, dep *domain.AssetDependency) error {
	if dep.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(dep).Error
}

func (r *GormAssetDependencyRepository) GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.AssetDependency, error) {
	var dep domain.AssetDependency
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&dep).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get dependency: %w", err)
	}
	return &dep, nil
}

func (r *GormAssetDependencyRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	var deps []domain.AssetDependency
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&deps).Error
	return deps, err
}

func (r *GormAssetDependencyRepository) ListByAsset(ctx context.Context, assetID uuid.UUID, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	var deps []domain.AssetDependency
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND (source_asset_id = ? OR target_asset_id = ?)", tenantID, assetID, assetID).
		Order("created_at DESC").
		Find(&deps).Error
	return deps, err
}

func (r *GormAssetDependencyRepository) Exists(ctx context.Context, tenantID, sourceAssetID, targetAssetID uuid.UUID, depType domain.DependencyType) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.AssetDependency{}).
		Where("tenant_id = ? AND source_asset_id = ? AND target_asset_id = ? AND type = ?",
			tenantID, sourceAssetID, targetAssetID, depType).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check dependency existence: %w", err)
	}
	return count > 0, nil
}

func (r *GormAssetDependencyRepository) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.AssetDependency{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete dependency: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("dependency not found")
	}
	return nil
}

func (r *GormAssetDependencyRepository) DeleteByAsset(ctx context.Context, assetID uuid.UUID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND (source_asset_id = ? OR target_asset_id = ?)", tenantID, assetID, assetID).
		Delete(&domain.AssetDependency{}).Error
}
