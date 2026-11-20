// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #1483 — migration 0090 adds the content-version stamp on frameworks and the
// pack provenance columns, plus one partial unique index. The previous release's
// shape is rebuilt by AutoMigrate over the current models and then stripping the
// 0090 columns, with the SQL layer at 89, so the SQL DDL is what is under test.
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

const lastVersionBefore0090 = 89

var (
	tenantA1483 = uuid.MustParse("00000000-0000-4000-8483-00000000000a")
	tenantB1483 = uuid.MustParse("00000000-0000-4000-8483-00000000000b")
)

type pack1483 struct{ *upgradeDB }

var packColumns = map[string][]string{
	"compliance_frameworks": {"catalog_content_version", "catalog_crosswalks_version", "catalog_pending_kinds"},
	"compliance_controls":   {"pack_key", "pack_item_key", "pack_version", "pack_base", "pack_base_hash", "pack_retired_in", "pack_reverted_by"},
	"control_crosswalks":    {"pack_key", "pack_item_key", "pack_version", "pack_base", "pack_base_hash", "pack_retired_in", "pack_reverted_by"},
	"controls":              {"pack_reverted_by"},
}

// strip0090 removes what 0090 adds, from a schema AutoMigrate just built.
func (c *pack1483) strip0090(t *testing.T) {
	t.Helper()
	require.NoError(t, c.db.Exec(`DROP INDEX IF EXISTS ux_compliance_controls_pack_item`).Error)
	require.NoError(t, c.db.Exec(`DROP INDEX IF EXISTS ux_control_crosswalks_pack_item`).Error)
	for table, cols := range packColumns {
		for _, col := range cols {
			require.NoError(t, c.db.Exec(fmt.Sprintf(`ALTER TABLE %s DROP COLUMN IF EXISTS %s`, table, col)).Error)
		}
	}
}

func pre0090DB(t *testing.T) *pack1483 {
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
	c := &pack1483{upgradeDB: &upgradeDB{dsn: withParams(t, base, params)}}
	c.db = openGorm(t, c.dsn)

	require.NoError(t, database.PrepareForAutoMigrate(c.db))
	require.NoError(t, c.db.AutoMigrate(schemaModels()...))
	c.strip0090(t)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(lastVersionBefore0090))
	return c
}

func (c *pack1483) hasColumn(t *testing.T, table, col string) bool {
	t.Helper()
	return c.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema()
		AND table_name = ? AND column_name = ?`, table, col) == 1
}

func (c *pack1483) seedRows(t *testing.T) {
	t.Helper()
	fw, ctl := uuid.New(), uuid.New()
	require.NoError(t, c.db.Exec(`INSERT INTO compliance_frameworks (id, tenant_id, name, version, created_at, updated_at)
		VALUES (?, ?, 'F', '1', now(), now())`, fw, tenantA1483).Error)
	require.NoError(t, c.db.Exec(`INSERT INTO compliance_controls (id, tenant_id, framework_id, reference_code, name, created_at, updated_at)
		VALUES (?, ?, ?, 'A.1', 'N', now(), now())`, ctl, tenantA1483, fw).Error)
	require.NoError(t, c.db.Exec(`INSERT INTO control_crosswalks (id, tenant_id, source_control_id, target_control_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, now(), now())`, uuid.New(), tenantA1483, ctl, uuid.New()).Error)
}

func (c *pack1483) shape(t *testing.T) [2]int64 {
	t.Helper()
	return [2]int64{
		c.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema()
			AND table_name IN ('compliance_frameworks','compliance_controls','control_crosswalks','controls')`),
		c.count(t, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema()
			AND tablename IN ('compliance_frameworks','compliance_controls','control_crosswalks','controls')`),
	}
}

// assertLeadsOnTenant checks from pg_index that the unique index's first column is tenant_id.
func (c *pack1483) assertLeadsOnTenant(t *testing.T, index string) {
	t.Helper()
	assert.EqualValues(t, 1, c.count(t, `SELECT count(*) FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = i.indkey[0]
		WHERE i.indexrelid = ?::regclass AND a.attname = 'tenant_id' AND i.indisunique`, index), "tenant_id leads %s", index)
	var def string
	require.NoError(t, c.db.Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?`, index).Scan(&def).Error)
	assert.NotContains(t, def, "deleted_at", "deleted rows are inside %s", index)
	assert.Contains(t, def, "WHERE")
}

