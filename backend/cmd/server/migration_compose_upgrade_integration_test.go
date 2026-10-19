// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #1001 — the compose and self-host installs ran on AutoMigrate alone: their
// boot never set DATABASE_URL, so the SQL layer never ran there. Turning it on
// replays the whole chain on a database that already holds customer data, and
// several files were written for a database that had none:
//
//   - 0058 dropped and recreated invitations (every pending invitation lost);
//   - 0056 deleted every MFA backup code, real ones included;
//   - 0076 stamped every late mitigation as already reminded;
//   - 0049 wrote "disabled before suspension reasons were recorded" on rules
//     disabled long after that field existed.
//
// This replays that upgrade in a throwaway schema, through the real
// RunMigrations, and compares every table before and after.
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/migrations"
)

// The eight codes the pre-#240 generator gave every user, printed by running
// the original GenerateBackupCodes from ff6961ba^. Written out, not computed,
// so the test does not trust the code under test to rebuild them.
var universalBackupCodes = []string{
	"4F2LIBGHU5SD", "AZ67MVK3YRWX", "ENCTQJOP4F2L", "IBGHU5SDAZ67",
	"MVK3YRWXENCT", "QJOP4F2LIBGH", "U5SDAZ67MVK3", "YRWXENCTQJOP",
}

func TestComposeUpgrade_SQLLayerKeepsCustomerData(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin := openGorm(t, dsn)
	schema := "compose1001_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() { _ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error })

	scoped := withParams(t, dsn, map[string]string{"search_path": schema + ",public"})
	db := openGorm(t, scoped)

	// The compose install: boots without DATABASE_URL, so Prepare + AutoMigrate
	// and no schema_migrations table.
	require.NoError(t, database.PrepareForAutoMigrate(db))
	require.NoError(t, db.AutoMigrate(schemaModels()...))

	tenant := uuid.New()
	inviteID := uuid.New()
	lateMitigation := uuid.New()
	disabledRule := uuid.New()
	universalUser, oldUniversalUser, realUser := uuid.New(), uuid.New(), uuid.New()
	realCodes := []string{"K7QWERTY2345", "M3ZXCVBN6789"}

	// Customer rows, as the application would have left them. Foreign keys
	// are suspended for these inserts only, so the test does not have to build
	// a whole organization, user and risk to hold them.
	tx := db.Begin()
	require.NoError(t, tx.Exec(`SET LOCAL session_replication_role = replica`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO invitations
		(id, organization_id, email, role, token_hash, status, expires_at, invited_by_id, last_sent_at, send_count, created_at, updated_at)
		VALUES (?, ?, 'invitee@upgrade.example', 'user', ?, 'pending', ?, ?, now(), 1, now(), now())`,
		inviteID, tenant, strings.Repeat("a", 64), time.Now().Add(72*time.Hour), uuid.New()).Error)
	require.NoError(t, tx.Exec(`INSERT INTO mitigations (id, tenant_id, risk_id, title, created_by, due_date, created_at, updated_at)
		VALUES (?, ?, ?, 'Patch the VPN', ?, ?, now(), now())`,
		lateMitigation, tenant, uuid.New(), uuid.New(), time.Now().Add(-48*time.Hour)).Error)
	require.NoError(t, tx.Exec(`INSERT INTO automation_rules (id, tenant_id, name, enabled, trigger, suspended_reason, created_at, updated_at)
		VALUES (?, ?, 'Escalate criticals', false, 'risk_created', '', now(), now())`,
		disabledRule, tenant).Error)
	// A residual-risk acceptance made in the app after #811, so it already has
	// its end date: 0077's backfill must leave it alone (#1023).
	require.NoError(t, tx.Exec(`INSERT INTO risks (id, tenant_id, name, title, created_by, lifecycle_state, accepted_at, acceptance_expires_at, created_at, updated_at)
		VALUES (?, ?, 'Legacy VPN appliance', 'Legacy VPN appliance', ?, 'residual_accepted', ?, ?, now(), now())`,
		uuid.New(), tenant, uuid.New(), time.Now().Add(-30*24*time.Hour), time.Now().Add(335*24*time.Hour)).Error)
	codes := func(user uuid.UUID, plain []string, at time.Time) {
		for _, c := range plain {
			h, err := bcrypt.GenerateFromPassword([]byte(c), bcrypt.MinCost)
			require.NoError(t, err)
			require.NoError(t, tx.Exec(`INSERT INTO mfa_backup_codes (id, user_id, tenant_id, code_hash, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?)`, uuid.New(), user, tenant, string(h), at, at).Error)
		}
	}
	// Still holding the universal set, enrolled after the fix date by an image
	// that predates it: only the hash check can catch these.
	codes(universalUser, universalBackupCodes, time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC))
	// Enrolled before the fix existed: 0056 deletes these on the date alone.
	codes(oldUniversalUser, universalBackupCodes[:3], time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))
	// Real CSPRNG codes: they must survive.
	codes(realUser, realCodes, time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC))
	require.NoError(t, tx.Commit().Error)

	cols := digestColumns(t, db)
	before := tableDigests(t, db, cols)

	// Turning the SQL layer on: the whole chain, through the real entry point.
	t.Setenv("DATABASE_URL", scoped)
	t.Setenv("MIGRATIONS_DIR", migrationsDir)
	require.NoError(t, migrations.RunMigrations())

	var dirty bool
	require.NoError(t, db.Raw(`SELECT dirty FROM schema_migrations`).Scan(&dirty).Error)
	require.False(t, dirty)

	after := tableDigests(t, db, cols)

	// Every table keeps its rows, except the universal backup codes, and only
	// the purge alters existing rows.
	changed := map[string]bool{}
	for table, b := range before {
		a := after[table]
		want := b.rows
		if table == "mfa_backup_codes" {
			want = int64(len(realCodes))
		}
		require.Equal(t, want, a.rows, "row count of %s", table)
		if a.hash != b.hash {
			changed[table] = true
		}
	}
	require.Equal(t, map[string]bool{"mfa_backup_codes": true}, changed,
		"only the purge may alter existing rows")

	var status string
	require.NoError(t, db.Raw(`SELECT status FROM invitations WHERE id = ?`, inviteID).Scan(&status).Error)
	require.Equal(t, "pending", status, "the pending invitation must survive the upgrade")

	var overdueSent, d1Sent *time.Time
	require.NoError(t, db.Raw(`SELECT reminder_overdue_sent_at, reminder_d1_sent_at FROM mitigations WHERE id = ?`, lateMitigation).
		Row().Scan(&overdueSent, &d1Sent))
	require.Nil(t, overdueSent, "the late plan's reminder must still be sent")
	require.Nil(t, d1Sent)

	var reason string
	require.NoError(t, db.Raw(`SELECT suspended_reason FROM automation_rules WHERE id = ?`, disabledRule).Scan(&reason).Error)
	require.Equal(t, "", reason, "no invented reason on a rule disabled after the field existed")

	for user, want := range map[uuid.UUID]int64{realUser: int64(len(realCodes)), universalUser: 0, oldUniversalUser: 0} {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM mfa_backup_codes WHERE user_id = ?`, user).Scan(&n).Error)
		require.Equal(t, want, n, "backup codes left for user %s", user)
	}

	// Everyone who lost codes is on the ledger once, and is handed out once:
	// that is what the boot reads to mail them (#1001).
	ledger := repository.NewGormMFABackupCodePurgeRepository(db)
	claimed, err := ledger.ClaimPending(context.Background(), 100)
	require.NoError(t, err)
	got := map[uuid.UUID]uuid.UUID{}
	for _, c := range claimed {
		got[c.UserID] = c.TenantID
	}
	require.Equal(t, map[uuid.UUID]uuid.UUID{universalUser: tenant, oldUniversalUser: tenant}, got)
	again, err := ledger.ClaimPending(context.Background(), 100)
	require.NoError(t, err)
	require.Empty(t, again, "a second boot must not mail anyone again")
}

