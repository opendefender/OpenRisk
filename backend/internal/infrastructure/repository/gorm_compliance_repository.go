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

// GormComplianceRepository implements domain.ComplianceRepository using GORM.
// ABSOLUTE RULES:
// - Filter by tenant_id on EVERY tenant-scoped query (controls, evidences)
// - If resource belongs to another tenant → return nil (not found), never 403
type GormComplianceRepository struct {
	db *gorm.DB
}

// NewGormComplianceRepository creates a new GORM-backed compliance repository.
func NewGormComplianceRepository(db *gorm.DB) *GormComplianceRepository {
	return &GormComplianceRepository{db: db}
}

// =============================================================================
// Frameworks (global — no tenant_id filtering)
// =============================================================================

// CreateFramework persists a new compliance framework.
func (r *GormComplianceRepository) CreateFramework(ctx context.Context, framework *domain.ComplianceFramework) error {
	return r.db.WithContext(ctx).Create(framework).Error
}

// GetFrameworkByID retrieves a framework by ID.
// Returns (nil, nil) if not found.
func (r *GormComplianceRepository) GetFrameworkByID(ctx context.Context, id uuid.UUID) (*domain.ComplianceFramework, error) {
	var fw domain.ComplianceFramework
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&fw).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get framework: %w", err)
	}
	return &fw, nil
}

// ListFrameworks returns all active (non-deleted) frameworks.
func (r *GormComplianceRepository) ListFrameworks(ctx context.Context) ([]domain.ComplianceFramework, error) {
	var frameworks []domain.ComplianceFramework
	err := r.db.WithContext(ctx).
		Order("name ASC").
		Find(&frameworks).Error
	return frameworks, err
}

// =============================================================================
// Controls (tenant-scoped — ALWAYS filter by tenant_id)
// =============================================================================

// CreateControl persists a new compliance control for a tenant.
func (r *GormComplianceRepository) CreateControl(ctx context.Context, control *domain.ComplianceControl) error {
	if control.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(control).Error
}

// GetControlByID retrieves a control by ID scoped to a tenant.
// Returns (nil, nil) if not found or belongs to another tenant.
func (r *GormComplianceRepository) GetControlByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceControl, error) {
	var control domain.ComplianceControl
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Framework").
		Preload("Evidences").
		First(&control).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found for this tenant → nil, nil
		}
		return nil, fmt.Errorf("failed to get control: %w", err)
	}
	return &control, nil
}

// ListControlsByFramework retrieves all controls for a (tenant, framework) pair.
func (r *GormComplianceRepository) ListControlsByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error) {
	var controls []domain.ComplianceControl
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND framework_id = ?", tenantID, frameworkID).
		Order("reference_code ASC").
		Find(&controls).Error
	return controls, err
}

// UpdateControl updates an existing control.
// MANDATORY: tenant_id is included in the WHERE clause via the struct's own TenantID.
func (r *GormComplianceRepository) UpdateControl(ctx context.Context, control *domain.ComplianceControl) error {
	if control.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}

	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", control.ID, control.TenantID).
		Save(control)

	if result.Error != nil {
		return fmt.Errorf("failed to update control: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("control not found")
	}
	return nil
}

// DeleteControl soft-deletes a control by ID scoped to a tenant.
func (r *GormComplianceRepository) DeleteControl(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ComplianceControl{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete control: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("control not found")
	}
	return nil
}

// =============================================================================
// Evidences (tenant-scoped — ALWAYS filter by tenant_id)
// =============================================================================

// CreateEvidence persists a new control evidence for a tenant.
func (r *GormComplianceRepository) CreateEvidence(ctx context.Context, evidence *domain.ControlEvidence) error {
	if evidence.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(evidence).Error
}

// GetEvidenceByID retrieves an evidence by ID scoped to a tenant.
// Returns (nil, nil) if not found or belongs to another tenant.
func (r *GormComplianceRepository) GetEvidenceByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ControlEvidence, error) {
	var evidence domain.ControlEvidence
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&evidence).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get evidence: %w", err)
	}
	return &evidence, nil
}

// ListEvidencesByControl retrieves all evidences for a (tenant, control) pair.
func (r *GormComplianceRepository) ListEvidencesByControl(ctx context.Context, tenantID uuid.UUID, controlID uuid.UUID) ([]domain.ControlEvidence, error) {
	var evidences []domain.ControlEvidence
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND control_id = ?", tenantID, controlID).
		Order("created_at DESC").
		Find(&evidences).Error
	return evidences, err
}

// DeleteEvidence soft-deletes an evidence by ID scoped to a tenant.
func (r *GormComplianceRepository) DeleteEvidence(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ControlEvidence{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete evidence: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("evidence not found")
	}
	return nil
}
