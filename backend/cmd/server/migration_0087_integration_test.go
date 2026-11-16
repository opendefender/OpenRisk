// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #1428 — migration 0087 creates control_exceptions. These tests start from the
// previous release's shape (AutoMigrate without the new table, SQL layer at 86)
// and run the migration through golang-migrate, so the SQL DDL is what is under
// test: its constraints, its tenant-leading index, its agreement with the row
// type AutoMigrate builds, and its down.
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/infrastructure/controlexception"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

const lastVersionBefore0087 = 86

var tenantA1428 = uuid.MustParse("00000000-0000-4000-8428-00000000000a")

type exc1428 struct{ *upgradeDB }

// pre0087DB builds the previous release's schema in a throwaway PostgreSQL
// schema: every model but control_exceptions, SQL layer at 86.
func pre0087DB(t *testing.T) *exc1428 {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "mig0087_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	admin := openGorm(t, base)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() { _ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error })

	params := map[string]string{"search_path": schema}
	if parsed, err := url.Parse(base); err == nil && parsed.Query().Get("sslmode") == "" {
		params["sslmode"] = "disable"
	}
	c := &exc1428{upgradeDB: &upgradeDB{dsn: withParams(t, base, params)}}
	c.db = openGorm(t, c.dsn)

	var previous []interface{}
	for _, m := range schemaModels() {
		if _, isException := m.(*controlexception.ExceptionRow); isException {
			continue
		}
		previous = append(previous, m)
	}
	require.NoError(t, database.PrepareForAutoMigrate(c.db))
	require.NoError(t, c.db.AutoMigrate(previous...))
	require.NoError(t, newMigrator(t, c.dsn).Migrate(lastVersionBefore0087))
	require.False(t, c.hasTable(t, controlexception.TableControlExceptions), "control_exceptions belongs to 0087")
	return c
}

func (c *exc1428) hasTable(t *testing.T, name string) bool {
	t.Helper()
	return c.count(t, `SELECT count(*) FROM pg_class cl JOIN pg_namespace n ON n.oid = cl.relnamespace
		WHERE cl.relname = ? AND n.nspname = current_schema()`, name) == 1
}

func (c *exc1428) tables(t *testing.T) int64 {
	t.Helper()
	return c.count(t, `SELECT count(*) FROM pg_class cl JOIN pg_namespace n ON n.oid = cl.relnamespace
		WHERE n.nspname = current_schema() AND cl.relkind = 'r'`)
}

func (c *exc1428) shape(t *testing.T) [2]int64 {
	t.Helper()
	return [2]int64{
		c.count(t, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND tablename = 'control_exceptions'`),
		c.count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid = 'control_exceptions'::regclass`),
	}
}

func (c *exc1428) insert(reason string, starts, expires time.Time, revokedAt *time.Time, revokedBy *uuid.UUID) error {
	return c.db.Exec(`INSERT INTO control_exceptions
		(id, tenant_id, control_id, owner_id, reason, approved_by, starts_at, expires_at, revoked_at, revoked_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New(), tenantA1428, uuid.New(), uuid.New(), reason, uuid.New(), starts, expires, revokedAt, revokedBy).Error
}

// assertRefusals: the table, however it was built, refuses what the design forbids.
func (c *exc1428) assertRefusals(t *testing.T) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, c.insert("a reason", now, now.Add(time.Hour), nil, nil), "a valid exception is stored")
	assert.Error(t, c.insert("a reason", now, now, nil, nil), "expires_at must be after starts_at")
	assert.Error(t, c.insert("a reason", now, now.Add(-time.Hour), nil, nil), "expires_at before starts_at")
	assert.Error(t, c.insert("   ", now, now.Add(time.Hour), nil, nil), "a blank reason (SQL trim() strips spaces only; the use case trims all whitespace)")
	by := uuid.New()
	assert.Error(t, c.insert("a reason", now, now.Add(time.Hour), &now, nil), "revoked_at without revoked_by")
	assert.Error(t, c.insert("a reason", now, now.Add(time.Hour), nil, &by), "revoked_by without revoked_at")
	assert.NoError(t, c.insert("a reason", now, now.Add(time.Hour), &now, &by), "a complete revocation")
	err := c.db.Exec(`INSERT INTO control_exceptions (id, control_id, owner_id, reason, approved_by, starts_at, expires_at)
		VALUES (?, ?, ?, 'r', ?, now(), now() + interval '1 hour')`, uuid.New(), uuid.New(), uuid.New(), uuid.New()).Error
	assert.Error(t, err, "tenant_id is NOT NULL")
}

func TestMigration0087_Success(t *testing.T) {
	c := pre0087DB(t)
	tablesBefore := c.tables(t)

	require.NoError(t, newMigrator(t, c.dsn).Migrate(87))
	v, dirty := c.version(t)
	assert.Equal(t, uint(87), v)
	assert.False(t, dirty)
	require.True(t, c.hasTable(t, controlexception.TableControlExceptions))
	assert.Equal(t, tablesBefore+1, c.tables(t), "0087 adds exactly one table")

	// tenant_id uuid NOT NULL, and an index leads with it.
	assert.Equal(t, int64(1), c.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema()
		AND table_name = 'control_exceptions' AND column_name = 'tenant_id' AND data_type = 'uuid' AND is_nullable = 'NO'`))
	assert.Equal(t, int64(1), c.count(t, `SELECT count(*) FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = i.indkey[0]
		WHERE i.indrelid = 'control_exceptions'::regclass AND a.attname = 'tenant_id' AND NOT i.indisprimary`))

	// The SQL-built table is the one the row type describes: AutoMigrate over it
	// (what the next boot does) neither fails nor adds an index or a check.
	before := c.shape(t)
	require.NoError(t, c.db.AutoMigrate(controlexception.Models()...))
	assert.Equal(t, before, c.shape(t), "AutoMigrate over the SQL-built table changes nothing")

	c.assertRefusals(t)
}

func TestMigration0087_Rollback(t *testing.T) {
	c := pre0087DB(t)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(87))
	withTable := c.tables(t)

	require.NoError(t, newMigrator(t, c.dsn).Steps(-1))
	assert.False(t, c.hasTable(t, controlexception.TableControlExceptions), "gone after down")
	v, dirty := c.version(t)
	assert.Equal(t, uint(86), v)
	assert.False(t, dirty)
	assert.Equal(t, withTable-1, c.tables(t), "down touched no other table")
	assert.True(t, c.hasTable(t, "controls"), "the control chain is untouched")

	// Reversible.
	require.NoError(t, newMigrator(t, c.dsn).Migrate(87))
	assert.True(t, c.hasTable(t, controlexception.TableControlExceptions))
}

// TestMigration0087_AutoMigrateFirstThenSQL — a compose boot builds the table
// with AutoMigrate, and the SQL layer that follows must be a no-op over it.
func TestMigration0087_AutoMigrateFirstThenSQL(t *testing.T) {
	c := pre0087DB(t)
	require.NoError(t, c.db.AutoMigrate(controlexception.Models()...))
	c.assertRefusals(t) // the AutoMigrate-built table refuses what 0087's refuses

	before := c.shape(t)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(87))
	assert.Equal(t, before, c.shape(t), "0087 over an AutoMigrate-built table changes nothing")
	v, dirty := c.version(t)
	assert.Equal(t, uint(87), v)
	assert.False(t, dirty)
}
