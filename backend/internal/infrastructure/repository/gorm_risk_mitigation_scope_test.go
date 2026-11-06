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
	"gorm.io/gorm"
)

// GHSA-2w75-9qj8-fwwp — a plan of tenant B that points at tenant A's risk (a
// row written before the create path checked the risk) must not show up in
// A's register: not in the risk detail, not in the list, and so never in the
// association save that wrote it into A's audit trail.
func setupRiskMitigationScope(t *testing.T) (*GormRiskRepository, *gorm.DB) {
	t.Helper()
	_, db := setupMitigationTxRepo(t, false)
	require.NoError(t, db.Exec(`CREATE TABLE assets (id TEXT PRIMARY KEY, name TEXT, criticality TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE risk_assets (risk_id TEXT NOT NULL, asset_id TEXT NOT NULL)`).Error)
	return &GormRiskRepository{db: db}, db
}

func plantMitigation(t *testing.T, db *gorm.DB, tenantID, riskID uuid.UUID, title string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO mitigations (id, tenant_id, risk_id, title, status, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		uuid.New().String(), tenantID.String(), riskID.String(), title, "PLANNED").Error)
}

func TestRiskGetByID_Unauthorized_ForeignMitigationNotPreloaded(t *testing.T) {
	repo, db := setupRiskMitigationScope(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	riskA := seedTxRisk(t, db, tenantA)
	plantMitigation(t, db, tenantA, riskA, "own plan")
	plantMitigation(t, db, tenantB, riskA, "planted by B")

	got, err := repo.GetByID(context.Background(), riskA, tenantA)
	require.NoError(t, err)
	require.Len(t, got.Mitigations, 1, "only tenant A's own plan rides with its risk")
	assert.Equal(t, "own plan", got.Mitigations[0].Title)
	assert.Equal(t, tenantA, got.Mitigations[0].TenantID)
}

func TestRiskList_Unauthorized_ForeignMitigationNotPreloaded(t *testing.T) {
	repo, db := setupRiskMitigationScope(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	riskA := seedTxRisk(t, db, tenantA)
	plantMitigation(t, db, tenantB, riskA, "planted by B")

	page, err := repo.List(context.Background(), tenantA, domain.RiskQuery{Page: 1, Limit: 20, SortBy: "created_at", SortOrder: "desc"})
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	assert.Empty(t, page.Data[0].Mitigations, "tenant B's plan must not appear in tenant A's list")
}
