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
)

func setupSessionRepo(t *testing.T) *GormSessionRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Mirrors the refresh_tokens shape after migration 0042. Written out rather
	// than AutoMigrated so drift between this DDL and the real table surfaces
	// here rather than in Postgres.
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT,
			ip_address TEXT,
			user_agent TEXT,
			expires_at DATETIME NOT NULL,
			last_used_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`).Error)

	return &GormSessionRepository{db: db}
}

func seedSession(t *testing.T, r *GormSessionRepository, userID uuid.UUID, hash, fingerprint, ua string, expires time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, r.db.Exec(
		`INSERT INTO refresh_tokens
		 (id, user_id, tenant_id, token_hash, device_fingerprint, ip_address, user_agent, expires_at, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id.String(), userID.String(), uuid.New().String(), hash, fingerprint,
		"41.202.10.7", ua, expires, time.Now(), time.Now(),
	).Error)
	return id
}

func TestListByUser_ReturnsOnlyLiveSessionsForThatUser(t *testing.T) {
	repo := setupSessionRepo(t)
	ctx := context.Background()
	me, someoneElse := uuid.New(), uuid.New()

	future := time.Now().Add(24 * time.Hour)
	seedSession(t, repo, me, "hash-live", "fp-a", "Mozilla/5.0 (Macintosh) Chrome/120 Safari/537", future)
	// Expired: no longer refreshable, so nothing a user can act on.
	seedSession(t, repo, me, "hash-expired", "fp-b", "curl/8.4.0", time.Now().Add(-time.Hour))
	// Another user's live session must never appear.
	seedSession(t, repo, someoneElse, "hash-other", "fp-c", "Mozilla/5.0 (Windows) Firefox/121", future)

	records, err := repo.ListByUser(ctx, me)
	require.NoError(t, err)

	require.Len(t, records, 1, "expected only the caller's live session")
	assert.Equal(t, "hash-live", records[0].TokenHash)
	assert.Equal(t, "41.202.10.7", records[0].IPAddress)
	// The UA is turned into something a person can recognise.
	assert.Equal(t, "Chrome on macOS", records[0].Device)
}

func TestRevoke_CrossUser_IsNotFound(t *testing.T) {
	// The security property behind the SelfScoped decision in
	// internal/security/isolation/registry.go: session IDs are UUIDs handed to
	// the browser, so the user_id predicate is the only thing stopping one user
	// revoking another's session by presenting its ID.
	repo := setupSessionRepo(t)
	ctx := context.Background()
	me, victim := uuid.New(), uuid.New()

	victimSession := seedSession(t, repo, victim, "hash-victim", "fp-v", "Firefox", time.Now().Add(time.Hour))

	revoked, err := repo.Revoke(ctx, me, victimSession)
	require.NoError(t, err)
	assert.False(t, revoked, "a foreign session id must affect no rows")

	// And it is still there.
	still, err := repo.ListByUser(ctx, victim)
	require.NoError(t, err)
	assert.Len(t, still, 1, "the victim's session must survive a cross-user revoke")
}

func TestRevoke_OwnSession_Succeeds(t *testing.T) {
	repo := setupSessionRepo(t)
	ctx := context.Background()
	me := uuid.New()

	id := seedSession(t, repo, me, "hash-mine", "fp-a", "Chrome", time.Now().Add(time.Hour))

	revoked, err := repo.Revoke(ctx, me, id)
	require.NoError(t, err)
	assert.True(t, revoked)

	remaining, err := repo.ListByUser(ctx, me)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestRevokeAllExcept_KeepsTheCallersOwnSession(t *testing.T) {
	repo := setupSessionRepo(t)
	ctx := context.Background()
	me, bystander := uuid.New(), uuid.New()

	future := time.Now().Add(time.Hour)
	seedSession(t, repo, me, "hash-current", "fp-a", "Chrome", future)
	seedSession(t, repo, me, "hash-other-1", "fp-b", "Firefox", future)
	seedSession(t, repo, me, "hash-other-2", "fp-c", "Safari", future)
	seedSession(t, repo, bystander, "hash-bystander", "fp-d", "Edge", future)

	n, err := repo.RevokeAllExcept(ctx, me, "hash-current")
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)

	mine, err := repo.ListByUser(ctx, me)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, "hash-current", mine[0].TokenHash, "the caller's own session must survive")

	// Another user's sessions are out of scope entirely.
	theirs, err := repo.ListByUser(ctx, bystander)
	require.NoError(t, err)
	assert.Len(t, theirs, 1)
}

func TestRevokeAllExcept_EmptyHashRevokesEverything(t *testing.T) {
	// This is the password-reset path: no session is spared.
	repo := setupSessionRepo(t)
	ctx := context.Background()
	me := uuid.New()

	future := time.Now().Add(time.Hour)
	seedSession(t, repo, me, "h1", "fp-a", "Chrome", future)
	seedSession(t, repo, me, "h2", "fp-b", "Firefox", future)

	n, err := repo.RevokeAllExcept(ctx, me, "")
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)

	mine, err := repo.ListByUser(ctx, me)
	require.NoError(t, err)
	assert.Empty(t, mine)
}

func TestHasSeenDevice(t *testing.T) {
	repo := setupSessionRepo(t)
	ctx := context.Background()
	me, other := uuid.New(), uuid.New()

	// Deliberately expired: "have I used this device before" is a question about
	// history, not about live sessions. Alerting because a token aged out would
	// train people to ignore the alert that matters.
	seedSession(t, repo, me, "h-old", "fp-known", "Chrome", time.Now().Add(-48*time.Hour))
	seedSession(t, repo, other, "h-other", "fp-stranger", "Chrome", time.Now().Add(time.Hour))

	seen, err := repo.HasSeenDevice(ctx, me, "fp-known")
	require.NoError(t, err)
	assert.True(t, seen, "an expired session still counts as a familiar device")

	seen, err = repo.HasSeenDevice(ctx, me, "fp-new")
	require.NoError(t, err)
	assert.False(t, seen, "an unseen fingerprint must trigger the alert")

	// A fingerprint belonging to somebody else is not familiar to me.
	seen, err = repo.HasSeenDevice(ctx, me, "fp-stranger")
	require.NoError(t, err)
	assert.False(t, seen)

	// No fingerprint at all: report "seen" so the caller stays silent rather than
	// alerting on every sign-in from a client that sends none.
	seen, err = repo.HasSeenDevice(ctx, me, "")
	require.NoError(t, err)
	assert.True(t, seen)
}
