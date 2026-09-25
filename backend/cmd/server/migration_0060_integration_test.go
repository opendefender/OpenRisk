// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #349 — migration 0060 against a populated database, upgraded the way a
// server boot upgrades it: PrepareForAutoMigrate, AutoMigrate(schemaModels()),
// then the SQL layer. Until now 0060 had only run on empty databases.
//
// The starting point is the previous release's shape: SQL layer at version 59,
// no mfa_policies table, no organization_members.mfa_grace_started_at, and
// memberships of every age and every legacy shape. It is built from today's
// models minus what 0060 owns, which is exact for the two objects 0060 touches.
// The same upgrade was also rehearsed with the real v1.1.0-rc.3 binary; see
// docs/runbooks/migration-0060.md.
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).
// OPENRISK_MIGRATION_TEST_ROWS raises the volume for a local timing run.

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/migrations"
)

const (
	// migrationsDir is the repository-root migrations/ seen from this package.
	migrationsDir = "../../../migrations"
	// lastVersionBefore0060 is where the previous release left the SQL layer.
	lastVersionBefore0060 = 59
	defaultUpgradeRows    = 20000
)

// Memberships whose outcome each test checks by name. Everything else is bulk.
var (
	veteranAdminID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	freshAdminID   = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	legacyRootID   = uuid.MustParse("00000000-0000-4000-8000-000000000003")
	invitedEarlyID = uuid.MustParse("00000000-0000-4000-8000-000000000004")
	noAnchorID     = uuid.MustParse("00000000-0000-4000-8000-000000000005")
)

type upgradeDB struct {
	db  *gorm.DB
	dsn string // DATABASE_URL pinned to the throwaway schema
}

// sqlState reads the PostgreSQL error code without importing the driver's
// error type.
func sqlState(err error) string {
	var coded interface{ SQLState() string }
	if errors.As(err, &coded) {
		return coded.SQLState()
	}
	return ""
}

func upgradeRows(t *testing.T) int {
	t.Helper()
	raw := os.Getenv("OPENRISK_MIGRATION_TEST_ROWS")
	if raw == "" {
		return defaultUpgradeRows
	}
	n, err := strconv.Atoi(raw)
	require.NoError(t, err, "OPENRISK_MIGRATION_TEST_ROWS")
	return n
}

