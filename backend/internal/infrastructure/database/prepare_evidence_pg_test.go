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

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #554: an evidences table from before record integrity, booted WITHOUT the SQL
// migration layer (the compose files set DB_HOST, not DATABASE_URL). Every
// legacy source must get a collection method, the observation period defaults to
// collected_at, content_hash stays empty (never invented) and nothing is dropped.
// Runs in a private schema; skipped without DATABASE_URL.
func TestPrepareEvidenceRecordIntegrity_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := testdb.Open(t, postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	schema := "evidence554_" + uuid.NewString()[:8]
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
	exec := func(sql string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	exec(`CREATE TABLE evidences (id uuid PRIMARY KEY, tenant_id uuid, source varchar(20), collected_at timestamptz, file_ref text)`)
	collected := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	ids := map[string]uuid.UUID{}
	for _, src := range []string{"manual", "integration", "scanner", "automation"} {
		ids[src] = uuid.New()
		exec(`INSERT INTO evidences VALUES (?, ?, ?, ?, 'tenants/x/evidence.pdf')`, ids[src], uuid.New(), src, collected)
	}

	if err := prepareEvidenceRecordIntegrity(db); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	for src, id := range ids {
		var row struct {
			Method, Hash, FileRef string
			From, To              time.Time
		}
		if err := db.Raw(`SELECT collection_method AS method, content_hash AS hash, file_ref,
		                         observed_from AS "from", observed_to AS "to"
		                    FROM evidences WHERE id = ?`, id).Scan(&row).Error; err != nil {
			t.Fatalf("read: %v", err)
		}
		if want := string(domain.CollectionMethodForSource(domain.EvidenceSource(src))); row.Method != want {
			t.Fatalf("source %s: method %q, want %q", src, row.Method, want)
		}
		if row.Hash != "" || row.FileRef == "" {
			t.Fatalf("source %s: hash must stay empty and the row intact, got %+v", src, row)
		}
		if !row.From.Equal(collected) || !row.To.Equal(collected) {
			t.Fatalf("source %s: observation %v..%v, want %v", src, row.From, row.To, collected)
		}
	}
	// Second run: the column exists, nothing runs, nothing errors.
	if err := prepareEvidenceRecordIntegrity(db); err != nil {
		t.Fatalf("second prepare: %v", err)
	}
}
