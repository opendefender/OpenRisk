// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

func setupVendorRepo(t *testing.T) (*GormVendorRepository, *gorm.DB) {
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
			cpes TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "assets", &domain.Asset{}))

	require.NoError(t, db.Exec(`
		CREATE TABLE risks (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			title TEXT NOT NULL,
			score REAL,
			criticality TEXT,
			status TEXT,
			asset_id TEXT,
			deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE risk_assets (risk_id TEXT NOT NULL, asset_id TEXT NOT NULL);`).Error)

	return NewGormVendorRepository(db), db
}

func seedVendorRepoAsset(t *testing.T, db *gorm.DB, tenant uuid.UUID, name string, cat domain.AssetCategory) domain.Asset {
	t.Helper()
	a := domain.Asset{ID: uuid.New(), TenantID: tenant, Name: name, Type: string(cat), Category: cat, Criticality: domain.CriticalityMedium}
	require.NoError(t, db.Create(&a).Error)
	return a
}

func seedVendorRepoRisk(t *testing.T, db *gorm.DB, tenant uuid.UUID, title string, score float64, legacyAsset *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var assetID any
	if legacyAsset != nil {
		assetID = legacyAsset.String()
	}
	require.NoError(t, db.Exec(
		`INSERT INTO risks (id, tenant_id, title, score, criticality, status, asset_id) VALUES (?, ?, ?, ?, 'high', 'open', ?)`,
		id.String(), tenant.String(), title, score, assetID).Error)
	return id
}

func joinVendorRepoRisk(t *testing.T, db *gorm.DB, riskID, assetID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO risk_assets (risk_id, asset_id) VALUES (?, ?)`, riskID.String(), assetID.String()).Error)
}

func TestVendorRepo_ListVendors_ScopedToTenantAndCategory(t *testing.T) {
	repo, db := setupVendorRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	seedVendorRepoAsset(t, db, tenantA, "Zeta SaaS", domain.CategoryVendor)
	seedVendorRepoAsset(t, db, tenantA, "Acme Cloud", domain.CategoryVendor)
	seedVendorRepoAsset(t, db, tenantA, "srv-01", domain.CategoryServer)
	seedVendorRepoAsset(t, db, tenantB, "Bravo", domain.CategoryVendor)

	listA, err := repo.ListVendors(ctx, tenantA)
	require.NoError(t, err)
	require.Len(t, listA, 2, "the server is not a vendor and Bravo belongs to tenant B")
	assert.Equal(t, "Acme Cloud", listA[0].Name, "ordered by name")
	for _, v := range listA {
		assert.Equal(t, tenantA, v.TenantID)
		assert.Equal(t, domain.CategoryVendor, v.Category)
	}

	listB, err := repo.ListVendors(ctx, tenantB)
	require.NoError(t, err)
	require.Len(t, listB, 1)
	assert.Equal(t, "Bravo", listB[0].Name)
}

func TestVendorRepo_RefusesTheNilTenant(t *testing.T) {
	repo, _ := setupVendorRepo(t)
	ctx := context.Background()

	_, err := repo.ListVendors(ctx, uuid.Nil)
	assert.Error(t, err)
	_, err = repo.GetVendor(ctx, uuid.New(), uuid.Nil)
	assert.Error(t, err)
	_, err = repo.AssetsByIDs(ctx, uuid.Nil, []uuid.UUID{uuid.New()})
	assert.Error(t, err)
	_, err = repo.RisksByAssetIDs(ctx, uuid.Nil, []uuid.UUID{uuid.New()})
	assert.Error(t, err)
}

func TestVendorRepo_GetVendor_RefusesForeignAndNonVendorRows(t *testing.T) {
	repo, db := setupVendorRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	vendor := seedVendorRepoAsset(t, db, tenantA, "Acme", domain.CategoryVendor)
	server := seedVendorRepoAsset(t, db, tenantA, "srv-01", domain.CategoryServer)

	got, err := repo.GetVendor(ctx, vendor.ID, tenantA)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, vendor.ID, got.ID)

	foreign, err := repo.GetVendor(ctx, vendor.ID, tenantB)
	require.NoError(t, err)
	assert.Nil(t, foreign, "another tenant's vendor reads back as absent")

	notAVendor, err := repo.GetVendor(ctx, server.ID, tenantA)
	require.NoError(t, err)
	assert.Nil(t, notAVendor, "a server is not a vendor (ADR 0001 D5b)")
}

func TestVendorRepo_AssetsByIDs_OmitsForeignIDs(t *testing.T) {
	repo, db := setupVendorRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	own := seedVendorRepoAsset(t, db, tenantA, "srv-a", domain.CategoryServer)
	foreign := seedVendorRepoAsset(t, db, tenantB, "srv-b", domain.CategoryServer)

	got, err := repo.AssetsByIDs(ctx, tenantA, []uuid.UUID{own.ID, foreign.ID, uuid.New()})

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, own.ID, got[0].ID)

	empty, err := repo.AssetsByIDs(ctx, tenantA, nil)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

// risk_assets carries no tenant_id. The gate is the parent risk's tenant, on
// both linkage mechanisms.
func TestVendorRepo_RisksByAssetIDs_GatedThroughTheParentRisk(t *testing.T) {
	repo, db := setupVendorRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	srv := seedVendorRepoAsset(t, db, tenantA, "srv-a", domain.CategoryServer)

	// Linked through risk_assets.
	ownJoined := seedVendorRepoRisk(t, db, tenantA, "own, joined", 4.0, nil)
	joinVendorRepoRisk(t, db, ownJoined, srv.ID)
	// Linked through the legacy column, and ALSO joined: must appear once.
	ownBoth := seedVendorRepoRisk(t, db, tenantA, "own, legacy and joined", 8.0, &srv.ID)
	joinVendorRepoRisk(t, db, ownBoth, srv.ID)

	// Another tenant's risks pointed at our asset, through both mechanisms.
	foreignJoined := seedVendorRepoRisk(t, db, tenantB, "foreign, joined", 9.9, nil)
	joinVendorRepoRisk(t, db, foreignJoined, srv.ID)
	seedVendorRepoRisk(t, db, tenantB, "foreign, legacy", 9.9, &srv.ID)

	// A soft-deleted risk of our own.
	deleted := seedVendorRepoRisk(t, db, tenantA, "deleted", 9.9, nil)
	joinVendorRepoRisk(t, db, deleted, srv.ID)
	require.NoError(t, db.Exec(`UPDATE risks SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, deleted.String()).Error)

	got, err := repo.RisksByAssetIDs(ctx, tenantA, []uuid.UUID{srv.ID})

	require.NoError(t, err)
	risks := got[srv.ID]
	require.Len(t, risks, 2)
	assert.Equal(t, ownBoth, risks[0].ID, "ordered by score, highest first")
	assert.Equal(t, ownJoined, risks[1].ID)
	for _, r := range risks {
		assert.NotContains(t, r.Title, "foreign")
		assert.NotEqual(t, "deleted", r.Title)
	}

	// Tenant B asking about tenant A's asset gets tenant B's own risks only —
	// and never tenant A's.
	fromB, err := repo.RisksByAssetIDs(ctx, tenantB, []uuid.UUID{srv.ID})
	require.NoError(t, err)
	for _, r := range fromB[srv.ID] {
		assert.Contains(t, r.Title, "foreign")
	}
}
