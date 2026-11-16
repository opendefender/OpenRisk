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

	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

func setupRiskReviewRepo(t *testing.T) (*GormRiskReviewRepository, *gorm.DB) {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE risks (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			review_interval_days INTEGER NOT NULL DEFAULT 0,
			next_review_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)
	// domain.Risk has an AfterSave hook that appends to risk_histories.
	require.NoError(t, db.Exec(`CREATE TABLE risk_histories (id TEXT PRIMARY KEY, risk_id TEXT, score REAL, impact INTEGER, probability INTEGER, status TEXT, changed_by TEXT, change_type TEXT, created_at DATETIME)`).Error)
	return NewGormRiskReviewRepository(db), db
}

// #808 criterion 4: BumpNextReview carries the tenant.
func TestRiskReviewRepo_BumpNextReview_CarriesTenant(t *testing.T) {
	repo, db := setupRiskReviewRepo(t)
	ctx := context.Background()
	tenantA, tenantB, id := uuid.New(), uuid.New(), uuid.New()
	due := time.Now().Add(-time.Hour).UTC()
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, review_interval_days, next_review_at) VALUES (?, ?, 30, ?)`, id.String(), tenantA.String(), due).Error)

	next := time.Now().Add(30 * 24 * time.Hour).UTC()
	require.NoError(t, repo.BumpNextReview(ctx, tenantB, id, next))
	got, err := repo.ListDueForReview(ctx, time.Now())
	require.NoError(t, err)
	assert.Len(t, got, 1, "another tenant must not push this risk's review out")

	require.NoError(t, repo.BumpNextReview(ctx, tenantA, id, next))
	got, err = repo.ListDueForReview(ctx, time.Now())
	require.NoError(t, err)
	assert.Empty(t, got)
}
