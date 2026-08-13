// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	if framework.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	err := r.db.WithContext(ctx).Create(framework).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.NewConflictError("framework", "name+version")
	}
	return err
}

// GetFrameworkByID retrieves a framework by ID scoped to a tenant.
// Returns (nil, nil) if not found or it belongs to another tenant.
func (r *GormComplianceRepository) GetFrameworkByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
	var fw domain.ComplianceFramework
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&fw).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get framework: %w", err)
	}
	return &fw, nil
}

// ListFrameworks returns a tenant's active (non-deleted) frameworks.
func (r *GormComplianceRepository) ListFrameworks(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error) {
	var frameworks []domain.ComplianceFramework
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
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
		First(&control).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found for this tenant → nil, nil
		}
		return nil, fmt.Errorf("failed to get control: %w", err)
	}

	// EvidenceCount counts the artifacts that CURRENTLY substantiate this control,
	// from the evidence library. It replaces the old Evidences preload, which
	// carried every attachment ever made regardless of whether it had expired.
	//
	// The count is what the "cannot mark implemented without proof" rule reads, so
	// a failure here must not silently report zero and block a legitimate edit:
	// the error propagates.
	var n int64
	if err := r.db.WithContext(ctx).
		Table("evidence_control_links l").
		Joins("JOIN evidences e ON e.id = l.evidence_id AND e.tenant_id = l.tenant_id AND e.deleted_at IS NULL").
		Where("l.tenant_id = ? AND l.control_id = ?", tenantID, id).
		Where("e.review = ? AND (e.valid_until IS NULL OR e.valid_until > ?)", domain.EvidenceReviewAccepted, time.Now()).
		Count(&n).Error; err != nil {
		return nil, fmt.Errorf("failed to count control evidence: %w", err)
	}
	control.EvidenceCount = int(n)

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

// SearchControls is a lightweight, tenant-scoped substring search over a control's
// reference code / name / description, for the universal search palette. Concrete
// method (off the ComplianceRepository port) so mocks stay intact.
func (r *GormComplianceRepository) SearchControls(ctx context.Context, tenantID uuid.UUID, q string, limit int) ([]domain.ComplianceControl, error) {
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	var controls []domain.ComplianceControl
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Where("LOWER(reference_code) LIKE ? OR LOWER(name) LIKE ? OR LOWER(COALESCE(description,'')) LIKE ?", like, like, like).
		Order("reference_code ASC").
		Limit(limit).
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

// DeleteFramework soft-deletes a framework by ID scoped to a tenant.
func (r *GormComplianceRepository) DeleteFramework(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
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

// CountEvidencesByFramework returns, per control of a (tenant, framework) pair,
// the number of artifacts that CURRENTLY substantiate it.
//
// Two things changed here when the evidence library landed (migration 0052).
//
// It reads the library, not control_evidences. There is one evidence register
// now; counting the old table would report zero for everything uploaded since,
// and the number gates whether a control may be marked implemented.
//
// And it counts COVERING artifacts, not attachments: expired and rejected proof
// is excluded, exactly as domain.Evidence.Covers defines it. That is a
// deliberate tightening. A control whose only certificate lapsed last quarter
// stops counting as evidenced, which means it can no longer be moved to
// "implemented" on the strength of it, and it surfaces on the missing-evidence
// worklist. The alternative — counting a file nobody could defend to an auditor
// — is the failure mode this module exists to remove.
//
// "Currently" is evaluated against time.Now() inside the repository because the
// port carries no clock. Callers that need a specific instant use
// domain.EvidenceRepository.CountCoveringByFramework, which takes one.
func (r *GormComplianceRepository) CountEvidencesByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (map[uuid.UUID]int, error) {
	type row struct {
		ControlID uuid.UUID
		Count     int
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("evidence_control_links l").
		Select("l.control_id AS control_id, COUNT(*) AS count").
		Joins("JOIN evidences e ON e.id = l.evidence_id AND e.tenant_id = l.tenant_id AND e.deleted_at IS NULL").
		Joins("JOIN compliance_controls c ON c.id = l.control_id AND c.tenant_id = l.tenant_id AND c.deleted_at IS NULL").
		Where("l.tenant_id = ? AND c.framework_id = ?", tenantID, frameworkID).
		Where("e.review = ? AND (e.valid_until IS NULL OR e.valid_until > ?)", domain.EvidenceReviewAccepted, time.Now()).
		Group("l.control_id").
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
