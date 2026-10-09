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

func setupSnapshotRepo(t *testing.T) *GormScoreSnapshotRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE tenant_score_snapshots (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "tenant_score_snapshots", &domain.TenantScoreSnapshot{}))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX ux_tenant_score_snapshot_day ON tenant_score_snapshots (tenant_id, day)`).Error)
	return NewGormScoreSnapshotRepository(db)
}

func day(s string) time.Time {
	d, _ := time.Parse("2006-01-02", s)
	return d
}

// The same day written twice keeps one row with the later value.
func TestScoreSnapshot_Upsert_Success(t *testing.T) {
	repo := setupSnapshotRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	require.NoError(t, repo.Upsert(ctx, &domain.TenantScoreSnapshot{TenantID: tenant, Day: day("2026-10-08").Add(9 * time.Hour), Value: 40, Band: "medium", FormulaVersion: "2.2"}))
	require.NoError(t, repo.Upsert(ctx, &domain.TenantScoreSnapshot{TenantID: tenant, Day: day("2026-10-08").Add(18 * time.Hour), Value: 37, Band: "medium", FormulaVersion: "2.2"}))
	require.NoError(t, repo.Upsert(ctx, &domain.TenantScoreSnapshot{TenantID: tenant, Day: day("2026-10-09"), Value: 35, Band: "medium", FormulaVersion: "2.2"}))

	rows, err := repo.ListSince(ctx, tenant, day("2026-10-01"))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, 37.0, rows[0].Value)
	assert.Equal(t, 35.0, rows[1].Value)
}

func TestScoreSnapshot_ListSince_NotFound(t *testing.T) {
	repo := setupSnapshotRepo(t)
	rows, err := repo.ListSince(context.Background(), uuid.New(), day("2026-01-01"))
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// One tenant's history never contains another tenant's rows, and a write
// without a tenant is refused.
func TestScoreSnapshot_Unauthorized(t *testing.T) {
	repo := setupSnapshotRepo(t)
	ctx := context.Background()
	mine, theirs := uuid.New(), uuid.New()
	require.NoError(t, repo.Upsert(ctx, &domain.TenantScoreSnapshot{TenantID: theirs, Day: day("2026-10-08"), Value: 90, Band: "critical", FormulaVersion: "2.2"}))
	require.NoError(t, repo.Upsert(ctx, &domain.TenantScoreSnapshot{TenantID: mine, Day: day("2026-10-08"), Value: 20, Band: "low", FormulaVersion: "2.2"}))

	rows, err := repo.ListSince(ctx, mine, day("2026-01-01"))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, mine, rows[0].TenantID)

	err = repo.Upsert(ctx, &domain.TenantScoreSnapshot{Day: day("2026-10-08"), Value: 1, Band: "low"})
	assert.Error(t, err)
}
