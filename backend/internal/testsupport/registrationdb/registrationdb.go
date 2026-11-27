// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package registrationdb builds the sqlite schema account registration writes
// to (#1518): organisations, users, memberships, and the RBAC mirror
// (tenants, roles, user_tenants). Tests only.
package registrationdb

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// Open returns a guarded in-memory database holding the registration tables.
// When withAdmin is set the predefined Admin role exists, as after SeedRBAC.
func Open(t testing.TB, withAdmin bool) (*gorm.DB, *domain.RoleEnhanced) {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	for _, m := range []struct {
		table, ddl string
		model      any
	}{
		{"users", `CREATE TABLE users (id TEXT PRIMARY KEY)`, &domain.User{}},
		{"organization_members", `CREATE TABLE organization_members (id TEXT PRIMARY KEY)`, &domain.OrganizationMember{}},
		{"organizations", `CREATE TABLE organizations (id TEXT PRIMARY KEY)`, &domain.Organization{}},
		{"roles", `CREATE TABLE roles (id TEXT PRIMARY KEY)`, &domain.RoleEnhanced{}},
		{"tenants", `CREATE TABLE tenants (id TEXT PRIMARY KEY)`, &domain.Tenant{}},
		{"user_tenants", `CREATE TABLE user_tenants (user_id TEXT, tenant_id TEXT, PRIMARY KEY (user_id, tenant_id))`, &domain.UserTenant{}},
	} {
		require.NoError(t, db.Exec(m.ddl).Error)
		require.NoError(t, sqliteschema.Reconcile(db, m.table, m.model))
	}
	if !withAdmin {
		return db, nil
	}
	admin := &domain.RoleEnhanced{Name: "Admin", Level: domain.RoleLevelAdmin, IsPredefined: true, IsActive: true}
	admin.ID = uuid.New()
	require.NoError(t, db.Create(admin).Error)
	return db, admin
}
