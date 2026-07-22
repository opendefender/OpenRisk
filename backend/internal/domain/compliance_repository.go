// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"

	"github.com/google/uuid"
)

// ComplianceRepository defines the port for compliance data persistence.
// Infrastructure layer implements this interface.
//
// ABSOLUTE RULE: All tenant-scoped methods MUST filter by tenant_id in the repository,
// never in the handler. If a resource belongs to another tenant → return nil (not found).
type ComplianceRepository interface {
	// =========================================================================
	// Frameworks (tenant-scoped — ALWAYS filter by tenant_id)
	// =========================================================================

	// CreateFramework persists a new compliance framework. framework.TenantID
	// MUST be set by the caller.
	CreateFramework(ctx context.Context, framework *ComplianceFramework) error

	// GetFrameworkByID retrieves a framework by ID scoped to a tenant.
	// Returns (nil, nil) if not found or it belongs to another tenant.
	GetFrameworkByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*ComplianceFramework, error)

	// ListFrameworks returns a tenant's active (non-deleted) frameworks.
	ListFrameworks(ctx context.Context, tenantID uuid.UUID) ([]ComplianceFramework, error)

	// DeleteFramework soft-deletes a framework by ID scoped to a tenant — a
	// tenant can only delete its own. The delete use case pairs this with
	// DeleteControlsByFramework so the tenant's controls go away too.
	DeleteFramework(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error

	// =========================================================================
	// Controls (tenant-scoped)
	// =========================================================================

	// CreateControl persists a new compliance control for a tenant.
	CreateControl(ctx context.Context, control *ComplianceControl) error

	// GetControlByID retrieves a control by ID scoped to a tenant.
	// Returns (nil, nil) if not found or belongs to another tenant.
	GetControlByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*ComplianceControl, error)

	// ListControlsByFramework retrieves all controls for a (tenant, framework) pair.
	ListControlsByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) ([]ComplianceControl, error)

	// UpdateControl updates an existing control.
	// MANDATORY: Must include tenant_id in WHERE clause.
	UpdateControl(ctx context.Context, control *ComplianceControl) error

	// DeleteControl soft-deletes a control by ID scoped to a tenant.
	DeleteControl(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error

	// DeleteControlsByFramework soft-deletes every control a tenant owns under a
	// framework. Returns the number of controls deleted.
	DeleteControlsByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (int64, error)

	// =========================================================================
	// Evidences (tenant-scoped)
	// =========================================================================

	// CreateEvidence persists a new control evidence for a tenant.
	CreateEvidence(ctx context.Context, evidence *ControlEvidence) error

	// GetEvidenceByID retrieves an evidence by ID scoped to a tenant.
	// Returns (nil, nil) if not found or belongs to another tenant.
	GetEvidenceByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*ControlEvidence, error)

	// ListEvidencesByControl retrieves all evidences for a (tenant, control) pair.
	ListEvidencesByControl(ctx context.Context, tenantID uuid.UUID, controlID uuid.UUID) ([]ControlEvidence, error)

	// CountEvidencesByFramework returns, for a (tenant, framework) pair, the number of
	// evidences attached to each control, keyed by control ID. Controls with no evidence
	// are simply absent from the map. Used by the compliance report to show, in a single
	// query, which controls are substantiated — avoids N per-control lookups.
	CountEvidencesByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (map[uuid.UUID]int, error)

	// DeleteEvidence soft-deletes an evidence by ID scoped to a tenant.
	DeleteEvidence(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error
}
