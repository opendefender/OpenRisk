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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

func setupUserEmailRepo(t *testing.T) (*GormUserRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Hand-written: both models carry Postgres-only defaults sqlite rejects. The
	// email column is unique and case-sensitive, as in production.
	for _, ddl := range []string{
		`CREATE TABLE roles (id TEXT PRIMARY KEY, name TEXT, description TEXT,
			permissions TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE, password TEXT, full_name TEXT, bio TEXT,
			phone TEXT, department TEXT, timezone TEXT, role_id TEXT, is_active BOOLEAN,
			avatar_url TEXT, avatar_key TEXT, locale TEXT, date_format TEXT,
			theme_mode TEXT, last_login DATETIME, tenant_id TEXT, default_org_id TEXT,
			created_by_id TEXT, created_at DATETIME, updated_at DATETIME,
			deleted_at DATETIME)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	return NewGormUserRepository(db), db
}

func insertUser(t *testing.T, db *gorm.DB, email string, created time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Create(&domain.User{
		ID: id, Email: email, Username: id.String()[:8], Password: "x",
		IsActive: true, CreatedAt: created,
	}).Error)
	return id
}

// #688: login used to match the address byte for byte, while sign-up and reset
// normalised it, so the account depended on the casing typed that day.
func TestGetByEmail_CaseInsensitive(t *testing.T) {
	repo, db := setupUserEmailRepo(t)
	id := insertUser(t, db, "awa.diallo@e2e.test", time.Now())

	for _, typed := range []string{"awa.diallo@e2e.test", "Awa.Diallo@E2E.test", "AWA.DIALLO@E2E.TEST"} {
		u, err := repo.GetByEmail(context.Background(), typed)
		require.NoError(t, err)
		require.NotNil(t, u, typed)
		assert.Equal(t, id, u.ID, typed)
	}
}

// Rows written before #687 may still hold a mixed-case address, and may even
// collide with a lowercase twin. The lookup must find the legacy row, and must
// pick the same row every time when there are two.
func TestGetByEmail_LegacyMixedCaseRows(t *testing.T) {
	repo, db := setupUserEmailRepo(t)
	ctx := context.Background()
	t0 := time.Now().Add(-48 * time.Hour)

	legacy := insertUser(t, db, "Karim@Alpha.test", t0)
	u, err := repo.GetByEmail(ctx, "karim@alpha.test")
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, legacy, u.ID, "a legacy mixed-case row is found by its normalised address")

	exact := insertUser(t, db, "karim@alpha.test", t0.Add(time.Hour))
	for i := 0; i < 3; i++ {
		u, err = repo.GetByEmail(ctx, "karim@alpha.test")
		require.NoError(t, err)
		assert.Equal(t, exact, u.ID, "the row spelled exactly as asked wins")
	}
	u, err = repo.GetByEmail(ctx, "KARIM@alpha.test")
	require.NoError(t, err)
	assert.Equal(t, legacy, u.ID, "with no exact spelling, the oldest row wins")

	u, err = repo.GetByEmail(ctx, "nobody@alpha.test")
	require.NoError(t, err)
	assert.Nil(t, u)
}