// withParams returns dsn with extra query parameters. Both drivers in play
// (pgx for GORM, lib/pq for golang-migrate) send unknown parameters to the
// server as run-time settings, which is how search_path and lock_timeout get
// pinned to every pooled connection.
func withParams(t *testing.T, dsn string, params map[string]string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	require.NoError(t, err, "DATABASE_URL must be a postgres:// URL")
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func openGorm(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	// The options database.Connect uses.
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		TranslateError:                           true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func newMigrator(t *testing.T, dsn string) *migrate.Migrate {
	t.Helper()
	m, err := migrate.New("file://"+migrationsDir, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	return m
}

// previousReleaseDB builds the pre-0060 schema in a throwaway PostgreSQL schema
// and fills organization_members with `rows` bulk memberships plus the named
// ones. It snapshots the table so the tests can prove what 0060 left alone.
func previousReleaseDB(t *testing.T, rows int) *upgradeDB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "mig0060_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]

	admin := openGorm(t, base)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() { _ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error })

	params := map[string]string{"search_path": schema + ",public"}
	// pgx falls back to a plain connection when sslmode is unset; lib/pq, which
	// golang-migrate uses, insists on TLS. CI's DATABASE_URL leaves it unset.
	if parsed, err := url.Parse(base); err == nil && parsed.Query().Get("sslmode") == "" {
		params["sslmode"] = "disable"
	}
	u := &upgradeDB{dsn: withParams(t, base, params)}
	u.db = openGorm(t, u.dsn)

	require.NoError(t, database.PrepareForAutoMigrate(u.db))
	require.NoError(t, u.db.AutoMigrate(schemaModels()...))
	require.NoError(t, newMigrator(t, u.dsn).Migrate(lastVersionBefore0060))

	// Remove what 0060 owns: the previous release had neither.
	require.NoError(t, u.db.Exec(`DROP TABLE mfa_policies`).Error)
	require.NoError(t, u.db.Exec(`ALTER TABLE organization_members DROP COLUMN mfa_grace_started_at`).Error)

	// Bulk memberships spread over 18 months, in every shape a long-lived
	// database accumulates: rows written by the ORM (joined_at = created_at),
	// rows written by hand or by old seeders (joined_at or created_at NULL),
	// invitations accepted after the row existed, deactivated and revoked
	// members, rows with no lifecycle status at all.
	require.NoError(t, u.db.Exec(`
		INSERT INTO organization_members
		       (id, organization_id, user_id, role, business_role, is_active, status,
		        joined_at, created_at, updated_at)
		SELECT gen_random_uuid(),
		       ('00000000-0000-4000-9000-' || lpad((g % 40)::text, 12, '0'))::uuid,
		       gen_random_uuid(),
		       (ARRAY['root','admin','user','user','user'])[1 + g % 5],
		       CASE WHEN g % 5 >= 2 THEN (ARRAY['rssi','risk_manager','auditor',''])[1 + g % 4] ELSE '' END,
		       g % 17 <> 0,
		       CASE WHEN g % 17 = 0 THEN 'deactivated' WHEN g % 29 = 0 THEN 'revoked' WHEN g % 23 = 0 THEN '' ELSE 'active' END,
		       CASE g % 10 WHEN 6 THEN NULL WHEN 8 THEN NULL WHEN 9 THEN b.at + interval '2 days' ELSE b.at END,
		       CASE g % 10 WHEN 7 THEN NULL WHEN 8 THEN NULL ELSE b.at END,
		       b.at + interval '1 hour'
		  FROM generate_series(1, ?) AS g,
		       LATERAL (SELECT now() - make_interval(days => g % 540, secs => g % 86400) AS at) AS b`,
		rows).Error)

	now := time.Now().UTC()
	named := []struct {
		id                  uuid.UUID
		role                string
		joinedAt, createdAt *time.Time
	}{
		{veteranAdminID, "admin", ptr(now.AddDate(0, -6, 0)), ptr(now.AddDate(0, -6, 0))},
		{freshAdminID, "admin", ptr(now.Add(-time.Hour)), ptr(now.Add(-time.Hour))},
		{legacyRootID, "root", nil, ptr(now.AddDate(0, 0, -3))},
		{invitedEarlyID, "admin", ptr(now.AddDate(0, 0, -2)), ptr(now.AddDate(0, 0, -10))},
		{noAnchorID, "admin", nil, nil},
	}
	for _, n := range named {
		require.NoError(t, u.db.Exec(`
			INSERT INTO organization_members
			       (id, organization_id, user_id, role, is_active, status, joined_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, true, 'active', ?, ?, now())`,
			n.id, uuid.New(), uuid.New(), n.role, n.joinedAt, n.createdAt).Error)
	}

	// The previous release already ran PrepareForAutoMigrate on every boot, so
	// a database it served has no unset status and no duplicate membership.
	// Settle that here, so the snapshot isolates what the upgrade itself does.
	require.NoError(t, database.PrepareForAutoMigrate(u.db))
	require.NoError(t, u.db.Exec(`CREATE TABLE pre_0060_members AS SELECT * FROM organization_members`).Error)
	return u
}

func ptr[T any](v T) *T { return &v }

// bootUpgrade runs what a server boot of the new release runs, in its order,
// and returns the time the SQL layer took.
func (u *upgradeDB) bootUpgrade(t *testing.T) (time.Duration, error) {
	t.Helper()
	require.NoError(t, database.PrepareForAutoMigrate(u.db))
	require.NoError(t, u.db.AutoMigrate(schemaModels()...))
	return u.runSQLLayer(t, u.dsn)
}

// runSQLLayer calls the real migrations.RunMigrations against dsn.
func (u *upgradeDB) runSQLLayer(t *testing.T, dsn string) (time.Duration, error) {
	t.Helper()
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("MIGRATIONS_DIR", migrationsDir)
	start := time.Now()
	err := migrations.RunMigrations()
	return time.Since(start), err
}

func (u *upgradeDB) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, u.db.Raw(query, args...).Scan(&n).Error, query)
	return n
}

