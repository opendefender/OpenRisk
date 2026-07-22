// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupAnalyticsDB creates an in-memory SQLite DB with just the columns the
// analytics/dashboard aggregation queries touch on the risks table.
func setupAnalyticsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE risks (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			status TEXT,
			level TEXT,
			severity TEXT,
			framework TEXT,
			score REAL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)
	return db
}

func insertAnalyticsRisk(t *testing.T, db *gorm.DB, tenant uuid.UUID, status, level string, score float64) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO risks (id, tenant_id, status, level, severity, framework, score, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		uuid.NewString(), tenant.String(), status, level, level, "ISO27001", score, time.Now(), time.Now(),
	).Error)
}

// TestAnalyticsService_GetRiskMetrics_TenantIsolation is a regression test for
// the cross-tenant data-leak fixed on the audit/product-readiness branch: the
// AnalyticsService previously aggregated risks across ALL tenants (no tenant_id
// filter), so any authenticated user saw global counts. Every metric MUST now
// be scoped to the caller's tenant (RULE #2).
func TestAnalyticsService_GetRiskMetrics_TenantIsolation(t *testing.T) {
	db := setupAnalyticsDB(t)
	svc := NewAnalyticsService(db)

	tenantA := uuid.New()
	tenantB := uuid.New()

	insertAnalyticsRisk(t, db, tenantA, "active", "high", 8)
	insertAnalyticsRisk(t, db, tenantA, "mitigated", "low", 2)
	// Tenant B has more risks; they must NEVER surface in tenant A's metrics.
	insertAnalyticsRisk(t, db, tenantB, "active", "high", 9)
	insertAnalyticsRisk(t, db, tenantB, "active", "high", 9)
	insertAnalyticsRisk(t, db, tenantB, "active", "high", 9)

	mA, err := svc.GetRiskMetrics(context.Background(), tenantA)
	require.NoError(t, err)
	require.Equal(t, int64(2), mA.TotalRisks, "tenant A must only see its own 2 risks, not tenant B's")
	require.Equal(t, int64(1), mA.ActiveRisks)
	require.Equal(t, int64(1), mA.MitigatedRisks)
	require.Equal(t, int64(1), mA.HighRisks)

	mB, err := svc.GetRiskMetrics(context.Background(), tenantB)
	require.NoError(t, err)
	require.Equal(t, int64(3), mB.TotalRisks, "tenant B sees only its own 3 risks")
	require.Equal(t, int64(3), mB.ActiveRisks)

	// An empty/unknown tenant must see nothing (fail-closed).
	mEmpty, err := svc.GetRiskMetrics(context.Background(), uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, int64(0), mEmpty.TotalRisks)
}

// TestDashboardDataService_SeverityDistribution_TenantIsolation covers the
// second leaky service (enhanced dashboard widgets).
func TestDashboardDataService_SeverityDistribution_TenantIsolation(t *testing.T) {
	db := setupAnalyticsDB(t)
	svc := NewDashboardDataService(db, nil)

	tenantA := uuid.New()
	tenantB := uuid.New()

	insertAnalyticsRisk(t, db, tenantA, "active", "high", 8)   // severity=high
	insertAnalyticsRisk(t, db, tenantB, "active", "critical", 9) // severity=critical (other tenant)
	insertAnalyticsRisk(t, db, tenantB, "active", "critical", 9)

	distA, err := svc.GetSeverityDistribution(context.Background(), tenantA)
	require.NoError(t, err)
	require.Equal(t, int64(1), distA.High)
	require.Equal(t, int64(0), distA.Critical, "tenant B's critical risks must not leak into tenant A")

	distB, err := svc.GetSeverityDistribution(context.Background(), tenantB)
	require.NoError(t, err)
	require.Equal(t, int64(2), distB.Critical)
	require.Equal(t, int64(0), distB.High)
}
