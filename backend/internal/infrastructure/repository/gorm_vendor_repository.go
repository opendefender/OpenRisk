// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormVendorRepository implements domain.VendorRepository (#669, ADR 0004 D1,
// D2). It reads the assets table — a vendor is an asset of category vendor — and
// owns no table of its own.
//
// ABSOLUTE RULE: every query below carries the tenant. The two tables without a
// tenant_id column of their own (risk_assets, and risks reached through the
// legacy asset_id) are gated on the PARENT RISK's tenant_id, stated where it
// happens.
type GormVendorRepository struct {
	db *gorm.DB
}

func NewGormVendorRepository(db *gorm.DB) *GormVendorRepository {
	return &GormVendorRepository{db: db}
}

var errVendorRepoNoTenant = errors.New("vendor repository: tenant_id is required")

func (r *GormVendorRepository) ListVendors(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorRepoNoTenant
	}
	var vendors []domain.Asset
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND category = ?", tenantID, domain.CategoryVendor).
		Order("name ASC").
		Find(&vendors).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list vendors: %w", err)
	}
	return vendors, nil
}

func (r *GormVendorRepository) GetVendor(ctx context.Context, id, tenantID uuid.UUID) (*domain.Asset, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorRepoNoTenant
	}
	var vendor domain.Asset
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND category = ?", id, tenantID, domain.CategoryVendor).
		First(&vendor).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get vendor: %w", err)
	}
	return &vendor, nil
}

func (r *GormVendorRepository) AssetsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]domain.Asset, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorRepoNoTenant
	}
	if len(ids) == 0 {
		return []domain.Asset{}, nil
	}
	var assets []domain.Asset
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Find(&assets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load linked assets: %w", err)
	}
	return assets, nil
}

// vendorChainRiskRow is one (asset, risk) pair as the two queries below scan it.
type vendorChainRiskRow struct {
	AssetID     uuid.UUID
	ID          uuid.UUID
	Title       string
	Score       float64
	Criticality string
	Status      string
}

// RisksByAssetIDs returns the tenant's risks per asset, through BOTH linkage
// mechanisms: the risk_assets many-to-many and the legacy risks.asset_id column.
// Both carry live data (see GormEntityRelationRepository.RisksForAsset), so
// reading only one would silently under-report the chain.
func (r *GormVendorRepository) RisksByAssetIDs(ctx context.Context, tenantID uuid.UUID, assetIDs []uuid.UUID) (map[uuid.UUID][]domain.VendorChainRisk, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorRepoNoTenant
	}
	out := make(map[uuid.UUID][]domain.VendorChainRisk, len(assetIDs))
	if len(assetIDs) == 0 {
		return out, nil
	}

	const cols = "risks.id AS id, risks.title AS title, risks.score AS score, risks.criticality AS criticality, risks.status AS status"

	// risk_assets has NO tenant_id. The gate is the parent risk's tenant_id in
	// the JOIN predicate: a join row pointing one of our assets at another
	// tenant's risk matches no risk and returns nothing.
	var joined []vendorChainRiskRow
	if err := r.db.WithContext(ctx).
		Table("risk_assets").
		Select("risk_assets.asset_id AS asset_id, "+cols).
		Joins("JOIN risks ON risks.id = risk_assets.risk_id AND risks.tenant_id = ?", tenantID).
		Where("risks.deleted_at IS NULL AND risk_assets.asset_id IN ?", assetIDs).
		Scan(&joined).Error; err != nil {
		return nil, fmt.Errorf("failed to load risks linked through risk_assets: %w", err)
	}

	var legacy []vendorChainRiskRow
	if err := r.db.WithContext(ctx).
		Table("risks").
		Select("risks.asset_id AS asset_id, "+cols).
		Where("risks.tenant_id = ? AND risks.deleted_at IS NULL AND risks.asset_id IN ?", tenantID, assetIDs).
		Scan(&legacy).Error; err != nil {
		return nil, fmt.Errorf("failed to load risks linked through risks.asset_id: %w", err)
	}

	seen := make(map[[2]uuid.UUID]struct{}, len(joined)+len(legacy))
	for _, row := range append(joined, legacy...) {
		key := [2]uuid.UUID{row.AssetID, row.ID}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out[row.AssetID] = append(out[row.AssetID], domain.VendorChainRisk{
			ID:          row.ID,
			Title:       row.Title,
			Score:       row.Score,
			Criticality: row.Criticality,
			Status:      row.Status,
		})
	}

	for assetID := range out {
		risks := out[assetID]
		sort.SliceStable(risks, func(i, j int) bool {
			if risks[i].Score != risks[j].Score {
				return risks[i].Score > risks[j].Score
			}
			return risks[i].Title < risks[j].Title
		})
	}
	return out, nil
}
