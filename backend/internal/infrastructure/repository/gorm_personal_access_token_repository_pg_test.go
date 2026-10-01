// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// patPGSchema creates a private schema holding the users, organizations and
// personal_access_tokens tables, and returns a function that opens a NEW
// connection pool on it. Each pool is a separate *gorm.DB, so a value written
// through one and read through another went through PostgreSQL, not through
// any process memory. The schema is dropped when the test ends.
func patPGSchema(t *testing.T) func() *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "pat782_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}

	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	scoped := dsn + sep + "search_path=" + schema
	if !strings.Contains(dsn, "://") { // key=value DSN
		scoped = dsn + " search_path=" + schema
	}

	open := func() *gorm.DB {
		t.Helper()
		db, err := gorm.Open(postgres.Open(scoped), cfg)
		if err != nil {
			t.Fatalf("open %s: %v", schema, err)
		}
		t.Cleanup(func() {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		return db
	}

	if err := open().AutoMigrate(&domain.Organization{}, &domain.User{}, &domain.PersonalAccessToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return open
}

func seedPATOwner(t *testing.T, db *gorm.DB) (userID uuid.UUID) {
	t.Helper()
	userID = uuid.New()
	err := db.Exec(`INSERT INTO users (id, email, username, password, full_name, created_at, updated_at)
	                VALUES (?, ?, ?, 'x', 'Owner', now(), now())`,
		userID, userID.String()+"@example.test", userID.String()).Error
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return userID
}

func newPATRow(userID, tenantID uuid.UUID, hash string) *domain.PersonalAccessToken {
	return &domain.PersonalAccessToken{
		UserID:      userID,
		TenantID:    tenantID,
		Name:        "CI/CD",
		TokenHash:   hash,
		TokenPrefix: hash[:8],
		Scopes:      []byte(`["*"]`),
	}
}

// Criterion 2 of #782: a token written through one connection pool is found by
// its hash through a pool opened afterwards — a restarted server.
func TestPATRepo_Postgres_SurvivesRestart(t *testing.T) {
	open := patPGSchema(t)
	ctx := context.Background()

	before := open()
	user, tenant := seedPATOwner(t, before), uuid.New()
	hash := fmt.Sprintf("%064x", uuid.New().ID())
	row := newPATRow(user, tenant, hash)
	if err := NewGormPersonalAccessTokenRepository(before).Create(ctx, row); err != nil {
		t.Fatalf("create: %v", err)
	}
	if row.ID == uuid.Nil {
		t.Fatal("Postgres did not assign an id")
	}
	sqlDB, _ := before.DB()
	_ = sqlDB.Close()

	after := NewGormPersonalAccessTokenRepository(open())
	got, err := after.GetByTokenHash(ctx, hash)
	if err != nil {
		t.Fatalf("token lost across the reconnect: %v", err)
	}
	if got.ID != row.ID || got.TenantID != tenant || got.UserID != user {
		t.Fatalf("read back %+v, want id %s tenant %s user %s", got, row.ID, tenant, user)
	}
	if err := after.UpdateLastUsed(ctx, tenant, got.ID); err != nil {
		t.Fatalf("update last used: %v", err)
	}
	again, _ := after.GetByTokenHash(ctx, hash)
	if again.LastUsedAt == nil {
		t.Error("last_used_at was not stamped")
	}
}

// Criterion 4 of #782, at the SQL level: the same person's tokens in two
// organizations never cross. Listing, revoking and stamping from the other
// tenant all match zero rows.
func TestPATRepo_TenantIsolation(t *testing.T) {
	open := patPGSchema(t)
	ctx := context.Background()
	db := open()
	repo := NewGormPersonalAccessTokenRepository(db)

	owner, other := seedPATOwner(t, db), seedPATOwner(t, db)
	tenantA, tenantB := uuid.New(), uuid.New()
	inA := newPATRow(owner, tenantA, fmt.Sprintf("%064x", 0xA))
	inB := newPATRow(owner, tenantB, fmt.Sprintf("%064x", 0xB))
	for _, r := range []*domain.PersonalAccessToken{inA, inB} {
		if err := repo.Create(ctx, r); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	listA, err := repo.ListByOwner(ctx, tenantA, owner)
	if err != nil || len(listA) != 1 || listA[0].ID != inA.ID {
		t.Fatalf("tenant A lists %v (err %v), want only its own token", listA, err)
	}
	if list, _ := repo.ListByOwner(ctx, tenantA, other); len(list) != 0 {
		t.Errorf("another user in tenant A lists %d tokens", len(list))
	}

	if removed, err := repo.DeleteByOwner(ctx, tenantB, owner, inA.ID); err != nil || removed {
		t.Errorf("revoking A's token from B: removed=%v err=%v, want false", removed, err)
	}
	if removed, err := repo.DeleteByOwner(ctx, tenantA, other, inA.ID); err != nil || removed {
		t.Errorf("revoking A's token as another user: removed=%v err=%v, want false", removed, err)
	}

	if err := repo.UpdateLastUsed(ctx, tenantB, inA.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetByTokenHash(ctx, inA.TokenHash); got.LastUsedAt != nil {
		t.Error("a stamp issued from tenant B touched tenant A's token")
	}

	if removed, err := repo.DeleteByOwner(ctx, tenantA, owner, inA.ID); err != nil || !removed {
		t.Fatalf("owner revoking in A: removed=%v err=%v, want true", removed, err)
	}
	if list, _ := repo.ListByOwner(ctx, tenantB, owner); len(list) != 1 {
		t.Errorf("revoking in A removed tenant B's token too: %d left", len(list))
	}
}
