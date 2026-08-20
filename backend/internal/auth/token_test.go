// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// newTokenHarness builds a TokenManager over an in-memory sqlite refresh_tokens
// table with an org resolver that yields claims for whichever org it is asked
// about (recording the last org it saw, so tests can assert org preservation).
func newTokenHarness(t *testing.T) (*TokenManager, *gorm.DB, *resolverSpy) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT,
			ip_address TEXT,
			user_agent TEXT,
			expires_at DATETIME NOT NULL,
			rotated_at DATETIME,
			last_used_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`).Error)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}

	tm := NewTokenManager(db, keys)
	spy := &resolverSpy{}
	tm.SetOrgSessionResolver(spy.resolve)
	return tm, db, spy
}

type resolverSpy struct {
	mu       sync.Mutex
	lastOrg  uuid.UUID
	calls    int
	failWith error // when set, resolver returns this error (simulates lost membership)
}

func (s *resolverSpy) resolve(_ context.Context, _ uuid.UUID, orgID uuid.UUID) (*SessionClaims, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastOrg = orgID
	if s.failWith != nil {
		return nil, s.failWith
	}
	return &SessionClaims{
		TenantID:    orgID,
		OrgRoles:    map[uuid.UUID]string{orgID: "admin"},
		Permissions: []string{"*"},
	}, nil
}

func countTokens(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&RefreshToken{}).Count(&n).Error)
	return n
}

// --- Rotation: R1 -> R2, R1 becomes unusable -------------------------------

func TestRefresh_Rotation_Success(t *testing.T) {
	tm, _, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair1, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)

	pair2, err := tm.RefreshTokenPair(ctx, pair1.RefreshToken, DeviceContext{})
	require.NoError(t, err)
	require.NotEqual(t, pair1.RefreshToken, pair2.RefreshToken, "rotation must mint a new refresh token")

	// The new token works.
	_, err = tm.RefreshTokenPair(ctx, pair2.RefreshToken, DeviceContext{})
	require.NoError(t, err)
}

// --- Reuse detection: replaying a rotated token kills the whole family ------

func TestRefresh_Reuse_RevokesFamily(t *testing.T) {
	tm, db, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair1, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)
	pair2, err := tm.RefreshTokenPair(ctx, pair1.RefreshToken, DeviceContext{}) // R1 spent, R2 issued
	require.NoError(t, err)

	// Replay the spent R1: reuse detected.
	_, err = tm.RefreshTokenPair(ctx, pair1.RefreshToken, DeviceContext{})
	require.ErrorIs(t, err, ErrRefreshTokenReuse)

	// The family is revoked: even the still-valid R2 no longer works.
	_, err = tm.RefreshTokenPair(ctx, pair2.RefreshToken, DeviceContext{})
	require.ErrorIs(t, err, ErrRefreshTokenInvalid)
	require.Equal(t, int64(0), countTokens(t, db), "reuse must purge the entire family")
}

// --- Expiry ----------------------------------------------------------------

func TestRefresh_Expired(t *testing.T) {
	tm, db, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)
	// Force expiry in the past.
	require.NoError(t, db.Model(&RefreshToken{}).
		Where("token_hash = ?", HashToken(pair.RefreshToken)).
		Update("expires_at", time.Now().Add(-time.Hour)).Error)

	_, err = tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
	require.ErrorIs(t, err, ErrRefreshTokenExpired)
	// A replay of the (now cleaned) expired token reads as invalid, not reuse.
	_, err = tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
	require.ErrorIs(t, err, ErrRefreshTokenInvalid)
}

// --- Invalid ---------------------------------------------------------------

func TestRefresh_Invalid(t *testing.T) {
	tm, _, _ := newTokenHarness(t)
	_, err := tm.RefreshTokenPair(context.Background(), "not-a-real-token", DeviceContext{})
	require.ErrorIs(t, err, ErrRefreshTokenInvalid)
}

// --- Concurrent rotation of one token: exactly one winner ------------------

func TestRefresh_ConcurrentRotation_OneWinner(t *testing.T) {
	tm, db, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, e := tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
			results[i] = e
		}(i)
	}
	close(start)
	wg.Wait()

	success, reuse := 0, 0
	for _, e := range results {
		switch {
		case e == nil:
			success++
		case errors.Is(e, ErrRefreshTokenReuse):
			reuse++
		}
	}
	require.Equal(t, 1, success, "a single-use token may rotate at most once")
	require.GreaterOrEqual(t, reuse, 1, "concurrent losers must be flagged as reuse")
	require.Equal(t, int64(0), countTokens(t, db), "concurrent reuse revokes the family")
}

// --- Org context is preserved across refresh (not reset to default) --------

func TestRefresh_PreservesOrgContext(t *testing.T) {
	tm, _, spy := newTokenHarness(t)
	ctx := context.Background()
	userID := uuid.New()
	sessionOrg := uuid.New() // the org the session belongs to

	pair, err := tm.GenerateTokenPair(ctx, userID, sessionOrg, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)

	_, err = tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
	require.NoError(t, err)
	require.Equal(t, sessionOrg, spy.lastOrg, "refresh must resolve claims for the session's own org")
}

// --- Lost membership on refresh revokes the family -------------------------

func TestRefresh_LostMembership_RevokesFamily(t *testing.T) {
	tm, db, spy := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)

	spy.failWith = errors.New("membership is not active")
	_, err = tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
	require.Error(t, err)
	require.Equal(t, int64(0), countTokens(t, db), "a refresh that can no longer be authorized revokes the family")
}

// --- Device fingerprint binding --------------------------------------------

func TestRefresh_DeviceMismatch(t *testing.T) {
	tm, _, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{Fingerprint: "device-A"})
	require.NoError(t, err)

	_, err = tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{Fingerprint: "device-B"})
	require.ErrorIs(t, err, ErrDeviceMismatch)
}

// --- IssueSessionForOrg validates membership via the resolver --------------

func TestIssueSessionForOrg_UsesResolver(t *testing.T) {
	tm, _, spy := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	// A resolver that refuses the org (not a member) must block session minting.
	spy.failWith = errors.New("user is not a member of this organization")
	_, err := tm.IssueSessionForOrg(ctx, userID, orgID, DeviceContext{})
	require.Error(t, err)

	// When the resolver authorizes it, a session is minted for that org.
	spy.failWith = nil
	pair, err := tm.IssueSessionForOrg(ctx, userID, orgID, DeviceContext{})
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.Equal(t, orgID, spy.lastOrg)
}
