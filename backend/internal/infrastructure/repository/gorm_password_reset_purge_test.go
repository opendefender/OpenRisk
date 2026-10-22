// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// D-074 (#879): expired reset requests are deleted; live ones, and the ones
// the rate-limit window still counts, are kept.
func TestPasswordResetRepository_DeleteExpiredBefore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE password_reset_tokens (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := sqliteschema.Reconcile(db, "password_reset_tokens", &domain.PasswordResetToken{}); err != nil {
		t.Fatal(err)
	}
	repo := NewGormPasswordResetRepository(db)
	ctx := context.Background()
	now := time.Now()

	add := func(expiresAt time.Time) uuid.UUID {
		row := &domain.PasswordResetToken{
			ID: uuid.New(), EmailHash: domain.HashEmailForReset("awa@example.test"),
			ExpiresAt: expiresAt, CreatedAt: expiresAt.Add(-domain.PasswordResetTTL),
		}
		if err := repo.Create(ctx, row); err != nil {
			t.Fatal(err)
		}
		return row.ID
	}
	old := add(now.Add(-3 * time.Hour))
	inWindow := add(now.Add(-10 * time.Minute)) // expired, but still counted by the hourly cap
	live := add(now.Add(20 * time.Minute))

	n, err := repo.DeleteExpiredBefore(ctx, now.Add(-domain.PasswordResetWindow))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted %d rows, want 1", n)
	}
	var left []uuid.UUID
	if err := db.Model(&domain.PasswordResetToken{}).Pluck("id", &left).Error; err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]bool{}
	for _, id := range left {
		got[id] = true
	}
	if got[old] || !got[inWindow] || !got[live] {
		t.Fatalf("kept %v; want the in-window and live rows only", left)
	}
}