type tableDigest struct {
	rows int64
	hash string
}

// digestColumns pins every table's columns before the upgrade, so the digest
// after it covers the same set: a column the upgrade adds is not a change to
// the rows that were there.
func digestColumns(t *testing.T, db *gorm.DB) map[string][]string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(`SELECT tablename FROM pg_tables
		WHERE schemaname = current_schema() AND tablename <> 'schema_migrations' ORDER BY 1`).Scan(&tables).Error)
	out := make(map[string][]string, len(tables))
	for _, table := range tables {
		var cols []string
		require.NoError(t, db.Raw(`SELECT column_name FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? ORDER BY column_name`, table).Scan(&cols).Error)
		out[table] = cols
	}
	return out
}

// tableDigests returns each table's row count and an md5 over its rows,
// NULL kept distinct from the empty string.
func tableDigests(t *testing.T, db *gorm.DB, cols map[string][]string) map[string]tableDigest {
	t.Helper()
	out := make(map[string]tableDigest, len(cols))
	for table, cs := range cols {
		parts := make([]string, len(cs))
		for i, c := range cs {
			parts[i] = fmt.Sprintf(`coalesce(quote_literal(%q::text), 'NULL')`, c)
		}
		var d tableDigest
		row := db.Raw(fmt.Sprintf(`SELECT count(*), md5(coalesce(string_agg(r, '|' ORDER BY r), ''))
			FROM (SELECT concat_ws(',', %s) AS r FROM %q) s`, strings.Join(parts, ", "), table)).Row()
		require.NoError(t, row.Scan(&d.rows, &d.hash), table)
		out[table] = d
	}
	return out
}
