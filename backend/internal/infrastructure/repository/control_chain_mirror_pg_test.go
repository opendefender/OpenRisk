// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newMirrorEnvPG builds the same schema in a throwaway PostgreSQL schema (a
// search_path of its own), so the mirror's transactions and ON CONFLICT clauses
// run on the engine that matters. It skips when DATABASE_URL is unset, as the
// other *_pg_test.go files do; the CI integration job sets it.
func newMirrorEnvPG(t *testing.T) *mirrorEnv {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	schema := "mirror1413_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		if s, err := admin.DB(); err == nil {
			_ = s.Close()
		}
	})
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := gorm.Open(postgres.Open(dsn+sep+"search_path="+schema), cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		if s, err := db.DB(); err == nil {
			_ = s.Close()
		}
	})
	return newMirrorEnv(t, db)
}

func TestControlChainMirror_Postgres(t *testing.T) {
	runMirrorSuite(t, mirrorSuite{env: newMirrorEnvPG})
}
