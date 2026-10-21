// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

// #1104 — clearing a plan's owner, assignee or reviewer must reach the row.
// GORM's Updates(struct) skips nil fields, so a slot set to nil was silently
// kept.

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

type ownershipRow struct {
	OwnerID    *string
	AssigneeID *string
	ReviewerID *string
	AssignedTo string
	Title      string
}

func readOwnership(t *testing.T, db *gorm.DB, id uuid.UUID) ownershipRow {
	t.Helper()
	var row ownershipRow
	require.NoError(t, db.Raw(`SELECT owner_id, assignee_id, reviewer_id, assigned_to, title FROM mitigations WHERE id = ?`, id.String()).Scan(&row).Error)
	return row
}

func seedOwnedPlan(t *testing.T, repo *GormMitigationRepository, tenant uuid.UUID) (*domain.Mitigation, uuid.UUID) {
	t.Helper()
	alice, bob, carol := uuid.New(), uuid.New(), uuid.New()
	m := &domain.Mitigation{
		ID: uuid.New(), TenantID: tenant, RiskID: uuid.New(), Title: "Patch VPN",
		Status: domain.MitigationInProgress, AssignedTo: domain.UUIDArray{bob},
		Ownership: domain.Ownership{OwnerID: &alice, AssigneeID: &bob, ReviewerID: &carol},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.db.Create(m).Error)
	return m, bob
}

func TestGormMitigationRepository_UpdateClearsOwnership_Success(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenant := uuid.New()
	m, _ := seedOwnedPlan(t, repo, tenant)

	m.AssigneeID = nil
	m.AssignedTo = domain.UUIDArray{}
	require.NoError(t, repo.Update(tenant.String(), m))
	row := readOwnership(t, db, m.ID)
	require.Nil(t, row.AssigneeID, "a cleared assignee is written")
	require.Equal(t, "[]", row.AssignedTo, "and the legacy array with it")
	require.NotNil(t, row.OwnerID, "the other slots are untouched")
	require.NotNil(t, row.ReviewerID)

	m.OwnerID, m.ReviewerID = nil, nil
	require.NoError(t, repo.Update(tenant.String(), m))
	row = readOwnership(t, db, m.ID)
	require.Nil(t, row.OwnerID)
	require.Nil(t, row.ReviewerID)
	require.Equal(t, "Patch VPN", row.Title)
}

// The row is not in this tenant: not found, and nothing written.
func TestGormMitigationRepository_UpdateClearsOwnership_NotFound(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenant := uuid.New()
	m, bob := seedOwnedPlan(t, repo, tenant)

	m.AssigneeID = nil
	require.ErrorIs(t, repo.Update(uuid.New().String(), m), domain.ErrNotFound)
	row := readOwnership(t, db, m.ID)
	require.NotNil(t, row.AssigneeID)
	require.Equal(t, bob.String(), *row.AssigneeID, "another tenant cannot clear it")
}

func TestGormMitigationRepository_UpdateClearsOwnership_Unauthorized(t *testing.T) {
	repo, _ := setupMitigationTxRepo(t, false)
	m, _ := seedOwnedPlan(t, repo, uuid.New())
	m.AssigneeID = nil
	require.Error(t, repo.Update("not-a-tenant", m))
}

// The same on Postgres, on the table AutoMigrate builds in production.
func TestGormMitigationRepository_UpdateClearsOwnership_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "mit_1104_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&domain.Mitigation{}))

	repo := &GormMitigationRepository{db: db}
	tenant := uuid.New()
	m, _ := seedOwnedPlan(t, repo, tenant)

	m.AssigneeID, m.OwnerID = nil, nil
	m.AssignedTo = domain.UUIDArray{}
	require.NoError(t, repo.Update(tenant.String(), m))
	got, err := repo.GetByID(tenant.String(), m.ID)
	require.NoError(t, err)
	require.Nil(t, got.AssigneeID)
	require.Nil(t, got.OwnerID)
	require.NotNil(t, got.ReviewerID, "untouched")
	require.Empty(t, got.AssignedTo)

	m.ReviewerID = nil
	require.ErrorIs(t, repo.Update(uuid.New().String(), m), domain.ErrNotFound)
	got, err = repo.GetByID(tenant.String(), m.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ReviewerID, "another tenant cannot clear it")
}
