// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DependencyType classifies HOW one asset depends on another. It is a small,
// documented vocabulary (kept flexible on the wire) that powers the asset
// dependency map ("cartographie des dépendances"): an edge Source --Type--> Target
// reads "Source <type> Target", e.g. "web-app RUNS_ON srv-01".
type DependencyType string

const (
	// DepDependsOn is the generic fallback ("A depends on B").
	DepDependsOn DependencyType = "depends_on"
	// DepRunsOn — an application/service runs on a host (app → server).
	DepRunsOn DependencyType = "runs_on"
	// DepConnectsTo — a network/API connection (A calls/reaches B).
	DepConnectsTo DependencyType = "connects_to"
	// DepHostedBy — a workload is hosted by a cloud/platform asset.
	DepHostedBy DependencyType = "hosted_by"
	// DepStoresDataIn — an app persists data in a database/storage asset.
	DepStoresDataIn DependencyType = "stores_data_in"
	// DepAuthenticatesVia — a service authenticates through an IdP/directory.
	DepAuthenticatesVia DependencyType = "authenticates_via"
	// DepBacksUpTo — an asset is backed up to another (target).
	DepBacksUpTo DependencyType = "backs_up_to"
	// DepManagedBy — an asset is operated/managed by a supplier/user.
	DepManagedBy DependencyType = "managed_by"
)

// IsValid reports whether t is one of the known dependency types. An empty
// type is treated as invalid by the use case (it defaults to DepDependsOn
// before persistence).
func (t DependencyType) IsValid() bool {
	switch t {
	case DepDependsOn, DepRunsOn, DepConnectsTo, DepHostedBy,
		DepStoresDataIn, DepAuthenticatesVia, DepBacksUpTo, DepManagedBy:
		return true
	default:
		return false
	}
}

// AssetDependency is a directed edge in a tenant's asset dependency graph:
// SourceAsset depends on TargetAsset. It is the persistence backbone of the
// dependency cartography — the front-end universe/topology view is built by
// listing every dependency for the tenant and drawing Source → Target.
//
// ABSOLUTE RULE: every query filters by tenant_id; both endpoints must belong
// to the same tenant (enforced in the use case, never trust the client).
type AssetDependency struct {
	ID            uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"tenant_id"`
	SourceAssetID uuid.UUID      `gorm:"type:uuid;not null;index" json:"source_asset_id"`
	TargetAssetID uuid.UUID      `gorm:"type:uuid;not null;index" json:"target_asset_id"`
	Type          DependencyType `gorm:"size:32;not null;default:'depends_on'" json:"type"`
	Description   string         `json:"description"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (AssetDependency) TableName() string {
	return "asset_dependencies"
}

// AssetDependencyRepository is the port for persisting the asset dependency
// graph. Infrastructure implements it.
//
// ABSOLUTE RULE: All methods MUST filter by tenant_id in the repository. If a
// dependency belongs to another tenant → return (nil, nil) / not found, never 403.
type AssetDependencyRepository interface {
	// Create persists a new directed dependency edge for a tenant.
	Create(ctx context.Context, dep *AssetDependency) error

	// GetByID retrieves a dependency by ID scoped to a tenant.
	// Returns (nil, nil) if not found or owned by another tenant.
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*AssetDependency, error)

	// ListByTenant returns every dependency for a tenant (the full graph).
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]AssetDependency, error)

	// ListByAsset returns every dependency touching an asset in EITHER
	// direction (source or target), scoped to a tenant.
	ListByAsset(ctx context.Context, assetID uuid.UUID, tenantID uuid.UUID) ([]AssetDependency, error)

	// Exists reports whether an identical edge (same source, target, type)
	// already exists for the tenant — used to reject duplicates.
	Exists(ctx context.Context, tenantID, sourceAssetID, targetAssetID uuid.UUID, depType DependencyType) (bool, error)

	// Delete soft-deletes a dependency by ID scoped to a tenant.
	Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error

	// DeleteByAsset removes every dependency touching an asset (either
	// direction). Called when an asset is deleted so no dangling edges remain.
	DeleteByAsset(ctx context.Context, assetID uuid.UUID, tenantID uuid.UUID) error
}
