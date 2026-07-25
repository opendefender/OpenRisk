// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// setupTimelineDB builds a minimal risks + risk_histories schema (just the
// columns the timeline queries touch), matching the analytics test pattern.
func setupTimelineDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE risks (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			deleted_at DATETIME
		);`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE risk_histories (
			id TEXT PRIMARY KEY,
			risk_id TEXT NOT NULL,
			score REAL DEFAULT 0,
			impact INTEGER DEFAULT 0,
			probability INTEGER DEFAULT 0,
			status TEXT,
			changed_by TEXT,
			change_type TEXT,
			created_at DATETIME
		);`).Error)
	return db
}

func insertRisk(t *testing.T, db *gorm.DB, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id) VALUES (?,?)`, id.String(), tenant.String()).Error)
	return id
}

func insertHistory(t *testing.T, db *gorm.DB, riskID uuid.UUID, changeType string, score float64) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO risk_histories (id, risk_id, score, status, change_type, created_at) VALUES (?,?,?,?,?,?)`,
		uuid.NewString(), riskID.String(), score, "open", changeType, time.Now(),
	).Error)
}

// TestRiskTimeline_CrossTenant proves a tenant can never read another tenant's
// risk history. The handler used to read history by risk UUID with no tenant
// check at all, so any authenticated user could pull scores/statuses/actors of
// any risk in the system (RULE #2 — history is gated by the parent risk's tenant).
func TestRiskTimeline_CrossTenant(t *testing.T) {
	db := setupTimelineDB(t)
	svc := &RiskTimelineService{db: db}

	tenantA := uuid.New()
	tenantB := uuid.New()
	riskA := insertRisk(t, db, tenantA)
	insertHistory(t, db, riskA, "CREATE", 5)
	insertHistory(t, db, riskA, "SCORE_CHANGE", 12)
	insertHistory(t, db, riskA, "STATUS_CHANGE", 12)

	// Owner (tenant A) sees the history.
	hist, err := svc.GetRiskTimeline(tenantA, riskA)
	require.NoError(t, err)
	require.Len(t, hist, 3)

	// Tenant B is denied on every per-risk accessor.
	_, err = svc.GetRiskTimeline(tenantB, riskA)
	require.ErrorIs(t, err, domain.ErrNotFound)

	_, _, err = svc.GetRiskTimelineWithPagination(tenantB, riskA, 50, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)

	_, err = svc.GetStatusChanges(tenantB, riskA)
	require.ErrorIs(t, err, domain.ErrNotFound)

	_, err = svc.GetScoreChanges(tenantB, riskA)
	require.ErrorIs(t, err, domain.ErrNotFound)

	_, err = svc.GetRiskChangesByType(tenantB, riskA, "SCORE_CHANGE")
	require.ErrorIs(t, err, domain.ErrNotFound)

	// An unknown / nil tenant is also denied (fail closed).
	_, err = svc.GetRiskTimeline(uuid.Nil, riskA)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

// TestRiskTimeline_RecentChanges_ScopedToTenant proves /timeline/recent returns
// only the caller's activity — it previously returned the newest changes across
// ALL tenants.
func TestRiskTimeline_RecentChanges_ScopedToTenant(t *testing.T) {
	db := setupTimelineDB(t)
	svc := &RiskTimelineService{db: db}

	tenantA := uuid.New()
	tenantB := uuid.New()
	riskA := insertRisk(t, db, tenantA)
	riskB := insertRisk(t, db, tenantB)
	insertHistory(t, db, riskA, "CREATE", 1)
	insertHistory(t, db, riskA, "SCORE_CHANGE", 2)
	insertHistory(t, db, riskB, "CREATE", 9)

	aChanges, err := svc.GetRecentChanges(tenantA, 100)
	require.NoError(t, err)
	require.Len(t, aChanges, 2, "tenant A only sees its own two changes")
	for _, ch := range aChanges {
		require.Equal(t, riskA, ch.RiskID)
	}

	bChanges, err := svc.GetRecentChanges(tenantB, 100)
	require.NoError(t, err)
	require.Len(t, bChanges, 1)
	require.Equal(t, riskB, bChanges[0].RiskID)
}
