// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// pgLegacyDB is the legacyDB of reconcile_test.go on a throwaway PostgreSQL
// schema. It skips when DATABASE_URL is unset; the CI integration job sets it.
func pgLegacyDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	schema := "mirrorcc1413_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
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
	for _, ddl := range []string{
		`CREATE TABLE compliance_controls (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, name text NOT NULL, description text,
			status text NOT NULL, created_at timestamptz, updated_at timestamptz, deleted_at timestamptz)`,
		`CREATE TABLE evidence_control_links (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, evidence_id uuid NOT NULL,
			control_id uuid NOT NULL, note text, created_at timestamptz)`,
		`CREATE TABLE risk_control_mappings (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, risk_id uuid NOT NULL,
			control_id uuid, note text, created_at timestamptz, deleted_at timestamptz)`,
		`CREATE TABLE evidences (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, deleted_at timestamptz)`,
		`CREATE TABLE risks (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, deleted_at timestamptz)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	require.NoError(t, db.AutoMigrate(Models()...))
	return db
}

// waitForBlockedInsert returns once a session is waiting on a lock in an INSERT
// into table: the reconciler has read its snapshot and is stuck behind the
// uncommitted mirror row.
func waitForBlockedInsert(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		// tenant: pg_stat_activity is a system view, it holds no tenant data.
		require.NoError(t, db.Raw(`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE ?`,
			`INSERT INTO "`+table+`"%`).Scan(&n).Error)
		if n > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("no INSERT INTO %s ever blocked", table)
}

// TestMirrorCommitDuringReconcile_DoesNotBreakIt: a mirror transaction commits
// the very rows Reconcile has planned to insert, after Reconcile took its
// REPEATABLE READ snapshot. PostgreSQL answers ON CONFLICT DO NOTHING on a row
// invisible to the snapshot with a serialization failure; Reconcile retries on
// it (it is insert-only and convergent), so the boot neither stops nor writes a
// duplicate, and the proof still holds.
func TestMirrorCommitDuringReconcile_DoesNotBreakIt(t *testing.T) {
	db := pgLegacyDB(t)
	ctx := context.Background()
	tenant, req, ev, link := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO compliance_controls (id, tenant_id, name, status, created_at, updated_at) VALUES (?, ?, 'R', 'implemented', ?, ?)`, req, tenant, now, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO evidences (id, tenant_id) VALUES (?, ?)`, ev, tenant).Error)
	require.NoError(t, db.Exec(`INSERT INTO evidence_control_links (id, tenant_id, evidence_id, control_id, created_at) VALUES (?, ?, ?, ?, ?)`, link, tenant, ev, req, now).Error)

	// The mirror transaction writes the control and the evidenced_by edge and
	// holds them uncommitted.
	mirror := db.Begin()
	require.NoError(t, mirror.Error)
	require.NoError(t, MirrorEvidenceLinked(ctx, mirror, tenant, link, ev, req, "", now))

	type outcome struct {
		res Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := Reconcile(ctx, db)
		done <- outcome{res, err}
	}()
	waitForBlockedInsert(t, db, TableControls)
	require.NoError(t, mirror.Commit().Error)

	select {
	case o := <-done:
		require.NoError(t, o.err, "a mirror commit during Reconcile must not stop the boot")
		assert.Equal(t, int64(1), o.res.Requirements)
	case <-time.After(30 * time.Second):
		t.Fatal("Reconcile did not return")
	}

	var controls, addressed, evidenced int64
	require.NoError(t, db.Model(&ControlRow{}).Where("tenant_id = ?", tenant).Count(&controls).Error)
	require.NoError(t, db.Model(&CanonicalEdgeRow{}).Where("tenant_id = ? AND verb = ?", tenant, string(canonical.VerbAddressedBy)).Count(&addressed).Error)
	require.NoError(t, db.Model(&CanonicalEdgeRow{}).Where("tenant_id = ? AND verb = ?", tenant, string(canonical.VerbEvidencedBy)).Count(&evidenced).Error)
	assert.EqualValues(t, 1, controls)
	assert.EqualValues(t, 1, addressed, "Reconcile added the addressed_by edge the mirror did not write")
	assert.EqualValues(t, 1, evidenced, "the mirror's edge, not a duplicate")
	again, err := Reconcile(ctx, db)
	require.NoError(t, err)
	assert.Zero(t, again.Created(), "converged")
}
