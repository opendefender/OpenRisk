// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #813 (D-056) — migration 0084 drops the legacy control_evidences table, once
// it has proven that every row lives in the evidence library. These tests start
// from the previous release's shape (AutoMigrate, SQL layer at 83, the legacy
// table in its 0028 + 0044 shape) and upgrade it the way a boot does, through
// the real RunMigrations.
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

const lastVersionBefore0084 = 83

var (
	tenantA813  = uuid.MustParse("00000000-0000-4000-8813-00000000000a")
	tenantB813  = uuid.MustParse("00000000-0000-4000-8813-00000000000b")
	controlA813 = uuid.MustParse("00000000-0000-4000-8813-0000000000ca")
	controlB813 = uuid.MustParse("00000000-0000-4000-8813-0000000000cb")

	// Legacy rows each test checks by name.
	movedByBoot813 = uuid.MustParse("00000000-0000-4000-8813-000000000001") // already in the library, edited since
	neverMoved813  = uuid.MustParse("00000000-0000-4000-8813-000000000002") // the boot backfill missed it
	deleted813     = uuid.MustParse("00000000-0000-4000-8813-000000000003") // soft-deleted legacy row
	tenantB813Row  = uuid.MustParse("00000000-0000-4000-8813-000000000004") // another tenant's attachment
)

// pre0084DB builds the previous release's schema in a throwaway PostgreSQL
// schema. withLegacyTable false leaves control_evidences out, as on a database
// installed after the table left AutoMigrate.
func pre0084DB(t *testing.T, withLegacyTable bool) *upgradeDB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "mig0084_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]

	admin := openGorm(t, base)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() { _ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error })

	params := map[string]string{"search_path": schema + ",public"}
	if parsed, err := url.Parse(base); err == nil && parsed.Query().Get("sslmode") == "" {
		params["sslmode"] = "disable"
	}
	u := &upgradeDB{dsn: withParams(t, base, params)}
	u.db = openGorm(t, u.dsn)

	require.NoError(t, database.PrepareForAutoMigrate(u.db))
	require.NoError(t, u.db.AutoMigrate(schemaModels()...))
	require.NoError(t, newMigrator(t, u.dsn).Migrate(lastVersionBefore0084))

	for _, c := range []struct{ id, tenant uuid.UUID }{{controlA813, tenantA813}, {controlB813, tenantB813}} {
		fw := uuid.New()
		require.NoError(t, u.db.Exec(`
			INSERT INTO compliance_frameworks (id, tenant_id, name, version, catalog_key, created_at, updated_at)
			VALUES (?, ?, 'ISO 27001', '2022', '', now(), now())`, fw, c.tenant).Error)
		require.NoError(t, u.db.Exec(`
			INSERT INTO compliance_controls (id, tenant_id, framework_id, reference_code, name, source_reference, status, created_at, updated_at)
			VALUES (?, ?, ?, 'A.5.1', 'Policies', '', 'not_implemented', now(), now())`, c.id, c.tenant, fw).Error)
	}

	if withLegacyTable {
		require.NoError(t, u.db.Exec(`
			CREATE TABLE control_evidences (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    tenant_id UUID NOT NULL,
			    control_id UUID NOT NULL,
			    filename VARCHAR(255) NOT NULL DEFAULT '',
			    url TEXT NOT NULL DEFAULT '',
			    description TEXT,
			    uploaded_by UUID,
			    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			    deleted_at TIMESTAMP,
			    owner_id UUID, assignee_id UUID, reviewer_id UUID,
			    CONSTRAINT fk_evidences_control FOREIGN KEY (control_id)
			        REFERENCES compliance_controls(id) ON DELETE CASCADE)`).Error)
	}
	return u
}

// seedLegacyRows writes the four named legacy rows, and the library row a
// previous boot made for movedByBoot813, edited by a user since.
func (u *upgradeDB) seedLegacyRows(t *testing.T) {
	t.Helper()
	uploader := uuid.New()
	require.NoError(t, u.db.Exec(`
		INSERT INTO control_evidences (id, tenant_id, control_id, filename, url, description, uploaded_by, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, 'policy.pdf', 'evidence/a/policy.pdf', 'signed policy', ?, now() - interval '400 days', now() - interval '300 days', NULL),
		       (?, ?, ?, '',           '',                      NULL,            NULL, now() - interval '200 days', now() - interval '200 days', NULL),
		       (?, ?, ?, 'old.pdf',    'evidence/a/old.pdf',    NULL,            ?, now() - interval '500 days', now() - interval '90 days', now() - interval '90 days'),
		       (?, ?, ?, 'b.pdf',      'evidence/b/b.pdf',      'tenant b',      ?, now() - interval '100 days', now() - interval '100 days', NULL)`,
		movedByBoot813, tenantA813, controlA813, uploader,
		neverMoved813, tenantA813, controlA813,
		deleted813, tenantA813, controlA813, uploader,
		tenantB813Row, tenantB813, controlB813, uploader).Error)

	require.NoError(t, u.db.Exec(`
		INSERT INTO evidences (id, tenant_id, title, type, description, file_ref, filename,
		        collected_at, collected_by, review, source, owner_id, created_at, updated_at)
		VALUES (?, ?, 'Renamed by the user', 'document', 'signed policy', 'evidence/a/policy.pdf', 'policy.pdf',
		        now() - interval '400 days', NULL, 'accepted', 'manual', NULL, now() - interval '400 days', now())`,
		movedByBoot813, tenantA813).Error)
	require.NoError(t, u.db.Exec(`
		INSERT INTO evidence_control_links (id, tenant_id, evidence_id, control_id, created_at)
		VALUES (gen_random_uuid(), ?, ?, ?, now())`, tenantA813, movedByBoot813, controlA813).Error)
}

