// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Snapshot is what the loader reads for one control, from one consistent
// read-only view: the computation's inputs plus the two display fields.
type Snapshot struct {
	Inputs         Inputs
	DeclaredStatus canonical.DeclaredStatus
	// OpenRiskCount is the number of live, open risks the control mitigates
	// (mitigated_by edges). Display only: it does not enter Compute.
	OpenRiskCount int
}

// Loader reads the inputs of one control of one tenant. Every query it runs
// carries the tenant. A control that is not live in that tenant, another tenant's
// included, is domain.ErrNotFound (code control_status.not_found); the loader
// never says which.
type Loader interface {
	Load(ctx context.Context, tenantID, controlID uuid.UUID, now time.Time) (*Snapshot, error)
}

// ControlRef names a control together with its own tenant, as the worker's
// cross-tenant claim returns it.
type ControlRef struct {
	TenantID  uuid.UUID
	ControlID uuid.UUID
}

// Store is the evaluation cache. It is written only by the engine: no route, no
// DTO. Save is guarded by evaluatedAt, so an older computation never overwrites a
// newer one; it never clears the stale flag a later write set.
type Store interface {
	// Claim returns up to limit controls that need computing, across tenants (the
	// one cross-tenant read, declared as such): rows marked stale, rows whose
	// next_change_at has passed, and live controls that have no row yet. It clears
	// stale on the rows it returns and leases them, so a crash before Save is
	// retried after the lease and a second replica does not take the same rows.
	Claim(ctx context.Context, now time.Time, limit int) ([]ControlRef, error)
	// Save upserts the evaluation of one control in its tenant.
	Save(ctx context.Context, tenantID, controlID uuid.UUID, ev Evaluation, evaluatedAt time.Time) error
	// Delete removes the row of a control that is gone.
	Delete(ctx context.Context, tenantID, controlID uuid.UUID) error
}

// Clock returns the instant of a computation. It is injected so a test pins it.
type Clock func() time.Time
