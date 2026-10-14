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

// GormHostInventoryRepository stores host inventories (#910). Every query is
// filtered by tenant_id, except TenantsWithInventories, which only enumerates
// tenant ids for the daily job.
type GormHostInventoryRepository struct {
	db *gorm.DB
}

// NewGormHostInventoryRepository builds the repository.
func NewGormHostInventoryRepository(db *gorm.DB) *GormHostInventoryRepository {
	return &GormHostInventoryRepository{db: db}
}

var _ domain.HostInventoryRepository = (*GormHostInventoryRepository)(nil)

// packageInsertBatch keeps one INSERT under PostgreSQL's 65 535 parameters.
const packageInsertBatch = 1000

// Replace upserts the inventory on (tenant, host key) and replaces its
// packages in one transaction, so a reader never sees half a snapshot.
func (r *GormHostInventoryRepository) Replace(ctx context.Context, inv *domain.HostInventory, pkgs []domain.HostPackageInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.HostInventory
		err := tx.Where("tenant_id = ? AND host_key = ?", inv.TenantID, inv.HostKey).First(&existing).Error
		switch {
		case err == nil:
			// Same host, same row: keep its id and match state.
			inv.ID = existing.ID
			inv.CreatedAt = existing.CreatedAt
			inv.MatchedAt = existing.MatchedAt
			if err := tx.Save(inv).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if inv.ID == uuid.Nil {
				inv.ID = uuid.New()
			}
			if err := tx.Create(inv).Error; err != nil {
				return err
			}
		default:
			return err
		}

		if err := tx.Where("tenant_id = ? AND inventory_id = ?", inv.TenantID, inv.ID).
			Delete(&domain.HostPackage{}).Error; err != nil {
			return err
		}
		rows := make([]domain.HostPackage, 0, len(pkgs))
		for _, p := range pkgs {
			rows = append(rows, domain.HostPackage{
				ID: uuid.New(), TenantID: inv.TenantID, InventoryID: inv.ID,
				Name: p.Name, Version: p.Version, Source: p.Source, Publisher: p.Publisher, Arch: p.Arch,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.CreateInBatches(rows, packageInsertBatch).Error
	})
}

// Get returns one of the tenant's inventories, nil when it has none by that id.
func (r *GormHostInventoryRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.HostInventory, error) {
	var inv domain.HostInventory
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// List returns the tenant's inventories, by host.
func (r *GormHostInventoryRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.HostInventory, error) {
	var out []domain.HostInventory
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("host_key").Find(&out).Error
	return out, err
}

// ListByAsset returns the tenant's inventories attached to one asset.
func (r *GormHostInventoryRepository) ListByAsset(ctx context.Context, tenantID, assetID uuid.UUID) ([]domain.HostInventory, error) {
	var out []domain.HostInventory
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND asset_id = ?", tenantID, assetID).
		Order("host_key").Find(&out).Error
	return out, err
}

// Packages returns the packages of one of the tenant's inventories.
func (r *GormHostInventoryRepository) Packages(ctx context.Context, tenantID, inventoryID uuid.UUID) ([]domain.HostPackage, error) {
	var out []domain.HostPackage
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND inventory_id = ?", tenantID, inventoryID).
		Order("name").Find(&out).Error
	return out, err
}

// MarkMatched records the outcome of a match on one of the tenant's inventories.
func (r *GormHostInventoryRepository) MarkMatched(ctx context.Context, tenantID, id uuid.UUID, at time.Time, errCode string, uncheckable int) error {
	return r.db.WithContext(ctx).Model(&domain.HostInventory{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]any{
			"matched_at": at, "match_error": errCode, "uncheckable_count": uncheckable, "updated_at": at,
		}).Error
}

// TenantsWithInventories enumerates tenant ids for the daily re-match; each
// tenant's data is then read through the tenant-scoped methods above.
func (r *GormHostInventoryRepository) TenantsWithInventories(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	// Tenant-agnostic by design: enumerates tenants for the scheduler, reads no tenant data.
	err := r.db.WithContext(ctx).Model(&domain.HostInventory{}).Distinct("tenant_id").Pluck("tenant_id", &ids).Error
	return ids, err
}
