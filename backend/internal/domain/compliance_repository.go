// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
	// Frameworks (global — no tenant_id filtering)
	// =========================================================================

	// CreateFramework persists a new compliance framework.
	CreateFramework(ctx context.Context, framework *ComplianceFramework) error

	// GetFrameworkByID retrieves a framework by ID.
	// Returns (nil, nil) if not found.
	GetFrameworkByID(ctx context.Context, id uuid.UUID) (*ComplianceFramework, error)

	// ListFrameworks returns all active (non-deleted) frameworks.
	ListFrameworks(ctx context.Context) ([]ComplianceFramework, error)

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

	// DeleteEvidence soft-deletes an evidence by ID scoped to a tenant.
	DeleteEvidence(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error
}
