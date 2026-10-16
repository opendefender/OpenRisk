// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package database

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// #757: a database from before the overdue reminder, booted WITHOUT the SQL
// migration layer (the compose files set DB_HOST, not DATABASE_URL). Plans
// already late must be stamped before the sweep can see them, or the first
// sweep mails every one of them. The backfill runs once: a plan that becomes
// late afterwards is news and keeps its reminder. Runs in a private schema.
func TestPrepareMitigationOverdueReminder_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	schema := "overdue757_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE") })

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := gorm.Open(postgres.Open(dsn+sep+"search_path="+schema), cfg)
	if err != nil {
		t.Fatalf("open schema: %v", err)
	}
	t.Cleanup(func() {
		if s, err := db.DB(); err == nil {
			_ = s.Close()
		}
	})
	exec := func(sql string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	// An old shape: D-7 exists, D-1 and overdue do not.
	exec(`CREATE TABLE mitigations (id uuid PRIMARY KEY, tenant_id uuid, status text, due_date timestamptz, reminder_d7_sent_at timestamptz)`)
	late, upcoming := uuid.New(), uuid.New()
	exec(`INSERT INTO mitigations VALUES (?, ?, 'IN_PROGRESS', ?, NULL), (?, ?, 'IN_PROGRESS', ?, NULL)`,
		late, uuid.New(), time.Now().AddDate(0, 0, -40),
		upcoming, uuid.New(), time.Now().AddDate(0, 0, 3))

	if err := prepareMitigationOverdueReminder(db); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	stamped := func(id uuid.UUID) (overdue, d1, d7 bool) {
		t.Helper()
		var row struct{ O, D1, D7 bool }
		if err := db.Raw(`SELECT reminder_overdue_sent_at IS NOT NULL AS o,
		                         reminder_d1_sent_at IS NOT NULL AS d1,
		                         reminder_d7_sent_at IS NOT NULL AS d7
		                    FROM mitigations WHERE id = ?`, id).Scan(&row).Error; err != nil {
			t.Fatalf("read: %v", err)
		}
		return row.O, row.D1, row.D7
	}
	if o, d1, d7 := stamped(late); !o || !d1 || !d7 {
		t.Fatalf("an already-late plan must be fully stamped, got overdue=%v d1=%v d7=%v", o, d1, d7)
	}
	if o, d1, d7 := stamped(upcoming); o || d1 || d7 {
		t.Fatalf("an upcoming plan must keep every reminder, got overdue=%v d1=%v d7=%v", o, d1, d7)
	}

	// Later boots: the column exists, nothing runs. The plan that has just
	// become late keeps its one overdue reminder.
	exec(`UPDATE mitigations SET due_date = ? WHERE id = ?`, time.Now().AddDate(0, 0, -1), upcoming)
	if err := prepareMitigationOverdueReminder(db); err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	if o, _, _ := stamped(upcoming); o {
		t.Fatal("the backfill must run once, not on every boot")
	}
}
