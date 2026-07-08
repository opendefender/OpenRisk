// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupRiskScoringRepo creates an in-memory SQLite DB with just enough schema
// (risks, assets, risk_assets) to exercise GetRisksByAssetID.
func setupRiskScoringRepo(t *testing.T) (*GormRiskRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE risks (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			probability REAL NOT NULL DEFAULT 0,
			impact REAL NOT NULL DEFAULT 0,
			score REAL NOT NULL DEFAULT 0
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE assets (
			id TEXT PRIMARY KEY,
			criticality TEXT NOT NULL DEFAULT 'MEDIUM'
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE risk_assets (
			risk_id TEXT NOT NULL,
			asset_id TEXT NOT NULL
		);
	`).Error)

	return NewGormRiskRepository(db), db
}

func TestGetRisksByAssetID_ScansCriticalityWithoutError(t *testing.T) {
	// Regression test: the previous implementation scanned the assets.criticality
	// TEXT column (e.g. "HIGH") directly into a float64 field and would fail at
	// runtime with a scan type error. This must now succeed and return a
	// resolved numeric factor instead of the raw string.
	repo, db := setupRiskScoringRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()
	riskID := uuid.New()
	assetID := uuid.New()

	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, probability, impact, score) VALUES (?, ?, 0.5, 8.0, 4.0)`, riskID, tenantID).Error)
	require.NoError(t, db.Exec(`INSERT INTO assets (id, criticality) VALUES (?, 'HIGH')`, assetID).Error)
	require.NoError(t, db.Exec(`INSERT INTO risk_assets (risk_id, asset_id) VALUES (?, ?)`, riskID, assetID).Error)

	risks, err := repo.GetRisksByAssetID(ctx, assetID, tenantID)

	require.NoError(t, err)
	require.Len(t, risks, 1)
	assert.Equal(t, riskID, risks[0].ID)
	assert.Equal(t, 2.5, risks[0].AssetCriticality, "HIGH must resolve to its ScoreFactor(), not the raw string")
}

func TestGetRisksByAssetID_AveragesAcrossAllLinkedAssets(t *testing.T) {
	// A risk linked to two assets (one LOW=0.5, one CRITICAL=3.0) must use the
	// average of both factors (1.75), not just the factor of whichever asset
	// triggered the recalculation.
	repo, db := setupRiskScoringRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()
	riskID := uuid.New()
	assetLow := uuid.New()
	assetCritical := uuid.New()

	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, probability, impact, score) VALUES (?, ?, 0.5, 8.0, 4.0)`, riskID, tenantID).Error)
	require.NoError(t, db.Exec(`INSERT INTO assets (id, criticality) VALUES (?, 'LOW')`, assetLow).Error)
	require.NoError(t, db.Exec(`INSERT INTO assets (id, criticality) VALUES (?, 'CRITICAL')`, assetCritical).Error)
	require.NoError(t, db.Exec(`INSERT INTO risk_assets (risk_id, asset_id) VALUES (?, ?)`, riskID, assetLow).Error)
	require.NoError(t, db.Exec(`INSERT INTO risk_assets (risk_id, asset_id) VALUES (?, ?)`, riskID, assetCritical).Error)

	risks, err := repo.GetRisksByAssetID(ctx, assetCritical, tenantID)

	require.NoError(t, err)
	require.Len(t, risks, 1)
	assert.Equal(t, 1.75, risks[0].AssetCriticality)
}

func TestGetRisksByAssetID_NoLinkedRisks(t *testing.T) {
	repo, _ := setupRiskScoringRepo(t)

	risks, err := repo.GetRisksByAssetID(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Empty(t, risks)
}

func TestGetRisksByAssetID_ScopedToTenant(t *testing.T) {
	repo, db := setupRiskScoringRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	riskID := uuid.New()
	assetID := uuid.New()

	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, probability, impact, score) VALUES (?, ?, 0.5, 8.0, 4.0)`, riskID, tenantA).Error)
	require.NoError(t, db.Exec(`INSERT INTO assets (id, criticality) VALUES (?, 'MEDIUM')`, assetID).Error)
	require.NoError(t, db.Exec(`INSERT INTO risk_assets (risk_id, asset_id) VALUES (?, ?)`, riskID, assetID).Error)

	risks, err := repo.GetRisksByAssetID(ctx, assetID, tenantB)

	require.NoError(t, err)
	assert.Empty(t, risks, "a risk belonging to tenant A must not surface for tenant B")
}
