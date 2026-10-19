// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package database

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// #1001: no password may keep the backend from starting. The connection
// string goes through pgx's own parser, the one GORM's postgres driver uses.
func TestKeywordDSN_AnyPasswordReachesTheServerIntact(t *testing.T) {
	for _, pw := range []string{
		"",
		"7f3a9c",
		"with space",
		"it's",
		`back\slash`,
		`\'`,
		"p@ss:w/rd?#%&=+",
		"$HOME ${X} `cmd`",
		`"double"`,
		"tab\tnew\nline",
		"ünïcødé-密码",
		"host=evil port=1",
	} {
		cfg, err := pgconn.ParseConfig(keywordDSN("db", "openrisk", pw, "openrisk", "5432"))
		require.NoError(t, err, "password %q", pw)
		require.Equal(t, pw, cfg.Password, "password %q", pw)
		require.Equal(t, "db", cfg.Host, "password %q must not override the host", pw)
		require.Equal(t, uint16(5432), cfg.Port)
		require.Equal(t, "openrisk", cfg.User)
		require.Equal(t, "openrisk", cfg.Database)
		require.Equal(t, "UTC", cfg.RuntimeParams["TimeZone"])
	}
}
