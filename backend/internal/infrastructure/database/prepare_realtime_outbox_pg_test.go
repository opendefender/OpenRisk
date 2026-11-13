// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package database

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #344: a database from before the outbox, booted WITHOUT the SQL migration
// layer (the compose files set DB_HOST, not DATABASE_URL). AutoMigrate alone
// would add published_at with every legacy row NULL, and the relay would
// republish the whole retention window to every connected client. The prepare
// step adds the column with a default that reads as "published now", then drops
// the default, and only while the column is absent. Runs in a private schema.
func TestPrepareRealtimeOutbox_GatedOnAbsentColumn(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	schema := "rtprep344_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
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
	count := func(where string) int64 {
		t.Helper()
		var n int64
		if err := db.Raw("SELECT COUNT(*) FROM realtime_events WHERE " + where).Scan(&n).Error; err != nil {
			t.Fatalf("count %s: %v", where, err)
		}
		return n
	}

	// Before the outbox: the 0059 shape, three rows of history.
	exec(`CREATE TABLE realtime_events (
		id uuid PRIMARY KEY, tenant_id uuid NOT NULL, sequence bigint NOT NULL,
		type varchar(64) NOT NULL, version int NOT NULL DEFAULT 1, envelope_version int NOT NULL DEFAULT 1,
		aggregate_type varchar(64) NOT NULL, aggregate_id varchar(128), actor_id uuid,
		correlation_id varchar(64), causation_id varchar(64), payload jsonb,
		occurred_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now())`)
	tenant := uuid.New()
	for i := 1; i <= 3; i++ {
		exec(`INSERT INTO realtime_events (id, tenant_id, sequence, type, aggregate_type, occurred_at, created_at)
			VALUES (?, ?, ?, 'asset.created', 'asset', now(), now())`, uuid.New(), tenant, i)
	}

	// The DB_HOST boot: prepare, then AutoMigrate.
	if err := PrepareForAutoMigrate(db); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := db.AutoMigrate(&domain.RealtimeEvent{}, &domain.RealtimeEventSequence{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	if n := count("published_at IS NULL"); n != 0 {
		t.Fatalf("%d legacy row(s) read as pending: the relay would republish history", n)
	}
	if n := count("published_at IS NOT NULL"); n != 3 {
		t.Fatalf("want 3 legacy rows read as published, got %d", n)
	}
	if n := count("published_at IS NULL AND abandoned_at IS NULL"); n != 0 {
		t.Fatalf("pending = %d, want 0", n)
	}
	var def *string
	if err := db.Raw(`SELECT column_default FROM information_schema.columns
		WHERE table_schema = ? AND table_name = 'realtime_events' AND column_name = 'published_at'`, schema).Scan(&def).Error; err != nil {
		t.Fatalf("default: %v", err)
	}
	if def != nil {
		t.Fatalf("published_at keeps default %q: new rows would be born published", *def)
	}
	var idx int64
	if err := db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE schemaname = ? AND indexname = 'idx_rt_outbox_pending'`, schema).Scan(&idx).Error; err != nil || idx != 1 {
		t.Fatalf("AutoMigrate did not create the partial index (n=%d err=%v)", idx, err)
	}

	// A new pending row, then a second boot: prepare must be a no-op, so the
	// pending row is NOT stamped published.
	exec(`INSERT INTO realtime_events (id, tenant_id, sequence, type, aggregate_type, occurred_at, created_at)
		VALUES (?, ?, 4, 'asset.created', 'asset', now(), now())`, uuid.New(), tenant)
	if err := PrepareForAutoMigrate(db); err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	if err := db.AutoMigrate(&domain.RealtimeEvent{}, &domain.RealtimeEventSequence{}); err != nil {
		t.Fatalf("second automigrate: %v", err)
	}
	if n := count("published_at IS NULL"); n != 1 {
		t.Fatalf("a second boot changed rows: %d pending, want 1", n)
	}
}

// On a fresh database (no table yet) the prepare step does nothing and
// AutoMigrate builds the table with every row born pending.
func TestPrepareRealtimeOutbox_FreshDatabaseIsANoOp(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	schema := "rtprep344f_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
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
	if err := prepareRealtimeOutbox(db); err != nil {
		t.Fatalf("prepare on a fresh database: %v", err)
	}
	if err := db.AutoMigrate(&domain.RealtimeEvent{}, &domain.RealtimeEventSequence{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if !db.Migrator().HasColumn("realtime_events", "published_at") || !db.Migrator().HasTable("realtime_event_sequences") {
		t.Fatal("AutoMigrate must build the outbox schema on a fresh database")
	}
}
