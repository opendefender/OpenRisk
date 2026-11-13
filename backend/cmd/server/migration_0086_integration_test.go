// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #553 (D-108) — migration 0086 creates the six control-chain tables, and
// controlchain.Reconcile fills them from the legacy compliance tables. These
// tests start from the previous release's shape (AutoMigrate without the six
// tables, SQL layer at 85), seed a populated two-tenant database, and run the
// migration through golang-migrate, so the SQL DDL is what is under test.
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/scoring"
)

const lastVersionBefore0086 = 85

var (
	tenantA553 = uuid.MustParse("00000000-0000-4000-8553-00000000000a")
	tenantB553 = uuid.MustParse("00000000-0000-4000-8553-00000000000b")
)

// the legacy tables Reconcile reads and must never write.
var legacyTables553 = []string{"compliance_controls", "evidence_control_links", "risk_control_mappings"}

type chain553 struct {
	*upgradeDB

	frameworkA, frameworkB uuid.UUID
	reqImplemented         uuid.UUID
	reqNotApplicable       uuid.UUID
	reqSoftDeleted         uuid.UUID
	reqB, reqBNotApplic    uuid.UUID
	evA, evADeleted, evB   uuid.UUID
	riskA, riskB           uuid.UUID
}

func (c *chain553) sqlExec(t *testing.T, q string, args ...any) {
	t.Helper()
	require.NoError(t, c.db.Exec(q, args...).Error, q)
}

// pre0086DB builds the previous release's schema in a throwaway PostgreSQL
// schema (every model but the six new ones, SQL layer at 85) and seeds it.
func pre0086DB(t *testing.T) *chain553 {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "mig0086_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	admin := openGorm(t, base)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() { _ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error })

	// The schema alone, no public: a migrated public schema (CI) would otherwise
	// answer to_regclass and information_schema lookups for these tables.
	params := map[string]string{"search_path": schema}
	if parsed, err := url.Parse(base); err == nil && parsed.Query().Get("sslmode") == "" {
		params["sslmode"] = "disable"
	}
	c := &chain553{upgradeDB: &upgradeDB{dsn: withParams(t, base, params)}}
	c.db = openGorm(t, c.dsn)

	var previous []interface{}
	for _, m := range schemaModels() {
		switch m.(type) {
		case *controlchain.ControlRow, *controlchain.ControlImplementationRow, *controlchain.ControlTestRow,
			*controlchain.ControlTestResultRow, *controlchain.ControlGapFindingRow, *controlchain.CanonicalEdgeRow:
			continue
		}
		previous = append(previous, m)
	}
	require.NoError(t, database.PrepareForAutoMigrate(c.db))
	require.NoError(t, c.db.AutoMigrate(previous...))
	require.NoError(t, newMigrator(t, c.dsn).Migrate(lastVersionBefore0086))
	for _, table := range controlchain.Tables() {
		require.False(t, c.hasTable(t, table), "%s belongs to 0086", table)
	}
	c.seed(t)
	return c
}

func (c *chain553) hasTable(t *testing.T, name string) bool {
	t.Helper()
	return c.count(t, `SELECT count(*) FROM pg_class cl JOIN pg_namespace n ON n.oid = cl.relnamespace
		WHERE cl.relname = ? AND n.nspname = current_schema()`, name) == 1
}

