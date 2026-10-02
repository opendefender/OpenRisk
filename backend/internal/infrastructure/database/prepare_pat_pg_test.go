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
)

// A database from before #782 has personal_access_tokens without tenant_id.
// The backfill must give each token the tenant it already acted in (its
// owner's default organization), drop the tokens that never authenticated
// anything, and run twice without harm. Runs in a private schema.
func TestBackfillPersonalAccessTokenTenant_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	schema := "patfill782_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE") })

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	scoped := dsn + sep + "search_path=" + schema
	if !strings.Contains(dsn, "://") {
		scoped = dsn + " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(scoped), cfg)
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
	// The pre-#782 shape: no tenant_id.
	exec(`CREATE TABLE users (id uuid PRIMARY KEY, default_org_id uuid)`)
	exec(`CREATE TABLE personal_access_tokens (id uuid PRIMARY KEY, user_id uuid NOT NULL, token_hash varchar(64) NOT NULL)`)

	org := uuid.New()
	withOrg, noOrg := uuid.New(), uuid.New()
	exec(`INSERT INTO users VALUES (?, ?), (?, NULL)`, withOrg, org, noOrg)
	kept, dropped := uuid.New(), uuid.New()
	exec(`INSERT INTO personal_access_tokens VALUES (?, ?, 'a'), (?, ?, 'b')`, kept, withOrg, dropped, noOrg)

	for i := 0; i < 2; i++ { // idempotent
		if err := backfillPersonalAccessTokenTenant(db); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}

	var rows []struct {
		ID       uuid.UUID
		TenantID *uuid.UUID
	}
	if err := db.Raw(`SELECT id, tenant_id FROM personal_access_tokens`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != kept {
		t.Fatalf("rows after backfill = %+v, want only %s", rows, kept)
	}
	if rows[0].TenantID == nil || *rows[0].TenantID != org {
		t.Fatalf("tenant_id = %v, want the owner's default org %s", rows[0].TenantID, org)
	}
}
