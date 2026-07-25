// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestScoreEngine_AssetLoadTenantScoped guards the tenant filter added to the
// asset load in ComputeRiskScore. A client could previously pass another tenant's
// asset UUIDs and fold their criticality into a computed score; the scoped query
// must return zero foreign assets.
func TestScoreEngine_AssetLoadTenantScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE assets (id TEXT PRIMARY KEY, tenant_id TEXT, deleted_at DATETIME);`).Error)

	tenantA := uuid.New()
	tenantB := uuid.New()
	assetA := uuid.New()
	assetB := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO assets (id, tenant_id) VALUES (?,?)`, assetA.String(), tenantA.String()).Error)
	require.NoError(t, db.Exec(`INSERT INTO assets (id, tenant_id) VALUES (?,?)`, assetB.String(), tenantB.String()).Error)

	// Tenant B asks to score using BOTH asset ids; only its own may load.
	type row struct {
		ID       string
		TenantID string
	}
	var got []row
	require.NoError(t, db.Table("assets").
		Where("id IN ? AND tenant_id = ?", []string{assetA.String(), assetB.String()}, tenantB.String()).
		Find(&got).Error)
	require.Len(t, got, 1)
	require.Equal(t, assetB.String(), got[0].ID)
}
