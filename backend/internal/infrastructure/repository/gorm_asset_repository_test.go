// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupAssetRepo creates an in-memory SQLite DB with the asset tables and
// returns a ready-to-use GormAssetRepository. `risks`/`risk_assets` are
// minimal stand-ins so Preload("Risks") doesn't error on a missing table.
func setupAssetRepo(t *testing.T) *GormAssetRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE assets (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			organization_id TEXT,
			name TEXT NOT NULL,
			type TEXT,
			criticality TEXT NOT NULL DEFAULT 'MEDIUM',
			owner TEXT,
			source TEXT NOT NULL DEFAULT 'MANUAL',
			external_id TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE risks (id TEXT PRIMARY KEY);`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE risk_assets (
			risk_id TEXT NOT NULL,
			asset_id TEXT NOT NULL
		);
	`).Error)

	require.NoError(t, db.Exec(`
		CREATE TABLE asset_snapshots (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			asset_id TEXT NOT NULL,
			name TEXT,
			type TEXT,
			criticality TEXT,
			owner TEXT,
			reason TEXT NOT NULL DEFAULT 'update',
			created_at DATETIME
		);
	`).Error)

	return NewGormAssetRepository(db)
}

func TestAssetRepository_CreateAndGetByID_Success(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()

	asset := &domain.Asset{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        "Production-DB-01",
		Type:        "Database",
		Criticality: domain.CriticalityHigh,
		Owner:       "IT Dept",
	}
	require.NoError(t, repo.Create(ctx, asset))

	got, err := repo.GetByID(ctx, asset.ID, tenantID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Production-DB-01", got.Name)
	assert.Equal(t, domain.CriticalityHigh, got.Criticality)
}

func TestAssetRepository_GetByID_NotFound(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()

	got, err := repo.GetByID(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestAssetRepository_GetByID_CrossTenantFails(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	asset := &domain.Asset{ID: uuid.New(), TenantID: tenantA, Name: "Tenant A Server"}
	require.NoError(t, repo.Create(ctx, asset))

	got, err := repo.GetByID(ctx, asset.ID, tenantB)
	require.NoError(t, err)
	assert.Nil(t, got, "asset from tenant A must not be visible to tenant B")
}

func TestAssetRepository_List_ScopedToTenant(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	require.NoError(t, repo.Create(ctx, &domain.Asset{ID: uuid.New(), TenantID: tenantA, Name: "A1"}))
	require.NoError(t, repo.Create(ctx, &domain.Asset{ID: uuid.New(), TenantID: tenantA, Name: "A2"}))
	require.NoError(t, repo.Create(ctx, &domain.Asset{ID: uuid.New(), TenantID: tenantB, Name: "B1"}))

	assetsA, err := repo.List(ctx, tenantA)
	require.NoError(t, err)
	assert.Len(t, assetsA, 2)

	assetsB, err := repo.List(ctx, tenantB)
	require.NoError(t, err)
	assert.Len(t, assetsB, 1)
}

func TestAssetRepository_Update_Success(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()

	asset := &domain.Asset{ID: uuid.New(), TenantID: tenantID, Name: "Server", Criticality: domain.CriticalityLow}
	require.NoError(t, repo.Create(ctx, asset))

	asset.Criticality = domain.CriticalityCritical
	require.NoError(t, repo.Update(ctx, asset))

	got, err := repo.GetByID(ctx, asset.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.CriticalityCritical, got.Criticality)
}

func TestAssetRepository_Update_CrossTenantFails(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	asset := &domain.Asset{ID: uuid.New(), TenantID: tenantA, Name: "Server", Criticality: domain.CriticalityLow}
	require.NoError(t, repo.Create(ctx, asset))

	// Attacker forges tenant B's ID onto tenant A's asset — must not apply.
	forged := &domain.Asset{ID: asset.ID, TenantID: tenantB, Criticality: domain.CriticalityCritical}
	err := repo.Update(ctx, forged)
	assert.Error(t, err, "update scoped to the wrong tenant must fail, not silently no-op")

	got, err := repo.GetByID(ctx, asset.ID, tenantA)
	require.NoError(t, err)
	assert.Equal(t, domain.CriticalityLow, got.Criticality, "tenant A's asset must be unmodified")
}

func TestAssetRepository_Delete_Success(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()

	asset := &domain.Asset{ID: uuid.New(), TenantID: tenantID, Name: "Server"}
	require.NoError(t, repo.Create(ctx, asset))
	require.NoError(t, repo.Delete(ctx, asset.ID, tenantID))

	got, err := repo.GetByID(ctx, asset.ID, tenantID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestAssetRepository_Delete_CrossTenantFails(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	asset := &domain.Asset{ID: uuid.New(), TenantID: tenantA, Name: "Server"}
	require.NoError(t, repo.Create(ctx, asset))

	err := repo.Delete(ctx, asset.ID, tenantB)
	assert.Error(t, err)

	got, err := repo.GetByID(ctx, asset.ID, tenantA)
	require.NoError(t, err)
	assert.NotNil(t, got, "tenant A's asset must survive tenant B's delete attempt")
}

func TestAssetRepository_SnapshotLifecycle(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()
	assetID := uuid.New()

	require.NoError(t, repo.CreateSnapshot(ctx, &domain.AssetSnapshot{
		ID: uuid.New(), TenantID: tenantID, AssetID: assetID,
		Name: "Server", Criticality: domain.CriticalityLow, Reason: "update",
	}))
	require.NoError(t, repo.CreateSnapshot(ctx, &domain.AssetSnapshot{
		ID: uuid.New(), TenantID: tenantID, AssetID: assetID,
		Name: "Server", Criticality: domain.CriticalityHigh, Reason: "update",
	}))

	history, err := repo.ListSnapshots(ctx, assetID, tenantID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	// newest first
	assert.Equal(t, domain.CriticalityHigh, history[0].Criticality)
}

func TestAssetRepository_ListSnapshots_CrossTenantEmpty(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	assetID := uuid.New()

	require.NoError(t, repo.CreateSnapshot(ctx, &domain.AssetSnapshot{
		ID: uuid.New(), TenantID: uuid.New(), AssetID: assetID, Reason: "update",
	}))

	history, err := repo.ListSnapshots(ctx, assetID, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, history)
}
