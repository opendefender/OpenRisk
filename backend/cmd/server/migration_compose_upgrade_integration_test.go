// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #1001 — the compose and self-host installs ran on AutoMigrate alone: their
// boot never set DATABASE_URL, so the SQL layer never ran there. Turning it on
// replays the whole chain, 0001 to the last, on a database that already holds
// data. 0058 used to drop and recreate invitations, which destroyed every
// pending invitation (and the links already mailed). This replays that
// upgrade in a throwaway schema.
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

func TestComposeUpgrade_SQLLayerKeepsPendingInvitations(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin := openGorm(t, dsn)
	schema := "compose1001_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() { _ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error })

	scoped := withParams(t, dsn, map[string]string{"search_path": schema + ",public"})
	db := openGorm(t, scoped)

	// The compose install: a boot without DATABASE_URL, so Prepare + AutoMigrate
	// and no schema_migrations table.
	require.NoError(t, database.PrepareForAutoMigrate(db))
	require.NoError(t, db.AutoMigrate(schemaModels()...))

	// A pending invitation, as an administrator would have left it. Foreign keys
	// are suspended for this one insert only, so the test does not have to
	// build a whole organization and user to hold one row.
	inviteID := uuid.New()
	tx := db.Begin()
	require.NoError(t, tx.Exec(`SET LOCAL session_replication_role = replica`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO invitations
		(id, organization_id, email, role, token_hash, status, expires_at, invited_by_id, last_sent_at, send_count, created_at, updated_at)
		VALUES (?, ?, 'invitee@upgrade.example', 'user', ?, 'pending', ?, ?, now(), 1, now(), now())`,
		inviteID, uuid.New(), strings.Repeat("a", 64), time.Now().Add(72*time.Hour), uuid.New()).Error)
	require.NoError(t, tx.Commit().Error)

	// Turning the SQL layer on: the whole chain on that database.
	m := newMigrator(t, scoped)
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate up on an AutoMigrate-only database: %v", err)
	}
	_, dirty, err := m.Version()
	require.NoError(t, err)
	require.False(t, dirty)

	var status string
	require.NoError(t, db.Raw(`SELECT status FROM invitations WHERE id = ?`, inviteID).Scan(&status).Error)
	require.Equal(t, "pending", status, "the pending invitation must survive the upgrade")
}