func (c *chain553) seed(t *testing.T) {
	t.Helper()
	id := uuid.New
	c.frameworkA, c.frameworkB = id(), id()
	c.reqImplemented, c.reqNotApplicable, c.reqSoftDeleted, c.reqB, c.reqBNotApplic = id(), id(), id(), id(), id()
	c.evA, c.evADeleted, c.evB, c.riskA, c.riskB = id(), id(), id(), id(), id()

	for _, f := range []struct{ id, tenant uuid.UUID }{{c.frameworkA, tenantA553}, {c.frameworkB, tenantB553}} {
		c.sqlExec(t, `INSERT INTO compliance_frameworks (id, tenant_id, name, version, catalog_key, created_at, updated_at)
			VALUES (?, ?, 'ISO 27001', '2022', '', now(), now())`, f.id, f.tenant)
	}
	ctl := func(rid, tenant, fw uuid.UUID, ref, status string, deleted bool) {
		c.sqlExec(t, `INSERT INTO compliance_controls (id, tenant_id, framework_id, reference_code, name, description, source_reference, status, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, 'desc', '', ?, now() - interval '30 days', now(), CASE WHEN ? THEN now() ELSE NULL END)`,
			rid, tenant, fw, ref, "Control "+ref, status, deleted)
	}
	ctl(c.reqImplemented, tenantA553, c.frameworkA, "A.5.1", "implemented", false)
	ctl(uuid.New(), tenantA553, c.frameworkA, "A.5.2", "in_progress", false)
	ctl(uuid.New(), tenantA553, c.frameworkA, "A.5.3", "not_implemented", false)
	ctl(c.reqNotApplicable, tenantA553, c.frameworkA, "A.7.1", "not_applicable", false)
	ctl(c.reqSoftDeleted, tenantA553, c.frameworkA, "A.9.9", "in_progress", true)
	ctl(c.reqB, tenantB553, c.frameworkB, "A.5.1", "implemented", false) // same reference code as tenant A's
	ctl(c.reqBNotApplic, tenantB553, c.frameworkB, "A.7.1", "not_applicable", false)

	ev := func(eid, tenant uuid.UUID, deleted bool) {
		c.sqlExec(t, `INSERT INTO evidences (id, tenant_id, title, type, description, file_ref, filename, collected_at, review, source, created_at, updated_at, deleted_at)
			VALUES (?, ?, 'proof', 'document', '', '', '', now(), 'accepted', 'manual', now(), now(), CASE WHEN ? THEN now() ELSE NULL END)`,
			eid, tenant, deleted)
	}
	ev(c.evA, tenantA553, false)
	ev(c.evADeleted, tenantA553, true)
	ev(c.evB, tenantB553, false)

	link := func(evidence, control, tenant uuid.UUID) {
		c.sqlExec(t, `INSERT INTO evidence_control_links (id, tenant_id, evidence_id, control_id, note, created_at) VALUES (?, ?, ?, ?, 'why', now())`,
			uuid.New(), tenant, evidence, control)
	}
	link(c.evA, c.reqImplemented, tenantA553)
	link(c.evADeleted, c.reqImplemented, tenantA553) // soft-deleted evidence: carried
	link(c.evB, c.reqImplemented, tenantA553)        // another tenant's evidence: unplaceable
	link(c.evA, uuid.New(), tenantA553)              // no such control: unplaceable
	link(c.evB, c.reqB, tenantB553)

	risk := func(rid, tenant uuid.UUID, p, impact, score, smart float64) {
		c.sqlExec(t, `INSERT INTO risks (id, tenant_id, name, title, created_by, probability, impact, score, criticality, smart_score, smart_level, residual_risk, created_at, updated_at)
			VALUES (?, ?, 'r', 'r', ?, ?, ?, ?, 'high', ?, 'high', ?, now(), now())`,
			rid, tenant, uuid.New(), p, impact, score, smart, score/2)
	}
	risk(c.riskA, tenantA553, 0.8, 8, 6.4, 71.5)
	risk(c.riskB, tenantB553, 0.5, 6, 3.0, 40.25)

	mapping := func(tenant, fw, risk uuid.UUID, control any, deleted bool) {
		c.sqlExec(t, `INSERT INTO risk_control_mappings (id, tenant_id, risk_id, framework_id, control_id, note, source, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, '', 'manual', now(), now(), CASE WHEN ? THEN now() ELSE NULL END)`,
			uuid.New(), tenant, risk, fw, control, deleted)
	}
	mapping(tenantA553, c.frameworkA, c.riskA, c.reqImplemented, false)
	mapping(tenantA553, c.frameworkA, c.riskA, c.reqImplemented, false)    // live duplicate of the pair
	mapping(tenantA553, c.frameworkA, c.riskA, nil, false)                 // framework-only
	mapping(tenantA553, c.frameworkA, c.riskA, c.reqNotApplicable, true)   // soft-deleted
	mapping(tenantA553, c.frameworkA, c.riskA, c.reqB, false)              // tenant B's control: unplaceable
	mapping(tenantA553, c.frameworkA, uuid.New(), c.reqImplemented, false) // no such risk: unplaceable
	mapping(tenantB553, c.frameworkB, c.riskB, c.reqB, false)
}