func (u *upgradeDB) member(t *testing.T, id uuid.UUID) domain.OrganizationMember {
	t.Helper()
	var m domain.OrganizationMember
	require.NoError(t, u.db.First(&m, "id = ?", id).Error)
	return m
}

func (u *upgradeDB) version(t *testing.T) (uint, bool) {
	t.Helper()
	v, dirty, err := newMigrator(t, u.dsn).Version()
	require.NoError(t, err)
	return v, dirty
}

// decide is DecideMFA for a privileged member under the shipped 7-day policy.
func decide(m domain.OrganizationMember, now time.Time) domain.MFADecision {
	return domain.DecideMFA(domain.MFADecisionInput{
		Privileged:     true,
		GraceStartedAt: m.MFAGraceAnchor(),
		GraceDays:      domain.MFAGraceDaysDefault,
		Now:            now,
	})
}

// assertBackfillInvariants is the post-migration contract of 0060 on the rows
// that existed before it ran. The runbook's post-checks are the same queries.
func (u *upgradeDB) assertBackfillInvariants(t *testing.T) {
	t.Helper()
	assert.Equal(t,
		u.count(t, `SELECT count(*) FROM pre_0060_members`),
		u.count(t, `SELECT count(*) FROM organization_members m JOIN pre_0060_members USING (id)`),
		"no membership may be lost")
	assert.Zero(t, u.count(t, `
		SELECT count(*) FROM organization_members m JOIN pre_0060_members USING (id)
		 WHERE m.mfa_grace_started_at IS DISTINCT FROM COALESCE(m.joined_at, m.created_at)`),
		"every pre-existing membership is anchored to when it began")
	assert.Zero(t, u.count(t, `
		SELECT count(*) FROM organization_members m JOIN pre_0060_members s USING (id)
		 WHERE (to_jsonb(m) - 'mfa_grace_started_at') IS DISTINCT FROM to_jsonb(s)`),
		"0060 writes the anchor and nothing else, updated_at included")
	assert.Equal(t,
		u.count(t, `SELECT count(*) FROM pre_0060_members WHERE joined_at IS NULL AND created_at IS NULL`),
		u.count(t, `SELECT count(*) FROM organization_members m JOIN pre_0060_members USING (id) WHERE m.mfa_grace_started_at IS NULL`),
		"an anchor stays NULL only where the row has no start date at all")
}

// assertMFAPolicySchema checks what 0060 promises about mfa_policies, whoever
// created the table.
func (u *upgradeDB) assertMFAPolicySchema(t *testing.T) {
	t.Helper()
	assert.Equal(t, int64(1), u.count(t, `
		SELECT count(*) FROM pg_constraint
		 WHERE conrelid = 'mfa_policies'::regclass AND contype = 'c'
		   AND conname = 'mfa_policies_grace_days_bounds' AND convalidated`),
		"grace_days must be bounded by the database, not only by the handler")

	var graceDefault, idDefault *string
	require.NoError(t, u.db.Raw(`
		SELECT column_default FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'mfa_policies' AND column_name = 'grace_days'`).
		Scan(&graceDefault).Error)
	require.NotNil(t, graceDefault, "grace_days needs a database default")
	assert.Equal(t, "7", *graceDefault)
	require.NoError(t, u.db.Raw(`
		SELECT column_default FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'mfa_policies' AND column_name = 'id'`).
		Scan(&idDefault).Error)
	require.NotNil(t, idDefault, "id needs a database default")
	assert.Equal(t, "gen_random_uuid()", *idDefault)

	assert.GreaterOrEqual(t, u.count(t, `
		SELECT count(*) FROM pg_index i
		  JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = i.indkey[0]
		 WHERE i.indrelid = 'mfa_policies'::regclass AND i.indisunique AND i.indnatts = 1
		   AND a.attname = 'tenant_id'`), int64(1),
		"one policy per tenant must be enforced by a unique index")

	var colType string
	var nullable string
	require.NoError(t, u.db.Raw(`
		SELECT data_type, is_nullable FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'organization_members'
		   AND column_name = 'mfa_grace_started_at'`).Row().Scan(&colType, &nullable))
	assert.Equal(t, "timestamp with time zone", colType)
	assert.Equal(t, "YES", nullable, "memberships written by the previous release during a rollout carry NULL")
}

