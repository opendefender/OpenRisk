// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

func setupInventoryRepo(t *testing.T) *GormHostInventoryRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE host_inventories (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "host_inventories", &domain.HostInventory{}))
	require.NoError(t, db.Exec(`CREATE TABLE host_packages (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "host_packages", &domain.HostPackage{}))
	return NewGormHostInventoryRepository(db)
}

func inv(tenant uuid.UUID, host string) *domain.HostInventory {
	return &domain.HostInventory{
		TenantID: tenant, HostKey: domain.HostKey(host), Hostname: host,
		Platform: "ubuntu", OSVersion: "22.04.3 LTS", Ecosystem: "Ubuntu:22.04:LTS",
		CollectedAt: time.Now(),
	}
}

func TestHostInventoryRepo_ReplaceKeepsOneSnapshotPerHost(t *testing.T) {
	repo := setupInventoryRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	first := inv(tenant, "srv-core-01")
	require.NoError(t, repo.Replace(ctx, first, []domain.HostPackageInput{
		{Name: "openssl", Version: "3.0.2-0ubuntu1.10", Source: "deb"},
		{Name: "bash", Version: "5.1-6ubuntu1", Source: "deb"},
	}))
	matchedAt := time.Now()
	require.NoError(t, repo.MarkMatched(ctx, tenant, first.ID, matchedAt, "", 0))

	// The same host, reported again by its FQDN: one row, new packages.
	second := inv(tenant, "SRV-CORE-01.corp.local")
	require.NoError(t, repo.Replace(ctx, second, []domain.HostPackageInput{
		{Name: "openssl", Version: "3.0.2-0ubuntu1.15", Source: "deb"},
	}))
	assert.Equal(t, first.ID, second.ID, "same host, same row")

	all, err := repo.List(ctx, tenant)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.NotNil(t, all[0].MatchedAt, "the last match survives a new snapshot")

	pkgs, err := repo.Packages(ctx, tenant, first.ID)
	require.NoError(t, err)
	require.Len(t, pkgs, 1, "the previous snapshot's packages are gone")
	assert.Equal(t, "3.0.2-0ubuntu1.15", pkgs[0].Version)
}

func TestHostInventoryRepo_NotFound(t *testing.T) {
	repo := setupInventoryRepo(t)
	got, err := repo.Get(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

// Two tenants with the same hostname never share an inventory, packages, or
// a match state.
func TestHostInventoryRepo_TenantIsolation(t *testing.T) {
	repo := setupInventoryRepo(t)
	ctx := context.Background()
	mine, theirs := uuid.New(), uuid.New()
	asset := uuid.New()

	a := inv(mine, "srv-core-01")
	a.AssetID = &asset
	require.NoError(t, repo.Replace(ctx, a, []domain.HostPackageInput{{Name: "openssl", Version: "1", Source: "deb"}}))
	b := inv(theirs, "srv-core-01")
	require.NoError(t, repo.Replace(ctx, b, []domain.HostPackageInput{{Name: "secret-tool", Version: "9", Source: "deb"}}))
	assert.NotEqual(t, a.ID, b.ID)

	got, err := repo.Get(ctx, mine, b.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "their inventory by id, asked as me")

	pkgs, err := repo.Packages(ctx, mine, b.ID)
	require.NoError(t, err)
	assert.Empty(t, pkgs, "their packages, asked as me")

	byAsset, err := repo.ListByAsset(ctx, theirs, asset)
	require.NoError(t, err)
	assert.Empty(t, byAsset, "my asset, asked as them")

	require.NoError(t, repo.MarkMatched(ctx, mine, b.ID, time.Now(), "source_unreachable", 3))
	theirsNow, err := repo.Get(ctx, theirs, b.ID)
	require.NoError(t, err)
	assert.Empty(t, theirsNow.MatchError, "I cannot write their match state")

	tenants, err := repo.TenantsWithInventories(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{mine, theirs}, tenants)
}
