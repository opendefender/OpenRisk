// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormRiskRepository implements domain.RiskRepository using GORM.
// ABSOLUTE RULES:
// - Filter by tenant_id on EVERY query (never in handler)
// - If risk belongs to another tenant → return 404 (never 403)
// - Create audit entries on mutations
type GormRiskRepository struct {
	db *gorm.DB
}

// NewGormRiskRepository creates a new GORM-backed risk repository.
func NewGormRiskRepository(db *gorm.DB) *GormRiskRepository {
	return &GormRiskRepository{db: db}
}

// =============================================================================
// CRUD Operations
// =============================================================================

// Create persists a new risk.
func (r *GormRiskRepository) Create(ctx context.Context, risk *domain.Risk) error {
	// Validate tenant_id is set
	if risk.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}

	return r.db.WithContext(ctx).Create(risk).Error
}

// GetByID retrieves a risk by ID scoped to a tenant.
// Returns (nil, nil) if not found (use case handles 404).
func (r *GormRiskRepository) GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Risk, error) {
	var risk domain.Risk
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Mitigations").
		Preload("Assets").
		First(&risk).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found → nil, nil (use case handles this)
		}
		return nil, fmt.Errorf("failed to get risk: %w", err)
	}
	return &risk, nil
}

// List retrieves risks with filtering, pagination, and sorting scoped to a tenant.
func (r *GormRiskRepository) List(ctx context.Context, tenantID uuid.UUID, query domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
	query.Sanitize()

	db := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("tenant_id = ?", tenantID)

	// Apply filters
	if query.Search != "" {
		// Use PostgreSQL full-text search for better performance
		// tsvector on (name || ' ' || description) with 'french' language
		db = db.Where(
			"to_tsvector(?, name || ' ' || COALESCE(description, '')) @@ plainto_tsquery(?, ?)",
			query.SearchLanguage,
			query.SearchLanguage,
			query.Search,
		)
	}

	// Status filter (supports multiple statuses)
	if len(query.Status) > 0 {
		db = db.Where("status IN ?", query.Status)
	}

	// Criticality filter
	if len(query.Criticality) > 0 {
		db = db.Where("criticality IN ?", query.Criticality)
	}

	// Framework filter (single framework)
	if query.Framework != "" {
		db = db.Where("? = ANY(frameworks)", query.Framework)
	}

	// Asset filter
	if query.AssetID != nil {
		db = db.Where("asset_id = ?", *query.AssetID)
	}

	// Assigned to filter
	if query.AssignedTo != nil {
		db = db.Where("assigned_to = ?", *query.AssignedTo)
	}

	// Tags filter (OR condition)
	if len(query.Tags) > 0 {
		db = db.Where("tags && ?", pq.Array(query.Tags))
	}

	// Date range filter
	if query.DateFrom != nil {
		db = db.Where("created_at >= ?", *query.DateFrom)
	}
	if query.DateTo != nil {
		db = db.Where("created_at <= ?", *query.DateTo)
	}

	// Source filter (multiple sources)
	if len(query.Source) > 0 {
		db = db.Where("source IN ?", query.Source)
	}

	// Score range filter
	if query.MinScore != nil {
		db = db.Where("score >= ?", *query.MinScore)
	}
	if query.MaxScore != nil {
		db = db.Where("score <= ?", *query.MaxScore)
	}

	// Treatment plan filter
	if len(query.TreatmentPlan) > 0 {
		db = db.Where("treatment_plan IN ?", query.TreatmentPlan)
	}

	// Count total before pagination
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count risks: %w", err)
	}

	// Apply pagination and sorting
	var risks []domain.Risk
	err := db.
		Order(fmt.Sprintf("%s %s", query.SortBy, query.SortOrder)).
		Offset(query.Offset()).
		Limit(query.Limit).
		Preload("Mitigations").
		Preload("Assets").
		Find(&risks).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list risks: %w", err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(query.Limit)))

	return &domain.PaginatedResult[domain.Risk]{
		Data:       risks,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}, nil
}

// Update updates an existing risk.
func (r *GormRiskRepository) Update(ctx context.Context, risk *domain.Risk) error {
	return r.db.WithContext(ctx).Save(risk).Error
}

// Delete soft-deletes a risk by ID scoped to a tenant.
func (r *GormRiskRepository) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.Risk{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete risk: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("risk not found")
	}
	return nil
}

// Count returns the total number of risks for a tenant.
func (r *GormRiskRepository) Count(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("tenant_id = ?", tenantID).
		Count(&count).Error
	return count, err
}

// =============================================================================
// Scoring Operations (called by Score Engine worker)
// =============================================================================

// UpdateScore updates the score and criticality fields for a risk.
// Called exclusively by the Score Engine worker after Redis event triggers recalculation.
func (r *GormRiskRepository) UpdateScore(ctx context.Context, riskID uuid.UUID, tenantID uuid.UUID, score float64, criticality string) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("id = ? AND tenant_id = ?", riskID, tenantID).
		Update("score", score).
		Update("criticality", criticality).
		Update("updated_at", time.Now())

	if result.Error != nil {
		return fmt.Errorf("failed to update risk score: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("risk not found (tenant isolation)")
	}
	return nil
}

