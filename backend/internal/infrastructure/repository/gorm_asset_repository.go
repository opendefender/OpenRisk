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

// GormAssetRepository implements domain.AssetRepository using GORM.
// ABSOLUTE RULE: Filter by tenant_id on EVERY query. If an asset belongs to
// another tenant → return (nil, nil), never 403.
type GormAssetRepository struct {
	db *gorm.DB
}

// NewGormAssetRepository creates a new GORM-backed asset repository.
func NewGormAssetRepository(db *gorm.DB) *GormAssetRepository {
	return &GormAssetRepository{db: db}
}

// Create persists a new asset for a tenant.
func (r *GormAssetRepository) Create(ctx context.Context, asset *domain.Asset) error {
	if asset.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(asset).Error
}

// GetByID retrieves an asset by ID scoped to a tenant, with linked risks preloaded.
func (r *GormAssetRepository) GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Asset, error) {
	var asset domain.Asset
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Risks").
		First(&asset).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}
	return &asset, nil
}

// List retrieves all assets for a tenant, with linked risks preloaded.
func (r *GormAssetRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	var assets []domain.Asset
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Risks").
		Order("name ASC").
		Find(&assets).Error
	return assets, err
}

// Update updates an existing asset.
// NOTE: Model()+Where()+Updates() (not Save()) is the pattern that actually
// honors the WHERE clause — see GormComplianceRepository.UpdateControl's doc
// comment for why Save() alone would let one tenant overwrite another's row.
func (r *GormAssetRepository) Update(ctx context.Context, asset *domain.Asset) error {
	if asset.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}

	result := r.db.WithContext(ctx).
		Model(&domain.Asset{}).
		Where("id = ? AND tenant_id = ?", asset.ID, asset.TenantID).
		Select("name", "type", "criticality", "owner").
		Updates(asset)

	if result.Error != nil {
		return fmt.Errorf("failed to update asset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}

// Delete soft-deletes an asset by ID scoped to a tenant.
func (r *GormAssetRepository) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.Asset{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete asset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}

// CreateSnapshot persists a historical snapshot of an asset's state.
func (r *GormAssetRepository) CreateSnapshot(ctx context.Context, snapshot *domain.AssetSnapshot) error {
	if snapshot.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(snapshot).Error
}

// ListSnapshots retrieves the history of an asset, newest first, scoped to a tenant.
func (r *GormAssetRepository) ListSnapshots(ctx context.Context, assetID uuid.UUID, tenantID uuid.UUID) ([]domain.AssetSnapshot, error) {
	var snapshots []domain.AssetSnapshot
	err := r.db.WithContext(ctx).
		Where("asset_id = ? AND tenant_id = ?", assetID, tenantID).
		Order("created_at DESC").
		Find(&snapshots).Error
	return snapshots, err
}
