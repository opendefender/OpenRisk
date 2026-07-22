// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

func setupDepRepo(t *testing.T) *GormAssetDependencyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE asset_dependencies (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			source_asset_id TEXT NOT NULL,
			target_asset_id TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'depends_on',
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)
	return NewGormAssetDependencyRepository(db)
}

func newDep(tenant, src, tgt uuid.UUID, t domain.DependencyType) *domain.AssetDependency {
	return &domain.AssetDependency{
		ID: uuid.New(), TenantID: tenant, SourceAssetID: src, TargetAssetID: tgt, Type: t,
	}
}

func TestDepRepo_CreateAndListByTenant_Isolation(t *testing.T) {
	repo := setupDepRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	a1, a2, b1, b2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	require.NoError(t, repo.Create(ctx, newDep(tenantA, a1, a2, domain.DepRunsOn)))
	require.NoError(t, repo.Create(ctx, newDep(tenantB, b1, b2, domain.DepDependsOn)))

	// Tenant A sees only its own edge.
	listA, err := repo.ListByTenant(ctx, tenantA)
	require.NoError(t, err)
	assert.Len(t, listA, 1)
	assert.Equal(t, tenantA, listA[0].TenantID)

	listB, err := repo.ListByTenant(ctx, tenantB)
	require.NoError(t, err)
	assert.Len(t, listB, 1)
}

func TestDepRepo_Exists(t *testing.T) {
	repo := setupDepRepo(t)
	ctx := context.Background()
	tenant, src, tgt := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, repo.Create(ctx, newDep(tenant, src, tgt, domain.DepConnectsTo)))

	got, err := repo.Exists(ctx, tenant, src, tgt, domain.DepConnectsTo)
	require.NoError(t, err)
	assert.True(t, got)

	// Different type → not a duplicate.
	got, err = repo.Exists(ctx, tenant, src, tgt, domain.DepRunsOn)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestDepRepo_GetByID_CrossTenantIsNotFound(t *testing.T) {
	repo := setupDepRepo(t)
	ctx := context.Background()
	owner, other := uuid.New(), uuid.New()
	d := newDep(owner, uuid.New(), uuid.New(), domain.DepDependsOn)
	require.NoError(t, repo.Create(ctx, d))

	got, err := repo.GetByID(ctx, d.ID, other)
	require.NoError(t, err)
	assert.Nil(t, got) // foreign tenant → (nil, nil)
}

func TestDepRepo_DeleteByAsset(t *testing.T) {
	repo := setupDepRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	hub, a, b := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, repo.Create(ctx, newDep(tenant, hub, a, domain.DepDependsOn)))   // hub → a
	require.NoError(t, repo.Create(ctx, newDep(tenant, b, hub, domain.DepConnectsTo)))  // b → hub (incoming)
	require.NoError(t, repo.Create(ctx, newDep(tenant, a, b, domain.DepRunsOn)))        // unrelated to hub

	require.NoError(t, repo.DeleteByAsset(ctx, hub, tenant))

	remaining, err := repo.ListByTenant(ctx, tenant)
	require.NoError(t, err)
	assert.Len(t, remaining, 1) // only a → b survives
	assert.Equal(t, a, remaining[0].SourceAssetID)
}

func TestDepRepo_ListByAsset_BothDirections(t *testing.T) {
	repo := setupDepRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	x, y, z := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, repo.Create(ctx, newDep(tenant, x, y, domain.DepDependsOn))) // x → y
	require.NoError(t, repo.Create(ctx, newDep(tenant, z, x, domain.DepConnectsTo))) // z → x

	got, err := repo.ListByAsset(ctx, x, tenant)
	require.NoError(t, err)
	assert.Len(t, got, 2) // both the outgoing and incoming edge touch x
}