func TestMigration0060_Success(t *testing.T) {
	rows := upgradeRows(t)
	u := previousReleaseDB(t, rows)

	took, err := u.bootUpgrade(t)
	require.NoError(t, err)
	t.Logf("SQL layer 0060→head on %d memberships: %s", rows+5, took)
	assert.Less(t, took, 2*time.Minute, "a boot must not spend minutes in the SQL layer at this volume")

	v, dirty := u.version(t)
	assert.False(t, dirty)
	assert.GreaterOrEqual(t, v, uint(65))

	u.assertBackfillInvariants(t)
	u.assertMFAPolicySchema(t)

	now := time.Now()

	veteran := u.member(t, veteranAdminID)
	require.NotNil(t, veteran.MFAGraceStartedAt)
	assert.WithinDuration(t, veteran.JoinedAt, *veteran.MFAGraceStartedAt, time.Microsecond)
	d := decide(veteran, now)
	assert.True(t, d.Required, "a six-month-old administrator gets no fresh week from the upgrade")
	assert.Equal(t, domain.MFAStateRequired, d.State)

	freshMember := u.member(t, freshAdminID)
	fresh := decide(freshMember, now)
	assert.True(t, fresh.GraceActive, "an administrator who joined an hour ago keeps their window")
	require.NotNil(t, fresh.Deadline)
	assert.WithinDuration(t, freshMember.JoinedAt.Add(7*24*time.Hour), *fresh.Deadline, 0,
		"the window runs from when they joined, not from the upgrade")

	legacy := u.member(t, legacyRootID)
	require.NotNil(t, legacy.MFAGraceStartedAt)
	assert.WithinDuration(t, legacy.CreatedAt, *legacy.MFAGraceStartedAt, time.Microsecond,
		"no joined_at: the anchor falls back to created_at")
	assert.True(t, decide(legacy, now).GraceActive)

	invited := u.member(t, invitedEarlyID)
	require.NotNil(t, invited.MFAGraceStartedAt)
	assert.WithinDuration(t, invited.JoinedAt, *invited.MFAGraceStartedAt, time.Microsecond,
		"joined_at wins over created_at: the requirement starts when the membership did")

	// A membership created after the upgrade, the way the application creates
	// one: nothing writes the anchor, so it reads from joined_at.
	created := domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: uuid.New(), UserID: uuid.New(),
		Role: domain.RoleAdmin, IsActive: true, Status: domain.MembershipActive,
	}
	require.NoError(t, u.db.Create(&created).Error)
	reloaded := u.member(t, created.ID)
	assert.Nil(t, reloaded.MFAGraceStartedAt)
	assert.WithinDuration(t, reloaded.JoinedAt, reloaded.MFAGraceAnchor(), 0)
	assert.True(t, decide(reloaded, time.Now()).GraceActive, "a new administrator gets the full window")

	// A membership written during a rolling deploy by a replica still on the
	// previous release, which does not know the column: same outcome.
	lateID := uuid.New()
	require.NoError(t, u.db.Exec(`
		INSERT INTO organization_members (id, organization_id, user_id, role, is_active, status, joined_at, created_at, updated_at)
		VALUES (?, ?, ?, 'admin', true, 'active', now(), now(), now())`, lateID, uuid.New(), uuid.New()).Error)
	late := u.member(t, lateID)
	assert.Nil(t, late.MFAGraceStartedAt)
	assert.True(t, decide(late, time.Now()).GraceActive)
}

