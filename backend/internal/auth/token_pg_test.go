// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// newPgTokenHarness opens DATABASE_URL on a schema of its own, so the test's
// refresh_tokens table never meets the real one, and drops it afterwards.
func newPgTokenHarness(t *testing.T) (*TokenManager, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		if sqlDB, err := admin.DB(); err == nil {
			sqlDB.Close()
		}
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&RefreshToken{}))

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return NewTokenManager(db, &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}), db
}

// waitForLockWaiter blocks until some backend is waiting on a row lock in the
// test's table: the revoking DELETE has started, taken its snapshot, and is
// parked behind the lock the test holds.
func waitForLockWaiter(t *testing.T, db *gorm.DB) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query ILIKE 'DELETE FROM "refresh_tokens"%'`).Scan(&n).Error)
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the revoking DELETE never blocked on the held lock")
}

// TestRevocation_RacingRotation_Postgres drives, on Postgres, the interleaving
// SQLite cannot produce (#725). Under READ COMMITTED a DELETE reads from the
// snapshot taken when it starts, and its deletions stay invisible until it
// commits. So:
//
//  1. the rotation claims the presented token (the witness);
//  2. a revocation starts its DELETE, and a lock held by the test parks it
//     before it commits;
//  3. the rotation stores its successor and still sees the witness, because the
//     DELETE has not committed, so it hands the successor out;
//  4. the lock is released, and the DELETE commits without the successor, which
//     was not in its snapshot.
//
// Every revoker must leave no token of what it revoked, and the token handed out
// in step 3 must be refused.
func TestRevocation_RacingRotation_Postgres(t *testing.T) {
	cases := []struct {
		name   string
		revoke func(ctx context.Context, tm *TokenManager, rt RefreshToken) error
	}{
		{"family", func(ctx context.Context, tm *TokenManager, rt RefreshToken) error {
			tm.revokeFamily(ctx, rt.FamilyID)
			return nil
		}},
		{"all user tokens", func(ctx context.Context, tm *TokenManager, rt RefreshToken) error {
			return tm.RevokeAllUserTokens(ctx, rt.UserID)
		}},
		{"user tokens in tenant", func(ctx context.Context, tm *TokenManager, rt RefreshToken) error {
			return tm.RevokeUserTokensInTenant(ctx, rt.UserID, rt.TenantID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tm, db := newPgTokenHarness(t)
			ctx := context.Background()
			userID, orgID := uuid.New(), uuid.New()

			pair, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
			require.NoError(t, err)
			var witness RefreshToken
			require.NoError(t, db.First(&witness).Error)

			lock := db.Begin()
			defer lock.Rollback()
			revoked := make(chan error, 1)

			// The org resolver runs between the claim and the insert.
			tm.SetOrgSessionResolver(func(_ context.Context, _ uuid.UUID, org uuid.UUID) (*SessionClaims, error) {
				var held RefreshToken
				if err := lock.Raw("SELECT * FROM refresh_tokens WHERE id = ? FOR UPDATE", witness.ID).Scan(&held).Error; err != nil {
					return nil, fmt.Errorf("lock witness: %w", err)
				}
				go func() { revoked <- tc.revoke(ctx, tm, witness) }()
				waitForLockWaiter(t, db)
				return &SessionClaims{TenantID: org, Permissions: []string{"*"}}, nil
			})

			issued, rotErr := tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})

			require.NoError(t, lock.Commit().Error)
			require.NoError(t, <-revoked)

			require.Equal(t, int64(0), countTokens(t, db), "no token may outlive the revocation it raced")
			if rotErr == nil {
				_, err := tm.RefreshTokenPair(ctx, issued.RefreshToken, DeviceContext{})
				require.ErrorIs(t, err, ErrRefreshTokenInvalid, "the token handed out mid-revocation must be dead")
			}
		})
	}
}
