// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #1478 — migration 0090 adds compliance_frameworks.catalog_version and catalog_revision. These
// tests start from the previous release's shape (AutoMigrate, then the new
// column removed, SQL layer at 89) and run the migration through golang-migrate,
// so the SQL is what is under test: no existing row loses data on up or down
// (AC5), and a row that predates the migration reads '' (AC2).
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

const lastVersionBefore0090 = 89

type fw1478 struct{ *upgradeDB }

type fwRow1478 struct {
	Name, Version, Description, CatalogKey string
}

func pre0090DB(t *testing.T) *fw1478 {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "mig0090_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	admin := openGorm(t, base)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() { _ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error })

	params := map[string]string{"search_path": schema}
	if parsed, err := url.Parse(base); err == nil && parsed.Query().Get("sslmode") == "" {
		params["sslmode"] = "disable"
	}
	c := &fw1478{upgradeDB: &upgradeDB{dsn: withParams(t, base, params)}}
	c.db = openGorm(t, c.dsn)

	require.NoError(t, database.PrepareForAutoMigrate(c.db))
	require.NoError(t, c.db.AutoMigrate(schemaModels()...))
	// The previous release had no catalog_version column.
	require.NoError(t, c.db.Exec(`ALTER TABLE compliance_frameworks DROP COLUMN catalog_version, DROP COLUMN catalog_revision`).Error)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(lastVersionBefore0090))
	require.False(t, c.hasCatalogVersion(t), "catalog_version belongs to 0090")
	return c
}

func (c *fw1478) hasCatalogVersion(t *testing.T) bool {
	t.Helper()
	return c.hasCol(t, "catalog_version") && c.hasCol(t, "catalog_revision")
}

func (c *fw1478) hasCol(t *testing.T, col string) bool {
	t.Helper()
	return c.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema()
		AND table_name = 'compliance_frameworks' AND column_name = ?`, col) == 1
}

func (c *fw1478) seed(t *testing.T, id uuid.UUID, name, catalogKey string) {
	t.Helper()
	require.NoError(t, c.db.Exec(`INSERT INTO compliance_frameworks (id, tenant_id, name, version, description, catalog_key)
		VALUES (?, ?, ?, '2022', 'desc', ?)`, id, uuid.New(), name, catalogKey).Error)
}

func (c *fw1478) read(t *testing.T, id uuid.UUID) fwRow1478 {
	t.Helper()
	var r fwRow1478
	require.NoError(t, c.db.Raw(`SELECT name, version, description, catalog_key FROM compliance_frameworks WHERE id = ?`, id).Row().Scan(
		&r.Name, &r.Version, &r.Description, &r.CatalogKey))
	return r
}

func (c *fw1478) catalogVersion(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var v, r string
	require.NoError(t, c.db.Raw(`SELECT catalog_version, catalog_revision FROM compliance_frameworks WHERE id = ?`, id).Row().Scan(&v, &r))
	return v + "|" + r
}

func TestMigration0090_Success(t *testing.T) {
	c := pre0090DB(t)
	imported, handBuilt := uuid.New(), uuid.New()
	c.seed(t, imported, "ISO 27001", "iso27001-2022")
	c.seed(t, handBuilt, "Internal", "")
	wantImported, wantHand := c.read(t, imported), c.read(t, handBuilt)

	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	v, dirty := c.version(t)
	assert.Equal(t, uint(90), v)
	assert.False(t, dirty)
	require.True(t, c.hasCatalogVersion(t))

	// AC5: nothing lost. AC2: the pre-existing rows read '' (never a guess).
	assert.Equal(t, wantImported, c.read(t, imported))
	assert.Equal(t, wantHand, c.read(t, handBuilt))
	assert.Equal(t, "|", c.catalogVersion(t, imported))
	assert.Equal(t, "|", c.catalogVersion(t, handBuilt))

	// NOT NULL with '' default.
	for _, col := range []string{"catalog_version", "catalog_revision"} {
		assert.Equal(t, int64(1), c.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema()
			AND table_name = 'compliance_frameworks' AND column_name = ? AND is_nullable = 'NO' AND column_default LIKE '''''%'`, col), col)
		assert.Error(t, c.db.Exec(`UPDATE compliance_frameworks SET `+col+` = NULL WHERE id = ?`, imported).Error, col)
	}
}

func TestMigration0090_Rollback(t *testing.T) {
	c := pre0090DB(t)
	id := uuid.New()
	c.seed(t, id, "ISO 27001", "iso27001-2022")
	want := c.read(t, id)

	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	require.NoError(t, c.db.Exec(`UPDATE compliance_frameworks SET catalog_version = '2022', catalog_revision = 'eb18cc12793c' WHERE id = ?`, id).Error)

	require.NoError(t, newMigrator(t, c.dsn).Steps(-1))
	v, dirty := c.version(t)
	assert.Equal(t, uint(89), v)
	assert.False(t, dirty)
	assert.False(t, c.hasCol(t, "catalog_version") || c.hasCol(t, "catalog_revision"), "both columns are gone after down")
	assert.Equal(t, want, c.read(t, id), "down loses no other column of the row")

	// Reversible: up again, the label is empty until the next import.
	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	assert.True(t, c.hasCatalogVersion(t))
	assert.Equal(t, want, c.read(t, id))
	assert.Equal(t, "|", c.catalogVersion(t, id))
}

// TestMigration0090_AutoMigrateFirstThenSQL — a compose boot adds the column
// with AutoMigrate; the SQL layer that follows must be a no-op over it.
func TestMigration0090_AutoMigrateFirstThenSQL(t *testing.T) {
	c := pre0090DB(t)
	id := uuid.New()
	c.seed(t, id, "ISO 27001", "iso27001-2022")
	require.NoError(t, c.db.AutoMigrate(schemaModels()...))
	require.True(t, c.hasCatalogVersion(t))

	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	v, dirty := c.version(t)
	assert.Equal(t, uint(90), v)
	assert.False(t, dirty)
	assert.Equal(t, "|", c.catalogVersion(t, id))
}