func (u *upgradeDB) hasTable(t *testing.T, name string) bool {
	t.Helper()
	return u.count(t, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = ? AND n.nspname = current_schema()`, name) == 1
}

// TestMigration0084_Success — every legacy row is in the library, in its own
// tenant, before the table goes; what a previous boot moved is left as the
// user edited it.
func TestMigration0084_Success(t *testing.T) {
	u := pre0084DB(t, true)
	u.seedLegacyRows(t)
	linksBefore := u.count(t, `SELECT count(*) FROM evidence_control_links`)

	_, err := u.bootUpgrade(t)
	require.NoError(t, err)

	assert.False(t, u.hasTable(t, "control_evidences"), "the legacy table is dropped")
	v, dirty := u.version(t)
	assert.GreaterOrEqual(t, v, uint(84), "boot runs every migration, 0084 included")
	assert.False(t, dirty)

	for _, row := range []struct{ id, tenant uuid.UUID }{
		{movedByBoot813, tenantA813}, {neverMoved813, tenantA813}, {deleted813, tenantA813}, {tenantB813Row, tenantB813},
	} {
		assert.Equal(t, int64(1), u.count(t, `SELECT count(*) FROM evidences WHERE id = ? AND tenant_id = ?`, row.id, row.tenant),
			"legacy row %s is in the library, in its own tenant", row.id)
	}

	assert.Equal(t, int64(1), u.count(t, `SELECT count(*) FROM evidences WHERE id = ? AND title = 'Renamed by the user'`, movedByBoot813),
		"a row a previous boot moved keeps the user's edit")
	assert.Equal(t, int64(1), u.count(t, `SELECT count(*) FROM evidences
		WHERE id = ? AND title = 'Preuve' AND file_ref = '' AND filename = '' AND review = 'accepted' AND source = 'manual'`, neverMoved813),
		"a row the boot never moved arrives with the backfill's defaults")
	assert.Equal(t, int64(1), u.count(t, `SELECT count(*) FROM evidences WHERE id = ? AND deleted_at IS NOT NULL`, deleted813),
		"a soft-deleted legacy row stays deleted in the library")

	assert.Equal(t, int64(1), u.count(t, `SELECT count(*) FROM evidence_control_links WHERE evidence_id = ? AND control_id = ? AND tenant_id = ?`,
		neverMoved813, controlA813, tenantA813))
	assert.Equal(t, int64(1), u.count(t, `SELECT count(*) FROM evidence_control_links WHERE evidence_id = ? AND control_id = ? AND tenant_id = ?`,
		tenantB813Row, controlB813, tenantB813))
	assert.Equal(t, int64(0), u.count(t, `SELECT count(*) FROM evidence_control_links WHERE evidence_id = ?`, deleted813),
		"a deleted legacy row gets no control link")
	assert.Equal(t, linksBefore+2, u.count(t, `SELECT count(*) FROM evidence_control_links`),
		"exactly the two missing links are added, the existing one is not duplicated")
}

// TestMigration0084_NotFound — no legacy table (every install since it left
// AutoMigrate), or an empty one: nothing to move, the library is untouched.
func TestMigration0084_NotFound(t *testing.T) {
	t.Run("no table", func(t *testing.T) {
		u := pre0084DB(t, false)
		before := u.count(t, `SELECT count(*) FROM evidences`)
		_, err := u.bootUpgrade(t)
		require.NoError(t, err)
		assert.False(t, u.hasTable(t, "control_evidences"))
		assert.Equal(t, before, u.count(t, `SELECT count(*) FROM evidences`))
		v, dirty := u.version(t)
		assert.GreaterOrEqual(t, v, uint(84), "boot runs every migration, 0084 included")
		assert.False(t, dirty)
	})
	t.Run("empty table", func(t *testing.T) {
		u := pre0084DB(t, true)
		_, err := u.bootUpgrade(t)
		require.NoError(t, err)
		assert.False(t, u.hasTable(t, "control_evidences"))
		assert.Equal(t, int64(0), u.count(t, `SELECT count(*) FROM evidences`))
	})
}

// TestMigration0084_Unauthorized — a legacy row whose id is already taken by
// ANOTHER tenant's evidence cannot be proven to live in the library. The
// migration refuses, rolls back everything it did, and keeps the table and
// its rows: no cross-tenant link is written, nothing is dropped.
func TestMigration0084_Unauthorized(t *testing.T) {
	u := pre0084DB(t, true)
	u.seedLegacyRows(t)
	collision := uuid.MustParse("00000000-0000-4000-8813-000000000005")
	require.NoError(t, u.db.Exec(`
		INSERT INTO evidences (id, tenant_id, title, type, file_ref, filename, collected_at, review, source, created_at, updated_at)
		VALUES (?, ?, 'tenant b proof', 'document', '', '', now(), 'accepted', 'manual', now(), now())`,
		collision, tenantB813).Error)
	require.NoError(t, u.db.Exec(`
		INSERT INTO control_evidences (id, tenant_id, control_id, filename, url)
		VALUES (?, ?, ?, 'a.pdf', 'evidence/a/a.pdf')`, collision, tenantA813, controlA813).Error)
	evidencesBefore := u.count(t, `SELECT count(*) FROM evidences`)
	linksBefore := u.count(t, `SELECT count(*) FROM evidence_control_links`)

	_, err := u.bootUpgrade(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nothing was dropped")

	assert.True(t, u.hasTable(t, "control_evidences"), "the legacy table is kept")
	assert.Equal(t, int64(5), u.count(t, `SELECT count(*) FROM control_evidences`), "and every row in it")
	assert.Equal(t, evidencesBefore, u.count(t, `SELECT count(*) FROM evidences`), "the backfill rolled back")
	assert.Equal(t, linksBefore, u.count(t, `SELECT count(*) FROM evidence_control_links`), "no link written, none across tenants")
	assert.Equal(t, int64(0), u.count(t, `SELECT count(*) FROM evidence_control_links WHERE evidence_id = ? AND tenant_id = ?`, collision, tenantA813))
	_, dirty := u.version(t)
	assert.True(t, dirty, "golang-migrate marks the version dirty, so the boot stops")
}

// TestMigration0084_RefusesWithoutLibrary — legacy rows on a database whose
// SQL layer runs before AutoMigrate ever created the library: refuse rather
// than drop rows that have nowhere to go.
func TestMigration0084_RefusesWithoutLibrary(t *testing.T) {
	u := pre0084DB(t, true)
	u.seedLegacyRows(t)
	require.NoError(t, u.db.Exec(`DROP TABLE evidence_control_links`).Error)

	// The fixture's search_path ends in public, where a migrated database (CI)
	// has its own evidence_control_links, and to_regclass would find that one.
	// Run the SQL layer on this schema alone, as a real database has one.
	var schema string
	require.NoError(t, u.db.Raw(`SELECT current_schema()`).Scan(&schema).Error)
	_, err := u.runSQLLayer(t, withParams(t, u.dsn, map[string]string{"search_path": schema}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evidence library does not exist yet")
	assert.Equal(t, int64(4), u.count(t, `SELECT count(*) FROM control_evidences`))
}

// TestMigration0084_Rollback — down recreates the table EMPTY (D-056 accepts
// the loss: the rows live on in the library), with its last shape and its
// foreign key; up again drops the empty table.
func TestMigration0084_Rollback(t *testing.T) {
	u := pre0084DB(t, true)
	u.seedLegacyRows(t)
	_, err := u.bootUpgrade(t)
	require.NoError(t, err)
	libraryAfterUp := u.count(t, `SELECT count(*) FROM evidences`)

	m := newMigrator(t, u.dsn)
	// Down to 83, not Steps(-1): later migrations (0085 on) sit above 0084.
	require.NoError(t, m.Migrate(lastVersionBefore0084))
	require.True(t, u.hasTable(t, "control_evidences"))
	assert.Equal(t, int64(0), u.count(t, `SELECT count(*) FROM control_evidences`), "recreated empty")
	assert.Equal(t, libraryAfterUp, u.count(t, `SELECT count(*) FROM evidences`), "down leaves the library alone")
	assert.Equal(t, int64(13), u.count(t, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'control_evidences'`), "0028 + 0044 columns")
	assert.Equal(t, int64(1), u.count(t, `SELECT count(*) FROM pg_constraint
		WHERE conname = 'fk_evidences_control' AND conrelid = 'control_evidences'::regclass`))
	v, dirty := u.version(t)
	assert.Equal(t, uint(83), v)
	assert.False(t, dirty)

	// down twice in a row would fail on the version chain, not on this file;
	// the file itself is idempotent.
	require.NoError(t, u.db.Exec(mustReadMigration(t, "0084_drop_control_evidences.down.sql")).Error)

	require.NoError(t, m.Steps(1))
	assert.False(t, u.hasTable(t, "control_evidences"))
	assert.Equal(t, libraryAfterUp, u.count(t, `SELECT count(*) FROM evidences`))
}

func mustReadMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(migrationsDir + "/" + name)
	require.NoError(t, err)
	return string(b)
}
