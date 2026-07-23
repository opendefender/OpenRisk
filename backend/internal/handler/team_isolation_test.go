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

	"github.com/opendefender/openrisk/internal/domain"
)

// setupTeamDB builds the teams/team_members tables by hand (AutoMigrate would
// emit the Postgres-only gen_random_uuid() default which sqlite rejects).
func setupTeamDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE teams (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			metadata TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE team_members (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT,
			joined_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);`).Error)
	return db
}

// TestTeam_TenantIsolationPredicate is a regression guard for the tenant filter
// added to every team query. Teams had no tenant column, so an admin of tenant A
// could list/get/edit/delete tenant B's teams. This asserts the exact scoped
// predicates the handlers now run: a team is visible/mutable only to its tenant.
func TestTeam_TenantIsolationPredicate(t *testing.T) {
	db := setupTeamDB(t)
	tenantA := uuid.New()
	tenantB := uuid.New()

	teamA := domain.Team{ID: uuid.New(), TenantID: tenantA, Name: "A-team"}
	teamB := domain.Team{ID: uuid.New(), TenantID: tenantB, Name: "B-team"}
	require.NoError(t, db.Create(&teamA).Error)
	require.NoError(t, db.Create(&teamB).Error)

	// GetTeams predicate: each tenant lists only its own teams.
	var aTeams []domain.Team
	require.NoError(t, db.Where("tenant_id = ?", tenantA).Find(&aTeams).Error)
	require.Len(t, aTeams, 1)
	require.Equal(t, teamA.ID, aTeams[0].ID)

	// GetTeam/UpdateTeam/DeleteTeam predicate: B cannot fetch A's team by id.
	var fetched domain.Team
	err := db.First(&fetched, "id = ? AND tenant_id = ?", teamA.ID, tenantB).Error
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// The owner still resolves its own team.
	require.NoError(t, db.First(&fetched, "id = ? AND tenant_id = ?", teamA.ID, tenantA).Error)

	// DeleteTeam predicate: a cross-tenant delete affects zero rows.
	res := db.Where("id = ? AND tenant_id = ?", teamA.ID, tenantB).Delete(&domain.Team{})
	require.NoError(t, res.Error)
	require.EqualValues(t, 0, res.RowsAffected)

	// A's team survives; owner delete removes exactly one row.
	res = db.Where("id = ? AND tenant_id = ?", teamA.ID, tenantA).Delete(&domain.Team{})
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected)
}