// legacyMD5 fingerprints a whole table, rows in id order.
func (c *chain553) legacyMD5(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range legacyTables553 {
		var sum string
		require.NoError(t, c.db.Raw(fmt.Sprintf(`SELECT COALESCE(md5(string_agg(x::text, '|' ORDER BY x.id)), '') FROM %s x`, table)).Scan(&sum).Error)
		out[table] = sum
		require.NotEmpty(t, sum, "%s must be populated for this test to mean anything", table)
	}
	return out
}

// scores553 fingerprints what the Score Engine, SmartScore and ADR 0003's
// residual read or store for every risk.
type scores553 struct {
	Columns string
	Credits map[uuid.UUID][]scoring.ControlCredit
	Resid   map[uuid.UUID]scoring.Residual
}

func (c *chain553) scores(t *testing.T) scores553 {
	t.Helper()
	var s scores553
	require.NoError(t, c.db.Raw(`SELECT md5(string_agg(concat_ws('|', id, tenant_id, score, smart_score, smart_level, residual_risk, criticality), ';' ORDER BY id)) FROM risks`).Scan(&s.Columns).Error)
	s.Credits = map[uuid.UUID][]scoring.ControlCredit{}
	s.Resid = map[uuid.UUID]scoring.Residual{}
	posture := repository.NewGormPostureRepository(c.db)
	// One instant for every snapshot, so before and after read the same validity window (#419).
	asOf := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, r := range []struct {
		tenant, risk uuid.UUID
		score        float64
	}{{tenantA553, c.riskA, 6.4}, {tenantB553, c.riskB, 3.0}} {
		credits, err := posture.ControlCreditsByRisk(context.Background(), r.tenant, []uuid.UUID{r.risk}, asOf)
		require.NoError(t, err)
		s.Credits[r.risk] = credits[r.risk]
		s.Resid[r.risk] = scoring.ComputeResidual(r.score, credits[r.risk])
	}
	require.NotEmpty(t, s.Credits[c.riskA], "the residual must have a control to credit, or this test proves nothing")
	return s
}

func (c *chain553) up(t *testing.T) {
	t.Helper()
	require.NoError(t, newMigrator(t, c.dsn).Migrate(86))
	for _, table := range controlchain.Tables() {
		require.True(t, c.hasTable(t, table), table)
	}
}

