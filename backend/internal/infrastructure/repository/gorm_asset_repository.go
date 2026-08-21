// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

// Statistics counts the tenant's inventory by criticality, category, type and
// source in ONE grouped pass, plus one scalar row for the period-scoped count.
//
// It is a concrete method, deliberately OFF domain.AssetRepository — the same
// pattern as GormRiskRepository.ListRisksForFinancial. Adding it to the port
// would force every mock and every fake in the suite to grow a method they do
// not exercise, and the consumer (the statistics use case) declares its own
// narrow interface instead.
//
// TENANT (RULE #2): every query below carries `tenant_id = ?`, and the caller
// cannot reach this method without a resolved tenant — Execute rejects uuid.Nil
// before it gets here.
//
// SOFT DELETES: `deleted_at IS NULL` is spelled out rather than relying on GORM's
// soft-delete scope, because these are Raw/Table queries where that scope does
// not apply. Leaving it off is how a dashboard total climbs above the inventory
// the user can see, and it is invisible until someone deletes something.
func (r *GormAssetRepository) Statistics(ctx context.Context, tenantID uuid.UUID, from, to *time.Time) (*domain.AssetStatistics, error) {
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant_id is required")
	}

	stats := &domain.AssetStatistics{
		// All four bands are always present. A band with no assets is a fact
		// worth stating; an absent key makes the client decide what it meant.
		ByCriticality: map[string]int64{
			string(domain.CriticalityCritical): 0,
			string(domain.CriticalityHigh):     0,
			string(domain.CriticalityMedium):   0,
			string(domain.CriticalityLow):      0,
		},
		ByCategory: map[string]int64{},
		ByType:     map[string]int64{},
		BySource:   map[string]int64{},
	}

	// --- one grouped pass over the dimensions ------------------------------
	//
	// Grouping on all four dimensions at once and folding in Go keeps this to a
	// single scan. Four separate GROUP BY queries would each rescan the table
	// for a tenant whose inventory is the thing we are trying not to load.
	type dimRow struct {
		Criticality string
		Category    string
		Type        string
		Source      string
		Count       int64
	}
	var rows []dimRow
	if err := r.db.WithContext(ctx).
		Model(&domain.Asset{}).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Select("COALESCE(criticality,'') AS criticality, " +
			"COALESCE(category,'') AS category, " +
			"COALESCE(type,'') AS type, " +
			"COALESCE(source,'') AS source, " +
			"COUNT(*) AS count").
		Group("1, 2, 3, 4").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate asset statistics: %w", err)
	}

	typeCounts := map[string]int64{}
	for _, row := range rows {
		stats.Total += row.Count

		crit := strings.ToUpper(strings.TrimSpace(row.Criticality))
		if crit == "" {
			// The column defaults to MEDIUM, but a row written before that
			// default existed can still be blank. Bucketing it as MEDIUM keeps
			// Σ ByCriticality == Total, which is the invariant on screen.
			crit = string(domain.CriticalityMedium)
		}
		if _, known := stats.ByCriticality[crit]; !known {
			stats.ByCriticality[crit] = 0
		}
		stats.ByCriticality[crit] += row.Count

		if cat := strings.TrimSpace(row.Category); cat == "" {
			stats.Uncategorised += row.Count
		} else {
			stats.ByCategory[cat] += row.Count
		}

		if ty := strings.TrimSpace(row.Type); ty == "" {
			stats.Untyped += row.Count
		} else {
			typeCounts[ty] += row.Count
		}

		src := strings.TrimSpace(row.Source)
		if src == "" {
			src = "MANUAL" // the column's default; same reasoning as criticality
		}
		stats.BySource[src] += row.Count
	}

	// --- free-text types, capped -------------------------------------------
	stats.DistinctTypes = int64(len(typeCounts))
	kept := topN(typeCounts, domain.AssetTypeCap)
	for _, k := range kept {
		stats.ByType[k] = typeCounts[k]
	}
	if n := int64(len(typeCounts)) - int64(len(kept)); n > 0 {
		stats.TypesTruncated = n
	}

	// --- the one period-scoped counter -------------------------------------
	if from != nil || to != nil {
		q := r.db.WithContext(ctx).
			Model(&domain.Asset{}).
			Where("tenant_id = ? AND deleted_at IS NULL", tenantID)
		if from != nil {
			q = q.Where("created_at >= ?", *from) // inclusive
		}
		if to != nil {
			q = q.Where("created_at < ?", *to) // exclusive — see internal/domain/timeframe
		}
		if err := q.Count(&stats.AddedInPeriod).Error; err != nil {
			return nil, fmt.Errorf("failed to count assets added in period: %w", err)
		}
	} else {
		stats.AddedInPeriod = stats.Total
	}

	return stats, nil
}

// topN returns the n keys with the highest counts, ties broken alphabetically so
// the same inventory always produces the same breakdown (an unstable order makes
// a chart's colours shuffle between refreshes for no reason).
func topN(counts map[string]int64, n int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	return keys
}
