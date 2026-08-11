// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

// GormRiskCategoryRepository persists the tenant's controlled classification
// vocabulary. Tenant-scoped on EVERY query (RULE #2) — a category id forged from
// another organisation reads back as not found.
type GormRiskCategoryRepository struct {
	db *gorm.DB
}

func NewGormRiskCategoryRepository(db *gorm.DB) *GormRiskCategoryRepository {
	return &GormRiskCategoryRepository{db: db}
}

func (r *GormRiskCategoryRepository) Create(ctx context.Context, c *domain.RiskCategory) error {
	if c.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *GormRiskCategoryRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RiskCategory, error) {
	var c domain.RiskCategory
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&c).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *GormRiskCategoryRepository) List(ctx context.Context, tenantID uuid.UUID, includeInactive bool) ([]domain.RiskCategory, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if !includeInactive {
		q = q.Where("active = ?", true)
	}
	var out []domain.RiskCategory
	if err := q.Order("sort_order ASC, name ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *GormRiskCategoryRepository) Update(ctx context.Context, c *domain.RiskCategory) error {
	res := r.db.WithContext(ctx).
		Model(&domain.RiskCategory{}).
		Where("id = ? AND tenant_id = ?", c.ID, c.TenantID).
		Updates(map[string]interface{}{
			"name":        c.Name,
			"slug":        c.Slug,
			"description": c.Description,
			"color":       c.Color,
			"sort_order":  c.SortOrder,
			"active":      c.Active,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("risk category", c.ID)
	}
	return nil
}

// Delete removes a category and DETACHES the risks that carried it in the same
// transaction. Leaving risks pointing at a deleted row would render an empty
// "Catégorie" cell that no longer explains itself.
func (r *GormRiskCategoryRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Risk{}).
			Where("category_id = ? AND tenant_id = ?", id, tenantID).
			Update("category_id", nil).Error; err != nil {
			return err
		}
		res := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&domain.RiskCategory{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.NewNotFoundError("risk category", id)
		}
		return nil
	})
}

func (r *GormRiskCategoryRepository) ExistsBySlug(ctx context.Context, tenantID uuid.UUID, slug string, excludeID *uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&domain.RiskCategory{}).
		Where("tenant_id = ? AND slug = ?", tenantID, slug)
	if excludeID != nil {
		q = q.Where("id <> ?", *excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *GormRiskCategoryRepository) CountRisks(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID]int64, error) {
	type row struct {
		CategoryID uuid.UUID
		N          int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Select("category_id, COUNT(*) AS n").
		Where("tenant_id = ? AND category_id IS NOT NULL AND deleted_at IS NULL", tenantID).
		Group("category_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, x := range rows {
		out[x.CategoryID] = x.N
	}
	return out, nil
}

// SeedDefaults gives a tenant a starting vocabulary. Idempotent: a tenant that
// already has ANY category (even a deactivated one) is left alone, so an admin
// who deliberately emptied the list does not get it back on the next boot.
func (r *GormRiskCategoryRepository) SeedDefaults(ctx context.Context, tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	var n int64
	if err := r.db.WithContext(ctx).Model(&domain.RiskCategory{}).
		Where("tenant_id = ?", tenantID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	rows := domain.DefaultRiskCategories()
	for i := range rows {
		rows[i].ID = uuid.New()
		rows[i].TenantID = tenantID
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

// ---------------------------------------------------------------------------
// Risk ↔ control mappings
// ---------------------------------------------------------------------------

// GormRiskControlMappingRepository persists the links backing the "Référentiel"
// column. Tenant-scoped throughout.
type GormRiskControlMappingRepository struct {
	db *gorm.DB
}

func NewGormRiskControlMappingRepository(db *gorm.DB) *GormRiskControlMappingRepository {
	return &GormRiskControlMappingRepository{db: db}
}

func (r *GormRiskControlMappingRepository) Create(ctx context.Context, m *domain.RiskControlMapping) error {
	if m.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *GormRiskControlMappingRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RiskControlMapping, error) {
	var m domain.RiskControlMapping
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *GormRiskControlMappingRepository) ListByRisk(ctx context.Context, tenantID, riskID uuid.UUID) ([]domain.RiskControlMapping, error) {
	byRisk, err := r.ListByRisks(ctx, tenantID, []uuid.UUID{riskID})
	if err != nil {
		return nil, err
	}
	return byRisk[riskID], nil
}

// ListByRisks resolves a whole page of the register in ONE query, joining the
// framework and control names the badge needs. A per-row lookup here would be an
// N+1 on the register's hot path.
func (r *GormRiskControlMappingRepository) ListByRisks(ctx context.Context, tenantID uuid.UUID, riskIDs []uuid.UUID) (map[uuid.UUID][]domain.RiskControlMapping, error) {
	out := map[uuid.UUID][]domain.RiskControlMapping{}
	if len(riskIDs) == 0 {
		return out, nil
	}

	type row struct {
		domain.RiskControlMapping
		FrameworkNameJoined string
		ControlCodeJoined   string
		ControlNameJoined   string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("risk_control_mappings AS m").
		Select(`m.*,
			f.name AS framework_name_joined,
			c.reference_code AS control_code_joined,
			c.name AS control_name_joined`).
		Joins("LEFT JOIN compliance_frameworks f ON f.id = m.framework_id AND f.tenant_id = m.tenant_id").
		Joins("LEFT JOIN compliance_controls c ON c.id = m.control_id AND c.tenant_id = m.tenant_id").
		Where("m.tenant_id = ? AND m.risk_id IN ? AND m.deleted_at IS NULL", tenantID, riskIDs).
		Order("m.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list risk control mappings: %w", err)
	}

	for _, x := range rows {
		m := x.RiskControlMapping
		m.FrameworkName = x.FrameworkNameJoined
		m.ControlCode = x.ControlCodeJoined
		m.ControlName = x.ControlNameJoined
		out[m.RiskID] = append(out[m.RiskID], m)
	}
	return out, nil
}

func (r *GormRiskControlMappingRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.RiskControlMapping{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("risk control mapping", id)
	}
	return nil
}

func (r *GormRiskControlMappingRepository) Exists(ctx context.Context, tenantID, riskID, frameworkID uuid.UUID, controlID *uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&domain.RiskControlMapping{}).
		Where("tenant_id = ? AND risk_id = ? AND framework_id = ?", tenantID, riskID, frameworkID)
	if controlID == nil {
		q = q.Where("control_id IS NULL")
	} else {
		q = q.Where("control_id = ?", *controlID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// UnmappedRiskIDs backs /risks/unmapped: every live risk of the tenant with no
// mapping at all. Closed risks are excluded — chasing the compliance mapping of
// something already resolved is busywork.
func (r *GormRiskControlMappingRepository) UnmappedRiskIDs(ctx context.Context, tenantID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Where("lifecycle_state <> ?", domain.StateClosed).
		Where(`NOT EXISTS (
			SELECT 1 FROM risk_control_mappings m
			WHERE m.risk_id = risks.id AND m.tenant_id = risks.tenant_id AND m.deleted_at IS NULL
		)`).
		Order("score DESC").
		Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list unmapped risks: %w", err)
	}
	return ids, nil
}