// TestMigration0086_Success — the migration builds the six tables, Reconcile
// fills them, the counts reconcile with count(*), and no score moves.
func TestMigration0086_Success(t *testing.T) {
	c := pre0086DB(t)
	md5Before := c.legacyMD5(t)
	scoresBefore := c.scores(t)

	c.up(t)
	v, dirty := c.version(t)
	assert.Equal(t, uint(86), v)
	assert.False(t, dirty)

	// The SQL-built tables are the ones the row types describe: AutoMigrate over
	// them (what the next boot does) neither fails nor adds an index or a check.
	shape := func() [2]int64 {
		return [2]int64{
			c.count(t, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND tablename IN ?`, controlchain.Tables()),
			c.count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace = current_schema()::regnamespace AND conrelid::regclass::text IN ?`, controlchain.Tables()),
		}
	}
	shapeBefore := shape()
	require.NoError(t, c.db.AutoMigrate(controlchain.Models()...))
	assert.Equal(t, shapeBefore, shape(), "AutoMigrate over the SQL-built tables changes nothing")

	res, err := controlchain.Reconcile(context.Background(), c.db)
	require.NoError(t, err)

	assert.Equal(t, c.count(t, `SELECT count(*) FROM compliance_controls`), res.Requirements)
	assert.Equal(t, int64(7), res.Requirements)
	assert.Equal(t, 7, res.ControlsCreated)
	assert.Equal(t, int64(7), c.count(t, `SELECT count(*) FROM controls`), "one control per compliance_controls row, deleted included")
	assert.Equal(t, 2, res.ApplicabilityFlagged)
	assert.Equal(t, int64(2), c.count(t, `SELECT count(*) FROM controls WHERE applicability = 'not_applicable' AND applicability_review_pending
		AND applicability_justification IS NULL AND declared_status = 'not_implemented'`))
	assert.Equal(t, int64(1), c.count(t, `SELECT count(*) FROM controls WHERE id = ? AND tenant_id = ? AND deleted_at IS NOT NULL`,
		canonical.ControlIDFor(c.reqSoftDeleted), tenantA553), "a soft-deleted requirement has a soft-deleted control")
	assert.Equal(t, int64(1), c.count(t, `SELECT count(*) FROM controls WHERE id = ? AND tenant_id = ? AND declared_status = 'implemented'
		AND source = 'openrisk_user' AND source_id = ? AND confidence = 0.70 AND provenance->>'method' = 'migration'`,
		canonical.ControlIDFor(c.reqImplemented), tenantA553, canonical.ControlIDFor(c.reqImplemented).String()))
	assert.Equal(t, int64(7), c.count(t, `SELECT count(*) FROM canonical_edges WHERE verb = 'addressed_by' AND from_type = 'Requirement' AND to_type = 'Control'`))

	assert.Equal(t, c.count(t, `SELECT count(*) FROM evidence_control_links`), res.Links)
	assert.Equal(t, int64(5), res.Links)
	assert.Equal(t, 3, res.LinksCarried)
	assert.Len(t, res.LinksUnplaceable, 2)
	assert.Equal(t, int64(3), c.count(t, `SELECT count(*) FROM canonical_edges WHERE verb = 'evidenced_by' AND from_type = 'Control' AND to_type = 'Evidence'`))

	assert.Equal(t, c.count(t, `SELECT count(*) FROM risk_control_mappings`), res.Mappings)
	assert.Equal(t, int64(7), res.Mappings)
	assert.Equal(t, 2, res.MappingsCarried)
	assert.Len(t, res.MappingsCollapsed, 1)
	assert.Equal(t, 1, res.MappingsFrameworkOnly)
	assert.Equal(t, 1, res.MappingsSoftDeleted)
	assert.Len(t, res.MappingsUnplaceable, 2)
	assert.Equal(t, int64(2), c.count(t, `SELECT count(*) FROM canonical_edges WHERE verb = 'mitigated_by' AND from_type = 'Risk' AND to_type = 'Control'`))

	// No edge crosses a tenant: every end sits in the edge's tenant.
	assert.Equal(t, int64(0), c.count(t, `SELECT count(*) FROM canonical_edges e JOIN controls k ON k.id = e.to_id AND e.to_type = 'Control' WHERE k.tenant_id <> e.tenant_id`))
	assert.Equal(t, int64(0), c.count(t, `SELECT count(*) FROM canonical_edges e JOIN evidences v ON v.id = e.to_id AND e.to_type = 'Evidence' WHERE v.tenant_id <> e.tenant_id`))
	assert.Equal(t, int64(0), c.count(t, `SELECT count(*) FROM canonical_edges e JOIN risks r ON r.id = e.from_id AND e.from_type = 'Risk' WHERE r.tenant_id <> e.tenant_id`))

	// Nothing legacy moved, and no score with it.
	assert.Equal(t, md5Before, c.legacyMD5(t), "the legacy tables are byte-identical")
	assert.Equal(t, scoresBefore, c.scores(t), "no Score, SmartScore, residual_risk or computed residual changed")

	// A second run inserts nothing.
	again, err := controlchain.Reconcile(context.Background(), c.db)
	require.NoError(t, err)
	assert.Equal(t, 0, again.Created())
	assert.Equal(t, int64(7), c.count(t, `SELECT count(*) FROM controls`))
	assert.Equal(t, int64(12), c.count(t, `SELECT count(*) FROM canonical_edges`))
	assert.Equal(t, md5Before, c.legacyMD5(t))
}

