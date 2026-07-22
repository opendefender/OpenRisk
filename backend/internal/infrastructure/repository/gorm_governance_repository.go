// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// Governance stores (spec §15). Three focused repositories — the immutable audit
// trail, delegations, and the approval engine (workflows + requests) — each
// tenant-scoped on every query (RULE #2): a resource owned by another tenant
// reads back as not found. The audit_events store is append-only (no Update/Delete).

// =============================================================================
// Audit trail (append-only)
// =============================================================================

type GormAuditEventRepository struct{ db *gorm.DB }

func NewGormAuditEventRepository(db *gorm.DB) *GormAuditEventRepository {
	return &GormAuditEventRepository{db: db}
}

func (r *GormAuditEventRepository) Append(ctx context.Context, e *domain.AuditEvent) error {
	if e.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	// SkipHooks so an explicit Append never re-enters the audittrail plugin.
	return r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).Create(e).Error
}

func (r *GormAuditEventRepository) List(ctx context.Context, tenantID uuid.UUID, f domain.AuditEventFilter) ([]domain.AuditEvent, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.AuditEvent{}).Where("tenant_id = ?", tenantID)
	if f.EntityType != "" {
		q = q.Where("entity_type = ?", f.EntityType)
	}
	if f.EntityID != "" {
		q = q.Where("entity_id = ?", f.EntityID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.ActorID != nil {
		q = q.Where("actor_id = ?", *f.ActorID)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("LOWER(summary) LIKE ? OR LOWER(entity_type) LIKE ? OR LOWER(entity_id) LIKE ?", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit events: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var events []domain.AuditEvent
	if err := q.Order("created_at DESC").Limit(limit).Offset(f.Offset).Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list audit events: %w", err)
	}
	return events, total, nil
}

// =============================================================================
// Delegations
// =============================================================================

type GormDelegationRepository struct{ db *gorm.DB }

func NewGormDelegationRepository(db *gorm.DB) *GormDelegationRepository {
	return &GormDelegationRepository{db: db}
}

func (r *GormDelegationRepository) Create(ctx context.Context, d *domain.Delegation) error {
	if d.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *GormDelegationRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Delegation, error) {
	var d domain.Delegation
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&d).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get delegation: %w", err)
	}
	return &d, nil
}

func (r *GormDelegationRepository) List(ctx context.Context, tenantID uuid.UUID, f domain.DelegationFilter) ([]domain.Delegation, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if f.DelegatorID != nil {
		q = q.Where("delegator_id = ?", *f.DelegatorID)
	}
	if f.DelegateID != nil {
		q = q.Where("delegate_id = ?", *f.DelegateID)
	}
	if f.ActiveOnly {
		q = q.Where("status = ?", domain.DelegationActive)
	}
	var out []domain.Delegation
	if err := q.Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("failed to list delegations: %w", err)
	}
	return out, nil
}

func (r *GormDelegationRepository) Update(ctx context.Context, d *domain.Delegation) error {
	res := r.db.WithContext(ctx).
		Model(&domain.Delegation{}).
		Where("id = ? AND tenant_id = ?", d.ID, d.TenantID).
		Updates(map[string]interface{}{
			"reason":      d.Reason,
			"permissions": d.Permissions,
			"status":      d.Status,
			"starts_at":   d.StartsAt,
			"ends_at":     d.EndsAt,
			"revoked_at":  d.RevokedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update delegation: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("delegation", d.ID)
	}
	return nil
}

// =============================================================================
// Approval engine — workflows (config) + requests (runtime state machine).
// Method names are unique across both interfaces so one struct implements both.
// =============================================================================

type GormApprovalRepository struct{ db *gorm.DB }

func NewGormApprovalRepository(db *gorm.DB) *GormApprovalRepository {
	return &GormApprovalRepository{db: db}
}

func (r *GormApprovalRepository) CreateWorkflow(ctx context.Context, w *domain.ApprovalWorkflow) error {
	if w.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(w).Error
}

func (r *GormApprovalRepository) GetWorkflowByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ApprovalWorkflow, error) {
	var w domain.ApprovalWorkflow
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&w).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	return &w, nil
}

func (r *GormApprovalRepository) ListWorkflows(ctx context.Context, tenantID uuid.UUID) ([]domain.ApprovalWorkflow, error) {
	var out []domain.ApprovalWorkflow
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	return out, nil
}

// FindWorkflow returns the enabled workflow governing (entity_type, action), or
// nil when none is configured (the action needs no approval).
func (r *GormApprovalRepository) FindWorkflow(ctx context.Context, tenantID uuid.UUID, entityType, action string) (*domain.ApprovalWorkflow, error) {
	var w domain.ApprovalWorkflow
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND entity_type = ? AND enabled = ?", tenantID, entityType, true)
	if action != "" {
		q = q.Where("action = ? OR action = ''", action)
	}
	err := q.Order("created_at DESC").First(&w).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}
	return &w, nil
}

func (r *GormApprovalRepository) UpdateWorkflow(ctx context.Context, w *domain.ApprovalWorkflow) error {
	res := r.db.WithContext(ctx).
		Model(&domain.ApprovalWorkflow{}).
		Where("id = ? AND tenant_id = ?", w.ID, w.TenantID).
		Updates(map[string]interface{}{
			"name":        w.Name,
			"description": w.Description,
			"entity_type": w.EntityType,
			"action":      w.Action,
			"enabled":     w.Enabled,
			"steps":       w.Steps,
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update workflow: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("workflow", w.ID)
	}
	return nil
}

func (r *GormApprovalRepository) DeleteWorkflow(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ApprovalWorkflow{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete workflow: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("workflow", id)
	}
	return nil
}

func (r *GormApprovalRepository) CreateRequest(ctx context.Context, req *domain.ApprovalRequest) error {
	if req.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(req).Error
}

func (r *GormApprovalRepository) GetRequestByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ApprovalRequest, error) {
	var req domain.ApprovalRequest
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&req).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get approval request: %w", err)
	}
	return &req, nil
}

func (r *GormApprovalRepository) ListRequests(ctx context.Context, tenantID uuid.UUID, f domain.ApprovalRequestFilter) ([]domain.ApprovalRequest, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.EntityType != "" {
		q = q.Where("entity_type = ?", f.EntityType)
	}
	if f.RequestedBy != nil {
		q = q.Where("requested_by = ?", *f.RequestedBy)
	}
	var out []domain.ApprovalRequest
	if err := q.Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("failed to list approval requests: %w", err)
	}
	return out, nil
}

func (r *GormApprovalRepository) UpdateRequest(ctx context.Context, req *domain.ApprovalRequest) error {
	res := r.db.WithContext(ctx).
		Model(&domain.ApprovalRequest{}).
		Where("id = ? AND tenant_id = ?", req.ID, req.TenantID).
		Updates(map[string]interface{}{
			"status":       req.Status,
			"current_step": req.CurrentStep,
			"decisions":    req.Decisions,
			"resolved_at":  req.ResolvedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update approval request: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("approval request", req.ID)
	}
	return nil
}