// GetRiskScore retrieves the current score of a risk.
func (r *GormRiskRepository) GetRiskScore(ctx context.Context, riskID uuid.UUID, tenantID uuid.UUID) (float64, error) {
	var score float64
	err := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("id = ? AND tenant_id = ?", riskID, tenantID).
		Select("score").
		Scan(&score).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, fmt.Errorf("risk not found")
		}
		return 0, fmt.Errorf("failed to get risk score: %w", err)
	}
	return score, nil
}

// GetRisksByAssetID retrieves all risks linked to an asset.
func (r *GormRiskRepository) GetRisksByAssetID(ctx context.Context, assetID uuid.UUID, tenantID uuid.UUID) ([]domain.RiskForScoring, error) {
	var risks []domain.RiskForScoring

	// Join with assets table to handle many2many relationship
	err := r.db.WithContext(ctx).
		Table("risks").
		Select(
			"risks.id,"+
				"risks.tenant_id,"+
				"risks.probability,"+
				"risks.impact,"+
				"assets.criticality as asset_criticality,"+
				"risks.score as current_score",
		).
		Joins("JOIN risk_assets ON risks.id = risk_assets.risk_id").
		Joins("JOIN assets ON risk_assets.asset_id = assets.id").
		Where("risks.tenant_id = ? AND assets.id = ?", tenantID, assetID).
		Scan(&risks).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get risks by asset ID: %w", err)
	}
	return risks, nil
}

// =============================================================================
// Audit & History Operations
// =============================================================================

// GetHistory retrieves paginated audit log entries for a risk.
func (r *GormRiskRepository) GetHistory(ctx context.Context, riskID uuid.UUID, tenantID uuid.UUID, page, limit int) ([]domain.AuditLogEntry, error) {
	// Validate pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var history []domain.AuditLogEntry
	err := r.db.WithContext(ctx).
		Table("audit_logs").
		Where("risk_id = ?", riskID).
		// NOTE: Add tenant_id filter if audit_logs table has tenant_id column
		Order("timestamp DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&history).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get risk history: %w", err)
	}
	return history, nil
}

// CreateAuditEntry creates an audit log entry for a risk change.
func (r *GormRiskRepository) CreateAuditEntry(ctx context.Context, entry *domain.AuditLogEntry) error {
	return r.db.WithContext(ctx).
		Table("audit_logs").
		Create(entry).Error
}

// =============================================================================
// Advanced Queries
// =============================================================================

// GetBySource retrieves risks filtered by source.
func (r *GormRiskRepository) GetBySource(ctx context.Context, tenantID uuid.UUID, source string) ([]domain.Risk, error) {
	var risks []domain.Risk
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND source = ?", tenantID, source).
		Find(&risks).Error
	return risks, err
}

// GetByCVE retrieves a risk by CVE ID.
func (r *GormRiskRepository) GetByCVE(ctx context.Context, cveID string, tenantID uuid.UUID) (*domain.Risk, error) {
	var risk domain.Risk
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND source_cve_id = ?", tenantID, cveID).
		First(&risk).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get risk by CVE: %w", err)
	}
	return &risk, nil
}

// BulkUpdate updates multiple risks atomically within a transaction.
func (r *GormRiskRepository) BulkUpdate(ctx context.Context, tenantID uuid.UUID, updates []domain.RiskUpdate) (int64, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}

	updatedCount := int64(0)
	for _, update := range updates {
		db := tx.Model(&domain.Risk{}).
			Where("id = ? AND tenant_id = ?", update.ID, tenantID)

		if update.Status != nil {
			db = db.Update("status", *update.Status)
		}
		if update.Score != nil {
			db = db.Update("score", *update.Score)
		}

		if db.Error != nil {
			tx.Rollback()
			return 0, fmt.Errorf("failed to update risk %s: %w", update.ID, db.Error)
		}
		updatedCount += db.RowsAffected
	}

	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("failed to commit bulk update: %w", err)
	}

	return updatedCount, nil
}

// BulkCreate creates multiple risks atomically within a transaction.
func (r *GormRiskRepository) BulkCreate(ctx context.Context, risks []*domain.Risk) (int64, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}

	if err := tx.CreateInBatches(risks, 100).Error; err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to bulk create risks: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("failed to commit bulk create: %w", err)
	}

	return int64(len(risks)), nil
}

// BulkDelete soft-deletes multiple risks atomically.
func (r *GormRiskRepository) BulkDelete(ctx context.Context, ids []uuid.UUID, tenantID uuid.UUID) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("id IN ? AND tenant_id = ?", ids, tenantID).
		Delete(&domain.Risk{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to bulk delete risks: %w", result.Error)
	}

	return result.RowsAffected, nil
}