// TestMigration0086_ConcurrentReconcile — replicas booting together: the
// advisory lock makes one of them do the work and the others find it done.
func TestMigration0086_ConcurrentReconcile(t *testing.T) {
	c := pre0086DB(t)
	c.up(t)

	const replicas = 4
	var wg sync.WaitGroup
	results := make([]controlchain.Result, replicas)
	errs := make([]error, replicas)
	start := make(chan struct{})
	for i := 0; i < replicas; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = controlchain.Reconcile(context.Background(), c.db)
		}(i)
	}
	close(start)
	wg.Wait()

	workers := 0
	for i := range results {
		require.NoError(t, errs[i])
		if results[i].Created() > 0 {
			workers++
		}
	}
	assert.Equal(t, 1, workers, "exactly one Reconcile inserted")
	assert.Equal(t, int64(7), c.count(t, `SELECT count(*) FROM controls`))
	assert.Equal(t, int64(12), c.count(t, `SELECT count(*) FROM canonical_edges`))
	for _, r := range results {
		assert.Equal(t, int64(7), r.Requirements)
		assert.Equal(t, 3, r.LinksCarried)
		assert.Equal(t, 2, r.MappingsCarried)
	}
}

// TestMigration0086_Rollback — down removes the six tables and nothing else;
// the legacy tables are byte-identical to before 0086 ever ran, and the next
// boot rebuilds the chain.
func TestMigration0086_Rollback(t *testing.T) {
	c := pre0086DB(t)
	md5Before := c.legacyMD5(t)
	scoresBefore := c.scores(t)
	tables := func() int64 {
		return c.count(t, `SELECT count(*) FROM pg_class cl JOIN pg_namespace n ON n.oid = cl.relnamespace
			WHERE n.nspname = current_schema() AND cl.relkind = 'r'`)
	}
	tablesBefore := tables()

	c.up(t)
	_, err := controlchain.Reconcile(context.Background(), c.db)
	require.NoError(t, err)
	require.Equal(t, int64(7), c.count(t, `SELECT count(*) FROM controls`))

	require.NoError(t, newMigrator(t, c.dsn).Steps(-1))
	for _, table := range controlchain.Tables() {
		assert.False(t, c.hasTable(t, table), "%s is gone after down", table)
	}
	v, dirty := c.version(t)
	assert.Equal(t, uint(85), v)
	assert.False(t, dirty)
	assert.Equal(t, tablesBefore, tables(), "down touched no other table")
	assert.Equal(t, md5Before, c.legacyMD5(t), "compliance_controls, evidence_control_links and risk_control_mappings are byte-identical")
	assert.Equal(t, scoresBefore, c.scores(t))

	// Reversible: up again, and the next boot's Reconcile rebuilds the same chain.
	c.up(t)
	res, err := controlchain.Reconcile(context.Background(), c.db)
	require.NoError(t, err)
	assert.Equal(t, 7, res.ControlsCreated)
	assert.Equal(t, int64(12), c.count(t, `SELECT count(*) FROM canonical_edges`))
}

// TestMigration0086_AutoMigrateOnly — a compose boot never runs the SQL layer:
// AutoMigrate alone must build tables that refuse what 0086's refuse.
func TestMigration0086_AutoMigrateOnly(t *testing.T) {
	c := pre0086DB(t)
	require.NoError(t, c.db.AutoMigrate(controlchain.Models()...))
	res, err := controlchain.Reconcile(context.Background(), c.db)
	require.NoError(t, err)
	assert.Equal(t, 7, res.ControlsCreated)

	err = c.db.Exec(`INSERT INTO controls (id, tenant_id, source, source_id, confidence, provenance, name, applicability, declared_status)
		VALUES (?, ?, 'openrisk_user', 'x', 0.7, '{}', 'n', 'not_applicable', 'implemented')`, uuid.New(), tenantA553).Error
	require.Error(t, err, "not_applicable without a justification or the pending flag is refused by the AutoMigrate-built table too")
	err = c.db.Exec(`INSERT INTO canonical_edges (id, tenant_id, from_type, from_id, verb, to_type, to_id, source, source_id, confidence, provenance)
		VALUES (?, ?, 'Control', ?, 'implemented_by', 'ControlImplementation', ?, 'openrisk_user', 'x', 0.7, '{}')`, uuid.New(), tenantA553, uuid.New(), uuid.New()).Error
	require.Error(t, err, "implemented_by is a field, never an edge row")
}
