// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package migrations

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// Printed by running the original GenerateBackupCodes from ff6961ba^.
func TestLegacyUniversalBackupCodes_MatchTheOldGenerator(t *testing.T) {
	require.Equal(t, []string{
		"4F2LIBGHU5SD", "AZ67MVK3YRWX", "ENCTQJOP4F2L", "IBGHU5SDAZ67",
		"MVK3YRWXENCT", "QJOP4F2LIBGH", "U5SDAZ67MVK3", "YRWXENCTQJOP",
	}, legacyUniversalBackupCodes())
}

func TestDatabaseURL_PrefersDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://a:b@h:1/d")
	t.Setenv("MIGRATIONS_DIR", "/app/migrations")
	require.Equal(t, "postgres://a:b@h:1/d", databaseURL())
}

func TestDatabaseURL_SkipsWithoutMigrationsDir(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MIGRATIONS_DIR", "")
	t.Setenv("DB_PASSWORD", "x")
	require.Empty(t, databaseURL(), "a dev run with neither variable keeps skipping the SQL layer")
}

// #1001: a hand-picked compose password with URL syntax in it must reach the
// server intact, not break the boot.
func TestDatabaseURL_BuildsFromDBEnvAndEncodesThePassword(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MIGRATIONS_DIR", "/app/migrations")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "openrisk")
	t.Setenv("DB_PASSWORD", "p@ss:w/rd?#% x")
	t.Setenv("DB_NAME", "openrisk")

	u, err := url.Parse(databaseURL())
	require.NoError(t, err)
	pw, _ := u.User.Password()
	require.Equal(t, "p@ss:w/rd?#% x", pw)
	require.Equal(t, "openrisk", u.User.Username())
	require.Equal(t, "db:5432", u.Host)
	require.Equal(t, "/openrisk", u.Path)
	require.Equal(t, "disable", u.Query().Get("sslmode"))
}

func TestDatabaseURL_DefaultsMatchDatabaseConnect(t *testing.T) {
	for _, k := range []string{"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
		t.Setenv(k, "")
	}
	t.Setenv("MIGRATIONS_DIR", "migrations")
	require.Equal(t, "postgres://openrisk:@localhost:5434/openrisk?sslmode=disable", databaseURL())
}
