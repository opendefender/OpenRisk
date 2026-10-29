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

// #612 — organisations left on the legacy 'starter' default move to Free, the
// plans set on purpose stay, and the backfill runs twice without harm. Runs in
// a private schema.
func TestRetireStarterPlan_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	schema := "starter612_" + uuid.NewString()[:8]
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

	// A fresh database has no organizations table: nothing to do.
	if err := retireStarterPlan(db); err != nil {
		t.Fatalf("fresh database: %v", err)
	}

	for _, q := range []string{
		"CREATE TABLE organizations (name text PRIMARY KEY, plan text DEFAULT 'starter')",
		"INSERT INTO organizations VALUES ('legacy', 'starter'), ('paid', 'business'), ('free', 'free')",
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	for run := 0; run < 2; run++ {
		if err := retireStarterPlan(db); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	want := map[string]string{"legacy": "free", "paid": "business", "free": "free"}
	type row struct{ Name, Plan string }
	var rows []row
	if err := db.Raw("SELECT name, plan FROM organizations").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for _, r := range rows {
		if want[r.Name] != r.Plan {
			t.Errorf("%s: plan = %q, want %q", r.Name, r.Plan, want[r.Name])
		}
	}
}
