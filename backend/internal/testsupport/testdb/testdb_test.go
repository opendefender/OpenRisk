// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package testdb_test

import (
	"fmt"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

type probeRow struct {
	ID       string `gorm:"primaryKey"`
	TenantID string `gorm:"column:tenant_id"`
}

// fakeTB records Errorf and runs cleanups on demand.
type fakeTB struct {
	testing.TB
	errs     []string
	cleanups []func()
}

func (f *fakeTB) Helper()                        {}
func (f *fakeTB) Errorf(format string, a ...any) { f.errs = append(f.errs, fmt.Sprintf(format, a...)) }
func (f *fakeTB) Cleanup(fn func())              { f.cleanups = append(f.cleanups, fn) }
func (f *fakeTB) finish() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
}

func open(t *testing.T, tb testing.TB) *gorm.DB {
	t.Helper()
	db, err := testdb.Open(tb, sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&probeRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTestdb_ProductionCallerIsChecked(t *testing.T) {
	tb := &fakeTB{TB: t}
	db := open(t, tb)
	if err := testdb.Probe(db, &[]probeRow{}); err == nil {
		t.Fatal("an unscoped query from a non-test file must be refused")
	}
	tb.finish()
	if len(tb.errs) != 1 || !strings.Contains(tb.errs[0], "probe_rows") || !strings.Contains(tb.errs[0], "probe.go") {
		t.Fatalf("want exactly one violation naming probe.go, got %v", tb.errs)
	}
}

func TestTestdb_TestFileFixtureIsExempt(t *testing.T) {
	tb := &fakeTB{TB: t}
	db := open(t, tb)
	if err := db.Find(&[]probeRow{}).Error; err != nil {
		t.Fatalf("a fixture read from a test file must stay legal: %v", err)
	}
	tb.finish()
	if len(tb.errs) != 0 {
		t.Fatalf("want no violation, got %v", tb.errs)
	}
}

func TestTestdb_ScopedProductionQueryPasses(t *testing.T) {
	tb := &fakeTB{TB: t}
	db := open(t, tb)
	if err := testdb.Probe(db.Where("tenant_id = ?", "t"), &[]probeRow{}); err != nil {
		t.Fatal(err)
	}
	tb.finish()
	if len(tb.errs) != 0 {
		t.Fatalf("want no violation, got %v", tb.errs)
	}
}