func (c *pack1483) insertLink(tenant uuid.UUID, packKey, item string, src, tgt uuid.UUID, deleted bool) error {
	del := "NULL"
	if deleted {
		del = "now()"
	}
	return c.db.Exec(fmt.Sprintf(`INSERT INTO control_crosswalks (id, tenant_id, source_control_id, target_control_id, pack_key, pack_item_key, deleted_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, %s, now(), now())`, del), uuid.New(), tenant, src, tgt, packKey, item).Error
}

func (c *pack1483) insertControl(fw, tenant uuid.UUID, ref, packKey, item string, deleted bool) error {
	del := "NULL"
	if deleted {
		del = "now()"
	}
	return c.db.Exec(fmt.Sprintf(`INSERT INTO compliance_controls (id, tenant_id, framework_id, reference_code, name, pack_key, pack_item_key, deleted_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'N', ?, ?, %s, now(), now())`, del), uuid.New(), tenant, fw, ref, packKey, item).Error
}

func TestMigration0090_Success(t *testing.T) {
	c := pre0090DB(t)
	for table, cols := range packColumns {
		for _, col := range cols {
			require.False(t, c.hasColumn(t, table, col), "%s.%s belongs to 0090", table, col)
		}
	}
	c.seedRows(t)

	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	v, dirty := c.version(t)
	assert.Equal(t, uint(90), v)
	assert.False(t, dirty)
	for table, cols := range packColumns {
		for _, col := range cols {
			assert.True(t, c.hasColumn(t, table, col), "%s.%s added", table, col)
		}
	}

	// Existing rows are untouched and carry the defaults: '' means unknown.
	assert.EqualValues(t, 1, c.count(t, `SELECT count(*) FROM compliance_frameworks
		WHERE catalog_content_version = '' AND catalog_crosswalks_version = '' AND catalog_pending_kinds = '[]'::jsonb`))
	assert.EqualValues(t, 1, c.count(t, `SELECT count(*) FROM compliance_controls
		WHERE pack_key = '' AND pack_item_key = '' AND pack_version = '' AND pack_base IS NULL AND pack_base_hash IS NULL
		AND pack_retired_in = '' AND pack_reverted_by IS NULL`))
	assert.EqualValues(t, 1, c.count(t, `SELECT count(*) FROM control_crosswalks WHERE pack_key = '' AND pack_base IS NULL`))
	assert.EqualValues(t, 1, c.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema()
		AND table_name = 'compliance_controls' AND column_name = 'pack_base_hash' AND data_type = 'character' AND character_maximum_length = 64`))

	// Criterion 11: both partial unique indexes lead on tenant_id.
	c.assertLeadsOnTenant(t, "ux_compliance_controls_pack_item")
	c.assertLeadsOnTenant(t, "ux_control_crosswalks_pack_item")
	var def string
	require.NoError(t, c.db.Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'ux_compliance_controls_pack_item'`).Scan(&def).Error)
	assert.Contains(t, def, "(tenant_id, framework_id, pack_key, pack_item_key)")
	require.NoError(t, c.db.Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'ux_control_crosswalks_pack_item'`).Scan(&def).Error)
	assert.Contains(t, def, "(tenant_id, pack_key, pack_item_key, source_control_id, target_control_id)")

	// Links: a deleted pack row still holds its key, another control pair does not,
	// another tenant does not, hand-made links are outside the index.
	s1, t1, s2, t2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, c.insertLink(tenantA1483, "crosswalks", "iso:A|soc2:B", s1, t1, true))
	assert.Error(t, c.insertLink(tenantA1483, "crosswalks", "iso:A|soc2:B", s1, t1, false), "a deleted pack link still holds its key")
	assert.NoError(t, c.insertLink(tenantA1483, "crosswalks", "iso:A|soc2:B", s2, t2, false), "the same item for another control pair (a second framework)")
	assert.NoError(t, c.insertLink(tenantB1483, "crosswalks", "iso:A|soc2:B", s1, t1, false), "another tenant")
	assert.NoError(t, c.insertLink(tenantA1483, "", "", s1, t1, false))
	assert.NoError(t, c.insertLink(tenantA1483, "", "", s1, t1, true), "hand-made links are outside the index")

	// Behaviour: deleted rows count, hand-made rows do not, tenants are independent.
	fw := uuid.New()
	require.NoError(t, c.insertControl(fw, tenantA1483, "A.1", "iso", "A.1", true))
	assert.Error(t, c.insertControl(fw, tenantA1483, "A.1", "iso", "A.1", false), "a deleted pack row still holds its key")
	assert.NoError(t, c.insertControl(uuid.New(), tenantA1483, "A.1", "iso", "A.1", false), "another framework")
	assert.NoError(t, c.insertControl(fw, tenantB1483, "A.1", "iso", "A.1", false), "another tenant")
	assert.NoError(t, c.insertControl(fw, tenantA1483, "H.1", "", "", false))
	assert.NoError(t, c.insertControl(fw, tenantA1483, "H.1", "", "", true), "hand-made rows are outside the index")
}

// Up/down/up on a database that holds rows, and an expand-only up file.
func TestMigration0090_Rollback(t *testing.T) {
	c := pre0090DB(t)
	c.seedRows(t)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	require.NoError(t, c.insertControl(uuid.New(), tenantA1483, "P.1", "iso", "P.1", false))

	require.NoError(t, newMigrator(t, c.dsn).Steps(-1))
	v, dirty := c.version(t)
	assert.Equal(t, uint(89), v)
	assert.False(t, dirty)
	for table, cols := range packColumns {
		for _, col := range cols {
			assert.False(t, c.hasColumn(t, table, col), "%s.%s gone after down", table, col)
		}
	}
	assert.EqualValues(t, 0, c.count(t, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname IN ('ux_compliance_controls_pack_item', 'ux_control_crosswalks_pack_item')`))
	assert.EqualValues(t, 2, c.count(t, `SELECT count(*) FROM compliance_controls`), "down loses no row")
	assert.EqualValues(t, 1, c.count(t, `SELECT count(*) FROM compliance_frameworks`))

	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	assert.True(t, c.hasColumn(t, "compliance_controls", "pack_key"))
	assert.EqualValues(t, 2, c.count(t, `SELECT count(*) FROM compliance_controls`))
}

