// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// TestRiskRepo_ForeignCategoryIsNotPreloaded: a risk that already holds another
// tenant's category_id (stored before the write-side check existed) reads back
// with Category == nil, on both GetByID and List (#1319).
func TestRiskRepo_ForeignCategoryIsNotPreloaded(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE risk_categories (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT, slug TEXT, description TEXT, color TEXT, sort_order INTEGER, active BOOLEAN, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE risks (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, title TEXT, category_id TEXT, probability REAL, impact REAL, score REAL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mitigations (id TEXT PRIMARY KEY, tenant_id TEXT, risk_id TEXT, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE assets (id TEXT PRIMARY KEY, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE risk_assets (risk_id TEXT, asset_id TEXT)`).Error)

	a, b := uuid.New(), uuid.New()
	own, foreign := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO risk_categories (id, tenant_id, name) VALUES (?,?,?)`, own.String(), a.String(), "own").Error)
	require.NoError(t, db.Exec(`INSERT INTO risk_categories (id, tenant_id, name) VALUES (?,?,?)`, foreign.String(), b.String(), "SECRET-OF-B").Error)
	riskOwn, riskForeign := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, title, category_id) VALUES (?,?,?,?)`, riskOwn.String(), a.String(), "o", own.String()).Error)
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, title, category_id) VALUES (?,?,?,?)`, riskForeign.String(), a.String(), "f", foreign.String()).Error)

	repo := NewGormRiskRepository(db)
	ctx := context.Background()

	// Success: the tenant's own category is still loaded.
	got, err := repo.GetByID(ctx, riskOwn, a)
	require.NoError(t, err)
	require.NotNil(t, got.Category)
	require.Equal(t, "own", got.Category.Name)

	// CrossTenant: a foreign category id is not resolved.
	got, err = repo.GetByID(ctx, riskForeign, a)
	require.NoError(t, err)
	require.Nil(t, got.Category, "another tenant's category rode along with the risk")

	page, err := repo.List(ctx, a, domain.RiskQuery{Page: 1, Limit: 50})
	require.NoError(t, err)
	for _, r := range page.Data {
		if r.ID == riskForeign {
			require.Nil(t, r.Category, "List preloaded another tenant's category")
		}
	}

	// NotFound / Unauthorized: another tenant's risk and a nil tenant read nothing.
	got, err = repo.GetByID(ctx, riskOwn, b)
	require.NoError(t, err)
	require.Nil(t, got)
	got, err = repo.GetByID(ctx, riskOwn, uuid.Nil)
	require.NoError(t, err)
	require.Nil(t, got)
}