// A membership with neither joined_at nor created_at has no start date to
// anchor to. 0060 leaves it NULL and the decision fails closed.
func TestMigration0060_NotFound(t *testing.T) {
	u := previousReleaseDB(t, 200)
	_, err := u.bootUpgrade(t)
	require.NoError(t, err)

	m := u.member(t, noAnchorID)
	assert.Nil(t, m.MFAGraceStartedAt)
	assert.True(t, m.MFAGraceAnchor().IsZero())
	d := decide(m, time.Now())
	assert.True(t, d.Required, "an unknown anchor must never read as an open-ended grace period")
	assert.Nil(t, d.Deadline)
}

// A direct SQL write must not be able to switch the MFA requirement off, on an
// upgraded database as on a fresh one. Before 0065 the bound only existed when
// 0060's CREATE TABLE ran, which a server boot never lets it do: AutoMigrate
// creates mfa_policies first.
func TestMigration0060_Unauthorized(t *testing.T) {
	u := previousReleaseDB(t, 200)
	_, err := u.bootUpgrade(t)
	require.NoError(t, err)

	for _, days := range []int{-1, 91, 36500} {
		err := u.db.Exec(`INSERT INTO mfa_policies (tenant_id, grace_days) VALUES (?, ?)`, uuid.New(), days).Error
		assert.ErrorIs(t, err, gorm.ErrCheckConstraintViolated, "grace_days=%d", days)
	}

	tenant := uuid.New()
	require.NoError(t, u.db.Exec(`INSERT INTO mfa_policies (tenant_id) VALUES (?)`, tenant).Error)
	var stored domain.MFAPolicy
	require.NoError(t, u.db.First(&stored, "tenant_id = ?", tenant).Error)
	assert.Equal(t, domain.MFAGraceDaysDefault, stored.GraceDays, "a raw insert gets the shipped default")
	assert.NotEqual(t, uuid.Nil, stored.ID)

	err = u.db.Exec(`INSERT INTO mfa_policies (tenant_id, grace_days) VALUES (?, 3)`, tenant).Error
	assert.ErrorIs(t, err, gorm.ErrDuplicatedKey, "a second policy for the same tenant must be refused")
}

// Running 0060 again, as a re-upgrade after an application rollback does,
// changes nothing that is already right and only fills anchors still missing.
func TestMigration0060_Idempotent(t *testing.T) {
	u := previousReleaseDB(t, 2000)
	_, err := u.bootUpgrade(t)
	require.NoError(t, err)

	raw, err := os.ReadFile(migrationsDir + "/0060_mfa_deferred_enrollment.up.sql")
	require.NoError(t, err)

	// A promotion re-anchored one member; a previous-release replica wrote
	// another without an anchor.
	promotedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, u.db.Exec(`UPDATE organization_members SET mfa_grace_started_at = ? WHERE id = ?`, promotedAt, veteranAdminID).Error)
	lateID := uuid.New()
	require.NoError(t, u.db.Exec(`
		INSERT INTO organization_members (id, organization_id, user_id, role, is_active, status, joined_at, created_at, updated_at)
		VALUES (?, ?, ?, 'admin', true, 'active', now() - interval '1 day', now() - interval '1 day', now())`,
		lateID, uuid.New(), uuid.New()).Error)

	require.NoError(t, u.db.Exec(`CREATE TABLE before_rerun AS SELECT * FROM organization_members WHERE id <> ?`, lateID).Error)
	require.NoError(t, u.db.Exec(string(raw)).Error)

	assert.Zero(t, u.count(t, `
		SELECT count(*) FROM organization_members m JOIN before_rerun b USING (id)
		 WHERE to_jsonb(m) IS DISTINCT FROM to_jsonb(b)`),
		"a second run rewrites no anchor, not even one a promotion reset")
	late := u.member(t, lateID)
	require.NotNil(t, late.MFAGraceStartedAt)
	assert.WithinDuration(t, late.JoinedAt, *late.MFAGraceStartedAt, time.Microsecond)
	u.assertMFAPolicySchema(t)
}

