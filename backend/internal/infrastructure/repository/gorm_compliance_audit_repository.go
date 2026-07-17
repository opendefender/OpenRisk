// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormComplianceAuditRepository implements domain.ComplianceAuditRepository and
// domain.RemediationPlanRepository using GORM. Both are tenant-scoped on every
// query — a resource owned by another tenant reads back as not found (nil, nil).
type GormComplianceAuditRepository struct {
	db *gorm.DB
}

func NewGormComplianceAuditRepository(db *gorm.DB) *GormComplianceAuditRepository {
	return &GormComplianceAuditRepository{db: db}
}

// =============================================================================
// Audits
// =============================================================================

func (r *GormComplianceAuditRepository) CreateAudit(ctx context.Context, a *domain.ComplianceAudit) error {
	if a.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *GormComplianceAuditRepository) GetAuditByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceAudit, error) {
	var a domain.ComplianceAudit
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&a).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get audit: %w", err)
	}
	return &a, nil
}

func (r *GormComplianceAuditRepository) ListAudits(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceAudit, error) {
	var audits []domain.ComplianceAudit
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("COALESCE(scheduled_start, created_at) DESC").
		Find(&audits).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list audits: %w", err)
	}
	return audits, nil
}

func (r *GormComplianceAuditRepository) UpdateAudit(ctx context.Context, a *domain.ComplianceAudit) error {
	// Scope the write to the tenant so a forged ID can never touch another
	// tenant's row. Save on a tenant-filtered query updates only the matched row.
	res := r.db.WithContext(ctx).
		Model(&domain.ComplianceAudit{}).
		Where("id = ? AND tenant_id = ?", a.ID, a.TenantID).
		Updates(map[string]interface{}{
			"title":            a.Title,
			"framework_id":     a.FrameworkID,
			"type":             a.Type,
			"status":           a.Status,
			"auditor":          a.Auditor,
			"scope":            a.Scope,
			"summary":          a.Summary,
			"compliance_score": a.ComplianceScore,
			"scheduled_start":  a.ScheduledStart,
			"scheduled_end":    a.ScheduledEnd,
			"completed_at":     a.CompletedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update audit: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("audit", a.ID)
	}
	return nil
}

func (r *GormComplianceAuditRepository) DeleteAudit(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ComplianceAudit{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete audit: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("audit", id)
	}
	return nil
}

// =============================================================================
// Remediation plans
// =============================================================================

func (r *GormComplianceAuditRepository) CreateRemediation(ctx context.Context, rp *domain.RemediationPlan) error {
	if rp.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(rp).Error
}

func (r *GormComplianceAuditRepository) GetRemediationByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RemediationPlan, error) {
	var rp domain.RemediationPlan
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&rp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get remediation plan: %w", err)
	}
	return &rp, nil
}

func (r *GormComplianceAuditRepository) ListRemediations(ctx context.Context, tenantID uuid.UUID, filter domain.RemediationFilter) ([]domain.RemediationPlan, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if filter.ControlID != nil {
		q = q.Where("control_id = ?", *filter.ControlID)
	}
	if filter.FrameworkID != nil {
		q = q.Where("framework_id = ?", *filter.FrameworkID)
	}
	if filter.AuditID != nil {
		q = q.Where("audit_id = ?", *filter.AuditID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	var plans []domain.RemediationPlan
	if err := q.Order("created_at DESC").Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("failed to list remediation plans: %w", err)
	}
	return plans, nil
}

func (r *GormComplianceAuditRepository) UpdateRemediation(ctx context.Context, rp *domain.RemediationPlan) error {
	res := r.db.WithContext(ctx).
		Model(&domain.RemediationPlan{}).
		Where("id = ? AND tenant_id = ?", rp.ID, rp.TenantID).
		Updates(map[string]interface{}{
			"title":        rp.Title,
			"description":  rp.Description,
			"control_id":   rp.ControlID,
			"framework_id": rp.FrameworkID,
			"audit_id":     rp.AuditID,
			"priority":     rp.Priority,
			"status":       rp.Status,
			"assigned_to":  rp.AssignedTo,
			"due_date":     rp.DueDate,
			"completed_at": rp.CompletedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update remediation plan: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("remediation plan", rp.ID)
	}
	return nil
}

func (r *GormComplianceAuditRepository) DeleteRemediation(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.RemediationPlan{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete remediation plan: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("remediation plan", id)
	}
	return nil
}
