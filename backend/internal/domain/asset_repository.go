// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"context"

	"github.com/google/uuid"
)

// AssetRepository defines the port for asset data persistence.
// Infrastructure layer implements this interface.
//
// ABSOLUTE RULE: All methods MUST filter by tenant_id in the repository,
// never in the handler. If an asset belongs to another tenant → return
// (nil, nil) (not found), never 403.
type AssetRepository interface {
	// Create persists a new asset for a tenant.
	Create(ctx context.Context, asset *Asset) error

	// GetByID retrieves an asset by ID scoped to a tenant.
	// Returns (nil, nil) if not found or belongs to another tenant.
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*Asset, error)

	// List retrieves all assets for a tenant, with linked risks preloaded.
	List(ctx context.Context, tenantID uuid.UUID) ([]Asset, error)

	// Update updates an existing asset.
	// MANDATORY: tenant_id must be part of the WHERE clause.
	Update(ctx context.Context, asset *Asset) error

	// Delete soft-deletes an asset by ID scoped to a tenant.
	Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error

	// CreateSnapshot persists a historical snapshot of an asset's state.
	CreateSnapshot(ctx context.Context, snapshot *AssetSnapshot) error

	// ListSnapshots retrieves the history of an asset, newest first.
	ListSnapshots(ctx context.Context, assetID uuid.UUID, tenantID uuid.UUID) ([]AssetSnapshot, error)
}