// 0060 runs in one transaction. A failure half-way through the backfill must
// leave no anchor written, a DIRTY version the boot refuses to serve on, and a
// documented way back.
func TestMigration0060_InterruptedBackfill(t *testing.T) {
	rows := 4000
	u := previousReleaseDB(t, rows)
	require.NoError(t, database.PrepareForAutoMigrate(u.db))
	require.NoError(t, u.db.AutoMigrate(schemaModels()...))

	// Fail on the (rows/2)th row the UPDATE touches. The sequence is not
	// transactional, so it still records how far the backfill got.
	require.NoError(t, u.db.Exec(`CREATE SEQUENCE backfill_progress`).Error)
	require.NoError(t, u.db.Exec(fmt.Sprintf(`
		CREATE FUNCTION interrupt_backfill() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF nextval('backfill_progress') > %d THEN
				RAISE EXCEPTION 'simulated interruption half-way through the backfill';
			END IF;
			RETURN NEW;
		END $$`, rows/2)).Error)
	require.NoError(t, u.db.Exec(`
		CREATE TRIGGER interrupt_backfill BEFORE UPDATE ON organization_members
		FOR EACH ROW EXECUTE FUNCTION interrupt_backfill()`).Error)

	_, err := u.runSQLLayer(t, u.dsn)
	require.Error(t, err)
	assert.Greater(t, u.count(t, `SELECT last_value FROM backfill_progress`), int64(rows/2),
		"the backfill really was half-way when it failed")
	assert.Zero(t, u.count(t, `SELECT count(*) FROM organization_members WHERE mfa_grace_started_at IS NOT NULL`),
		"the rows updated before the failure are rolled back with it")
	v, dirty := u.version(t)
	assert.Equal(t, uint(60), v)
	assert.True(t, dirty)

	// The next boot refuses to serve and says how to recover.
	_, err = u.runSQLLayer(t, u.dsn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DIRTY at version 60")

	// Recovery, as the runbook describes it: remove the cause, force the last
	// good version, boot again.
	require.NoError(t, u.db.Exec(`DROP TRIGGER interrupt_backfill ON organization_members`).Error)
	require.NoError(t, newMigrator(t, u.dsn).Force(lastVersionBefore0060))
	_, err = u.bootUpgrade(t)
	require.NoError(t, err)
	u.assertBackfillInvariants(t)
	u.assertMFAPolicySchema(t)
}

// A long transaction on organization_members (a bulk import, a stuck session)
// must make the migration fail fast when the operator sets lock_timeout, and
// that failure is recoverable the same way.
func TestMigration0060_LockTimeout(t *testing.T) {
	u := previousReleaseDB(t, 500)
	require.NoError(t, database.PrepareForAutoMigrate(u.db))
	require.NoError(t, u.db.AutoMigrate(schemaModels()...))

	holder := u.db.Begin()
	require.NoError(t, holder.Error)
	require.NoError(t, holder.Exec(`LOCK TABLE organization_members IN ROW EXCLUSIVE MODE`).Error)

	start := time.Now()
	_, err := u.runSQLLayer(t, withParams(t, u.dsn, map[string]string{"lock_timeout": "500ms"}))
	waited := time.Since(start)
	require.NoError(t, holder.Rollback().Error)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "lock timeout")
	assert.Less(t, waited, 30*time.Second, "lock_timeout bounds how long 0060 queues behind other work")
	assert.Zero(t, u.count(t, `SELECT count(*) FROM organization_members WHERE mfa_grace_started_at IS NOT NULL`))
	v, dirty := u.version(t)
	assert.Equal(t, uint(60), v)
	assert.True(t, dirty)

	require.NoError(t, newMigrator(t, u.dsn).Force(lastVersionBefore0060))
	_, err = u.bootUpgrade(t)
	require.NoError(t, err)
	u.assertBackfillInvariants(t)
}

