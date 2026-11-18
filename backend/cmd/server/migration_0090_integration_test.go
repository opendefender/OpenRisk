// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #555 — migration 0090 creates control_status_evaluations. These tests start
// from the previous release's shape (AutoMigrate without the new table, SQL layer
// at 88; 0089 is #1456's) and run the migration through golang-migrate, so the SQL
// DDL is what is under test: its constraints, its tenant-leading key, its
// agreement with the row type AutoMigrate builds, and its down.
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

	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

const lastVersionBefore0090 = 88

var tenantA555 = uuid.MustParse("00000000-0000-4000-8555-00000000000a")

type status555 struct{ *upgradeDB }

// pre0090DB builds the previous release's schema in a throwaway PostgreSQL
// schema: every model but control_status_evaluations, SQL layer at 88.
func pre0090DB(t *testing.T) *status555 {
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
	c := &status555{upgradeDB: &upgradeDB{dsn: withParams(t, base, params)}}
	c.db = openGorm(t, c.dsn)

	var previous []interface{}
	for _, m := range schemaModels() {
		if _, isStatus := m.(*controlstatus.EvaluationRow); isStatus {
			continue
		}
		previous = append(previous, m)
	}
	require.NoError(t, database.PrepareForAutoMigrate(c.db))
	require.NoError(t, c.db.AutoMigrate(previous...))
	require.NoError(t, newMigrator(t, c.dsn).Migrate(lastVersionBefore0090))
	require.False(t, c.hasTable(t, controlstatus.TableControlStatusEvaluations), "control_status_evaluations belongs to 0090")
	return c
}

func (c *status555) hasTable(t *testing.T, name string) bool {
	t.Helper()
	return c.count(t, `SELECT count(*) FROM pg_class cl JOIN pg_namespace n ON n.oid = cl.relnamespace
		WHERE cl.relname = ? AND n.nspname = current_schema()`, name) == 1
}

func (c *status555) tables(t *testing.T) int64 {
	t.Helper()
	return c.count(t, `SELECT count(*) FROM pg_class cl JOIN pg_namespace n ON n.oid = cl.relnamespace
		WHERE n.nspname = current_schema() AND cl.relkind = 'r'`)
}

func (c *status555) shape(t *testing.T) [2]int64 {
	t.Helper()
	return [2]int64{
		c.count(t, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND tablename = 'control_status_evaluations'`),
		c.count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid = 'control_status_evaluations'::regclass`),
	}
}

func (c *status555) insert(tenant, control uuid.UUID, status, underlying string, fresh, counted int, percent float64) error {
	return c.db.Exec(`INSERT INTO control_status_evaluations
		(tenant_id, control_id, status, underlying_status, fresh_signals, counted_signals, freshness_percent, evaluated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, tenant, control, status, underlying, fresh, counted, percent, time.Now().UTC()).Error
}

// assertRefusals: the table, however it was built, refuses what the design forbids.
func (c *status555) assertRefusals(t *testing.T) {
	t.Helper()
	control := uuid.New()
	require.NoError(t, c.insert(tenantA555, control, "pass", "pass", 1, 1, 100), "a valid row is stored")
	assert.Error(t, c.insert(tenantA555, control, "pass", "pass", 1, 1, 100), "one row per (tenant, control)")
	assert.NoError(t, c.insert(uuid.New(), control, "fail", "fail", 0, 1, 0), "the same control id under another tenant is another row")
	assert.NoError(t, c.insert(tenantA555, uuid.New(), "exception_active", "fail", 0, 1, 0))
	assert.Error(t, c.insert(tenantA555, uuid.New(), "green", "pass", 0, 0, 0), "status is the five values")
	assert.Error(t, c.insert(tenantA555, uuid.New(), "pass", "exception_active", 0, 0, 0), "underlying is never exception_active")
	assert.Error(t, c.insert(tenantA555, uuid.New(), "pass", "pass", 2, 1, 100), "fresh cannot exceed counted")
	assert.Error(t, c.insert(tenantA555, uuid.New(), "pass", "pass", 1, 1, 100.01), "percent is at most 100")
	err := c.db.Exec(`INSERT INTO control_status_evaluations (control_id, status, underlying_status, evaluated_at)
		VALUES (?, 'pass', 'pass', now())`, uuid.New()).Error
	assert.Error(t, err, "tenant_id is NOT NULL")
}

func TestMigration0090_Success(t *testing.T) {
	c := pre0090DB(t)
	tablesBefore := c.tables(t)

	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	v, dirty := c.version(t)
	assert.Equal(t, uint(90), v)
	assert.False(t, dirty)
	require.True(t, c.hasTable(t, controlstatus.TableControlStatusEvaluations))
	assert.Equal(t, tablesBefore+1, c.tables(t), "0090 adds exactly one table")

	// tenant_id uuid NOT NULL, and the primary key leads with it.
	assert.Equal(t, int64(1), c.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema()
		AND table_name = 'control_status_evaluations' AND column_name = 'tenant_id' AND data_type = 'uuid' AND is_nullable = 'NO'`))
	assert.Equal(t, int64(1), c.count(t, `SELECT count(*) FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = i.indkey[0]
		WHERE i.indrelid = 'control_status_evaluations'::regclass AND a.attname = 'tenant_id' AND i.indisprimary`))

	// The SQL-built table is the one the row type describes: AutoMigrate over it
	// (what the next boot does) neither fails nor adds an index or a check.
	before := c.shape(t)
	require.NoError(t, c.db.AutoMigrate(controlstatus.Models()...))
	assert.Equal(t, before, c.shape(t), "AutoMigrate over the SQL-built table changes nothing")

	c.assertRefusals(t)
}

func TestMigration0090_Rollback(t *testing.T) {
	c := pre0090DB(t)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	withTable := c.tables(t)

	require.NoError(t, newMigrator(t, c.dsn).Steps(-1))
	assert.False(t, c.hasTable(t, controlstatus.TableControlStatusEvaluations), "gone after down")
	v, dirty := c.version(t)
	assert.Equal(t, uint(88), v)
	assert.False(t, dirty)
	assert.Equal(t, withTable-1, c.tables(t), "down touched no other table")
	assert.True(t, c.hasTable(t, "controls"), "the control chain is untouched")
	assert.True(t, c.hasTable(t, "control_exceptions"), "the exceptions are untouched")

	// Reversible.
	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	assert.True(t, c.hasTable(t, controlstatus.TableControlStatusEvaluations))
}

// TestMigration0090_AutoMigrateFirstThenSQL — a compose boot builds the table
// with AutoMigrate, and the SQL layer that follows must be a no-op over it.
func TestMigration0090_AutoMigrateFirstThenSQL(t *testing.T) {
	c := pre0090DB(t)
	require.NoError(t, c.db.AutoMigrate(controlstatus.Models()...))
	c.assertRefusals(t) // the AutoMigrate-built table refuses what 0090's refuses

	before := c.shape(t)
	require.NoError(t, newMigrator(t, c.dsn).Migrate(90))
	assert.Equal(t, before, c.shape(t), "0090 over an AutoMigrate-built table changes nothing")
	v, dirty := c.version(t)
	assert.Equal(t, uint(90), v)
	assert.False(t, dirty)
}
