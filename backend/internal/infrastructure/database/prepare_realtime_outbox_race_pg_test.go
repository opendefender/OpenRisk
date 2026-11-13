// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package database

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #344: replicas booting together on a DB_HOST deployment all see the column
// absent. None may fail, and the legacy rows end up published.
func TestPrepareRealtimeOutbox_ConcurrentBootsAllSucceed(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := testdb.Open(t, postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	schema := "rtprepc344_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE") })
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := testdb.Open(t, postgres.Open(dsn+sep+"search_path="+schema), cfg)
	if err != nil {
		t.Fatalf("open schema: %v", err)
	}
	t.Cleanup(func() {
		if s, err := db.DB(); err == nil {
			_ = s.Close()
		}
	})
	if err := db.Exec(`CREATE TABLE realtime_events (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT now())`).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := db.Exec(`INSERT INTO realtime_events (id, tenant_id) VALUES (?, ?)`, uuid.New(), uuid.New()).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	const boots = 6
	errs := make(chan error, boots)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < boots; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- prepareRealtimeOutbox(db)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a concurrent boot failed: %v", err)
		}
	}
	var pending int64
	if err := db.Raw(`SELECT COUNT(*) FROM realtime_events WHERE published_at IS NULL`).Scan(&pending).Error; err != nil || pending != 0 {
		t.Fatalf("legacy rows pending=%d err=%v", pending, err)
	}
}
