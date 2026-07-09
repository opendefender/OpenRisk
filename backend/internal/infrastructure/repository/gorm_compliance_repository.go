// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"errors"
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
// Returns a domain.ErrConflict-typed error if (name, version) already
// exists — the DB unique index is the authoritative guard (no TOCTOU gap),
// use cases may still pre-check for a faster, friendlier error message.
func (r *GormComplianceRepository) CreateFramework(ctx context.Context, framework *domain.ComplianceFramework) error {
	err := r.db.WithContext(ctx).Create(framework).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.NewConflictError("framework", "name+version")
	}
	return err
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
// Returns a domain.ErrConflict-typed error if (tenant_id, framework_id,
// reference_code) already exists — see CreateFramework's doc comment for
// why this is checked at the DB level, not just pre-checked in Go.
func (r *GormComplianceRepository) CreateControl(ctx context.Context, control *domain.ComplianceControl) error {
	if control.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	err := r.db.WithContext(ctx).Create(control).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.NewConflictError("control", "reference_code")
	}
	return err
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
//
// NOTE: GORM's Save() ignores a chained Where() once the model's primary key
// is set — it derives its own WHERE clause from the PK alone, which would let
// one tenant overwrite another tenant's row by ID. Model()+Where()+Updates()
// (with an explicit Select) is the pattern that actually honors the WHERE
// clause, so tenant scoping is enforced at the SQL level, not just in Go.
func (r *GormComplianceRepository) UpdateControl(ctx context.Context, control *domain.ComplianceControl) error {
	if control.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}

	result := r.db.WithContext(ctx).
		Model(&domain.ComplianceControl{}).
		Where("id = ? AND tenant_id = ?", control.ID, control.TenantID).
		Select("reference_code", "name", "description", "status").
		Updates(control)

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

// DeleteFramework soft-deletes a framework by ID (global — no tenant filter).
func (r *GormComplianceRepository) DeleteFramework(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&domain.ComplianceFramework{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete framework: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("framework not found")
	}
	return nil
}

// DeleteControlsByFramework soft-deletes all of a tenant's controls under a framework.
func (r *GormComplianceRepository) DeleteControlsByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND framework_id = ?", tenantID, frameworkID).
		Delete(&domain.ComplianceControl{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete controls by framework: %w", result.Error)
	}
	return result.RowsAffected, nil
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

// CountEvidencesByFramework returns evidence counts per control for a (tenant, framework)
// pair in a single grouped query. Joins evidences to their controls so the framework and
// tenant filters both apply; soft-deleted rows on either side are excluded.
func (r *GormComplianceRepository) CountEvidencesByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (map[uuid.UUID]int, error) {
	type row struct {
		ControlID uuid.UUID
		Count     int
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&domain.ControlEvidence{}).
		Select("control_evidences.control_id AS control_id, COUNT(*) AS count").
		Joins("JOIN compliance_controls ON compliance_controls.id = control_evidences.control_id AND compliance_controls.deleted_at IS NULL").
		Where("control_evidences.tenant_id = ? AND compliance_controls.framework_id = ? AND compliance_controls.tenant_id = ?", tenantID, frameworkID, tenantID).
		Group("control_evidences.control_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count evidences by framework: %w", err)
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		counts[r.ControlID] = r.Count
	}
	return counts, nil
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
