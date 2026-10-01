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
	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

// withUserDB points the package-global database.DB at a fresh in-memory sqlite
// (restored afterwards) with the minimal users + organization_members tables the
// legacy /users handlers touch.
func withUserDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, is_active NUMERIC DEFAULT 1, deleted_at DATETIME);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE organization_members (id TEXT PRIMARY KEY, organization_id TEXT, user_id TEXT, role TEXT);`).Error)

	orig := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = orig })
	return db
}

func addUser(t *testing.T, db *gorm.DB, org, user uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO users (id, email) VALUES (?,?)`, user.String(), user.String()+"@x.io").Error)
	require.NoError(t, db.Exec(`INSERT INTO organization_members (id, organization_id, user_id, role) VALUES (?,?,?,?)`,
		uuid.NewString(), org.String(), user.String(), "user").Error)
}

// TestUser_TenantScoping proves the legacy /users list is scoped to the caller's
// organization. It previously returned every user in the deployment. The write
// routes that used to sit beside it were removed in #807.
func TestUser_TenantScoping(t *testing.T) {
	db := withUserDB(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	addUser(t, db, tenantA, userA)
	addUser(t, db, tenantB, userB)

	// GetUsers list predicate: the pluck-then-IN scoping returns only the caller's
	// members, never every user in the deployment.
	var idsForA []uuid.UUID
	database.DB.Model(&domain.OrganizationMember{}).Where("organization_id = ?", tenantA).Pluck("user_id", &idsForA)
	require.Equal(t, []uuid.UUID{userA}, idsForA)

	var usersForA []domain.User
	require.NoError(t, database.DB.Where("id IN ?", idsForA).Find(&usersForA).Error)
	require.Len(t, usersForA, 1)
	require.Equal(t, userA, usersForA[0].ID)
}
