// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The census runs on Postgres in production and on sqlite in the other tests,
// so this runs it against a real database when DATABASE_URL is set. It works
// inside one transaction on a temporary `users` table — which shadows the real
// one for that connection — and rolls everything back.
func TestPasswordHashCensus_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// A stand-in for auth.HashAlgorithm, which this package cannot import.
	classify := func(h string) string {
		switch {
		case strings.HasPrefix(h, "$argon2id$"):
			return "argon2id"
		case len(h) == 64:
			return "sha256_legacy"
		default:
			return "unknown"
		}
	}
	legacy := strings.Repeat("a", 64)

	err = db.Transaction(func(tx *gorm.DB) error {
		exec := func(sql string, args ...interface{}) {
			t.Helper()
			if err := tx.Exec(sql, args...).Error; err != nil {
				t.Fatalf("%s: %v", sql, err)
			}
		}
		exec(`CREATE TEMP TABLE users (
			id uuid PRIMARY KEY, password text, deleted_at timestamptz
		) ON COMMIT DROP`)
		exec(`INSERT INTO users (id, password) VALUES (?, '$argon2id$v=19$m=65536,t=3,p=4$x$y')`, uuid.New())
		exec(`INSERT INTO users (id, password) VALUES (?, ?)`, uuid.New(), legacy)
		exec(`INSERT INTO users (id, password, deleted_at) VALUES (?, ?, now())`, uuid.New(), legacy)
		exec(`INSERT INTO users (id, password) VALUES (?, NULL)`, uuid.New()) // SSO-only account

		counts, err := NewPasswordHashCensus(tx, classify).Count(context.Background())
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		for _, c := range []struct {
			state, algorithm string
			want             int64
		}{
			{"active", "argon2id", 1},
			{"active", "sha256_legacy", 1},
			{"active", "unknown", 1},
			{"deleted", "sha256_legacy", 1},
		} {
			if got := counts[c.state][c.algorithm]; got != c.want {
				t.Errorf("%s/%s = %d, want %d", c.state, c.algorithm, got, c.want)
			}
		}
		return errRollback
	})
	if err != errRollback {
		t.Fatalf("transaction: %v", err)
	}
}
