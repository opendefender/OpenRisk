// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package testdb is the only way a test opens a database: gorm.Open with the
// tenant guard (#808, D-111) installed in Enforce mode. coverage_test.go fails
// the build if a test reaches for gorm.Open directly.
package testdb

import (
	"fmt"
	"sync"
	"testing"

	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
)

// Open is gorm.Open with the tenant guard in Enforce mode. Statements issued
// directly from a *_test.go file (fixtures, assertions) are exempt; a query
// issued by production code is checked even when a test calls it. Every
// violation fails t at cleanup, even when the code under test dropped the
// error. The report is made at cleanup, not from the sink, because worker
// goroutines can outlive the test and logging after it finished panics.
func Open(t testing.TB, d gorm.Dialector, opts ...gorm.Option) (*gorm.DB, error) {
	t.Helper()
	db, err := gorm.Open(d, opts...)
	if err != nil {
		return nil, err
	}
	var (
		mu   sync.Mutex
		seen = map[tenantguard.Violation]bool{}
		list []tenantguard.Violation
	)
	collect := func(v tenantguard.Violation) {
		mu.Lock()
		defer mu.Unlock()
		if !seen[v] {
			seen[v] = true
			list = append(list, v)
		}
	}
	guard := tenantguard.New(tenantguard.Enforce, tenantguard.WithTestFileExemption(), tenantguard.WithSink(collect))
	// A test may pass one *gorm.Config to several Open calls (an admin
	// connection and a schema-scoped one). gorm.Open then rebuilds the shared
	// callbacks while the plugin registry still holds the previous guard, so
	// Use would answer ErrRegistered and leave this database unguarded.
	delete(db.Config.Plugins, guard.Name())
	if err := db.Use(guard); err != nil {
		return nil, fmt.Errorf("testdb: install tenant guard: %w", err)
	}
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, v := range list {
			t.Errorf("tenant guard: %s on %s (%s) from %s", v.Op, v.Table, v.Rule, v.Caller)
		}
	})
	return db, nil
}
