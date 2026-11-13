// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #808: the bulk update writes a risk through its tenant. The write never
// reaches another tenant's row, and a risk carrying no tenant is refused.
func TestBulkUpdateRiskFromData_WritesThroughTheTenant(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE risks (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "risks", &domain.Risk{}))
	require.NoError(t, db.Exec(`CREATE TABLE risk_histories (id TEXT PRIMARY KEY, risk_id TEXT, score REAL, impact INTEGER, probability INTEGER, status TEXT, changed_by TEXT, change_type TEXT, created_at DATETIME)`).Error)

	tenant := uuid.New()
	risk := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "r", Owner: "before", Probability: 3, Impact: 3}
	require.NoError(t, db.Create(risk).Error)
	svc := &BulkOperationService{db: db}

	require.NoError(t, svc.updateRiskFromData(risk, map[string]interface{}{"owner": "after"}))
	var got domain.Risk
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", tenant, risk.ID).First(&got).Error)
	require.Equal(t, "after", got.Owner)

	// A risk of tenant A handed in with tenant B's id never lands on A's row.
	foreign := &domain.Risk{ID: risk.ID, TenantID: uuid.New(), Title: "x", Owner: "intruder"}
	require.Error(t, svc.updateRiskFromData(foreign, map[string]interface{}{"owner": "intruder"}))
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", tenant, risk.ID).First(&got).Error)
	require.Equal(t, "after", got.Owner)

	require.Error(t, svc.updateRiskFromData(&domain.Risk{ID: risk.ID}, map[string]interface{}{}), "a risk with no tenant is refused")
}