func TestMigration0090_UpIsExpandOnly(t *testing.T) {
	raw, err := os.ReadFile("../../../migrations/0090_pack_provenance.up.sql")
	require.NoError(t, err)
	var stmts []string
	for _, line := range strings.Split(string(raw), "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		stmts = append(stmts, line)
	}
	sql := strings.ToUpper(strings.Join(stmts, " "))
	for _, forbidden := range []string{"DROP ", "RENAME", "ALTER COLUMN", "TRUNCATE", "DELETE FROM", "UPDATE "} {
		assert.NotContains(t, sql, forbidden, "0090 must only add columns, defaults and indexes")
	}
	assert.Regexp(t, regexp.MustCompile(`ADD COLUMN IF NOT EXISTS`), sql)
}

// A compose boot builds the columns and the index with AutoMigrate; the SQL layer
// that follows must be a no-op over them.
func TestMigration0090_AutoMigrateFirstThenSQL(t *testing.T) {
	c := pre0090DB(t)
	require.NoError(t, c.db.AutoMigrate(schemaModels()...))
	for table, cols := range packColumns {
		for _, col := range cols {
			require.True(t, c.hasColumn(t, table, col), "AutoMigrate builds %s.%s", table, col)
		}
	}
	c.assertLeadsOnTenant(t, "ux_compliance_controls_pack_item")
	c.assertLeadsOnTenant(t, "ux_control_crosswalks_pack_item")
	var def string
	require.NoError(t, c.db.Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'ux_control_crosswalks_pack_item'`).Scan(&def).Error)
	assert.Contains(t, def, "(tenant_id, pack_key, pack_item_key, source_control_id, target_control_id)")

	before := c.shape(t)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	assert.Equal(t, before, c.shape(t), "0090 over an AutoMigrate-built schema changes nothing")
	v, dirty := c.version(t)
	assert.Equal(t, uint(90), v)
	assert.False(t, dirty)
}