// 0060's ALTER TABLE and its UPDATE share a transaction, so the ACCESS
// EXCLUSIVE lock the ALTER takes is held until the backfill commits: every
// read of organization_members, login included, waits for the whole backfill.
// The runbook sizes the maintenance window from this.
func TestMigration0060_LockImpact(t *testing.T) {
	u := previousReleaseDB(t, 500)
	require.NoError(t, database.PrepareForAutoMigrate(u.db))
	require.NoError(t, u.db.AutoMigrate(schemaModels()...))

	raw, err := os.ReadFile(migrationsDir + "/0060_mfa_deferred_enrollment.up.sql")
	require.NoError(t, err)
	body := strings.NewReplacer("BEGIN;", "", "COMMIT;", "").Replace(string(raw))

	tx := u.db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec(body).Error)

	var pid int
	require.NoError(t, tx.Raw(`SELECT pg_backend_pid()`).Scan(&pid).Error)
	var modes []string
	require.NoError(t, u.db.Raw(`
		SELECT mode FROM pg_locks
		 WHERE pid = ? AND granted AND relation = 'organization_members'::regclass`, pid).Scan(&modes).Error)
	assert.Contains(t, modes, "AccessExclusiveLock")

	err = u.db.Transaction(func(reader *gorm.DB) error {
		if err := reader.Exec(`SET LOCAL lock_timeout = '200ms'`).Error; err != nil {
			return err
		}
		var n int64
		return reader.Raw(`SELECT count(*) FROM organization_members`).Scan(&n).Error
	})
	require.Error(t, err, "a plain read must wait while 0060 is in flight")
	assert.Equal(t, "55P03", sqlState(err))
}

// Rolling the schema back with the down migrations loses what 0060 stored and
// cannot be re-derived: tenant policies and promotion anchors. The runbook
// therefore rolls the application back and keeps the schema.
func TestMigration0060_Rollback(t *testing.T) {
	u := previousReleaseDB(t, 1000)
	_, err := u.bootUpgrade(t)
	require.NoError(t, err)

	tenant := uuid.New()
	require.NoError(t, u.db.Exec(`INSERT INTO mfa_policies (tenant_id, grace_days) VALUES (?, 30)`, tenant).Error)
	promotedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, u.db.Exec(`UPDATE organization_members SET mfa_grace_started_at = ? WHERE id = ?`, promotedAt, veteranAdminID).Error)

	require.NoError(t, newMigrator(t, u.dsn).Migrate(lastVersionBefore0060))
	assert.False(t, u.db.Migrator().HasTable("mfa_policies"))
	assert.False(t, u.db.Migrator().HasColumn(&domain.OrganizationMember{}, "mfa_grace_started_at"))
	assert.Zero(t, u.count(t, `
		SELECT count(*) FROM organization_members m JOIN pre_0060_members s USING (id)
		 WHERE to_jsonb(m) IS DISTINCT FROM to_jsonb(s)`),
		"the down migrations touch nothing but what 0060 added")

	// Upgrading again rebuilds the schema and re-derives the anchors from the
	// membership start; the tenant policy and the promotion anchor are gone.
	_, err = u.bootUpgrade(t)
	require.NoError(t, err)
	u.assertBackfillInvariants(t)
	u.assertMFAPolicySchema(t)
	assert.Zero(t, u.count(t, `SELECT count(*) FROM mfa_policies WHERE tenant_id = ?`, tenant))
	veteran := u.member(t, veteranAdminID)
	require.NotNil(t, veteran.MFAGraceStartedAt)
	assert.True(t, veteran.MFAGraceStartedAt.Before(promotedAt.Add(-24*time.Hour)),
		"the promotion anchor is replaced by the membership start")
}
