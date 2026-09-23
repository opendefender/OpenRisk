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
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// newTokenHarness builds a TokenManager over a temporary sqlite refresh_tokens
// table with an org resolver that yields claims for whichever org it is asked
// about (recording the last org it saw, so tests can assert org preservation).
func newTokenHarness(t *testing.T) (*TokenManager, *gorm.DB, *resolverSpy) {
	t.Helper()

	// Use a per-test temp file instead of ":memory:" so that concurrent
	// goroutines (TestRefresh_ConcurrentRotation_OneWinner) share a single
	// database through WAL mode and busy_timeout, which models the real
	// Postgres serialisation. ":memory:" with MaxOpenConns(1) was flaky
	// under make test's inter-package parallelism.
	dbPath := filepath.Join(t.TempDir(), "token_test.db")
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
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

// ageRotations pushes every rotation in the table past RotationGracePeriod, so a
// replay is judged as reuse instead of as the same client asking twice (#777).
func ageRotations(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Model(&RefreshToken{}).
		Where("rotated_at IS NOT NULL").
		Update("rotated_at", time.Now().Add(-RotationGracePeriod-time.Minute)).Error)
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

	// Replay the spent R1 once the grace window has passed: reuse detected.
	ageRotations(t, db)
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

// --- Concurrent rotation inside the grace window ---------------------------

// TestRefresh_ConcurrentRotation_WithinGraceAllSucceed covers the burst a real
// client produces — several tabs refreshing the same token at once (#777).
// Every request is served, nothing is revoked, and all the tokens minted belong
// to the one family, so a later replay still takes them all.
func TestRefresh_ConcurrentRotation_WithinGraceAllSucceed(t *testing.T) {
	tm, db, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	issued := make([]*TokenPair, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			p, e := tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
			issued[i], results[i] = p, e
		}(i)
	}
	close(start)
	wg.Wait()

	for i, e := range results {
		require.NoError(t, e, "request %d: a burst on one token is not a compromise", i)
		require.NotEmpty(t, issued[i].RefreshToken)
	}
	// The spent original plus one new token per request, all in one family.
	require.Equal(t, int64(n+1), countTokens(t, db), "every request mints exactly one token")

	var families int64
	require.NoError(t, db.Model(&RefreshToken{}).Distinct("family_id").Count(&families).Error)
	require.Equal(t, int64(1), families, "the burst must not fork the lineage")

	// Past the window, the original reads as a replay again and takes the whole
	// burst with it.
	ageRotations(t, db)
	_, err = tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
	require.ErrorIs(t, err, ErrRefreshTokenReuse)
	require.Equal(t, int64(0), countTokens(t, db), "reuse revokes everything minted during the window")
}

// --- Replay inside the window -----------------------------------------------

// TestRefresh_ReplayWithinGrace_MintsSibling is the retry case: the same client
// presents a token it has already spent, seconds later, because the first
// response never arrived.
func TestRefresh_ReplayWithinGrace_MintsSibling(t *testing.T) {
	tm, db, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair1, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)
	pair2, err := tm.RefreshTokenPair(ctx, pair1.RefreshToken, DeviceContext{})
	require.NoError(t, err)

	pair3, err := tm.RefreshTokenPair(ctx, pair1.RefreshToken, DeviceContext{})
	require.NoError(t, err, "a replay inside the window is served, not punished")
	require.NotEqual(t, pair2.RefreshToken, pair3.RefreshToken, "each replay gets its own token")
	require.Equal(t, int64(3), countTokens(t, db), "the spent original plus both siblings")

	// Both siblings work: the family was never revoked.
	_, err = tm.RefreshTokenPair(ctx, pair2.RefreshToken, DeviceContext{})
	require.NoError(t, err)
	_, err = tm.RefreshTokenPair(ctx, pair3.RefreshToken, DeviceContext{})
	require.NoError(t, err)
}

// TestRefresh_ReplayWithinGrace_DeviceMismatch shows the window is not a hole:
// tolerance applies to the client that owns the token, not to another device
// holding a copy of it.
func TestRefresh_ReplayWithinGrace_DeviceMismatch(t *testing.T) {
	tm, db, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	owner := DeviceContext{Fingerprint: "device-a"}
	pair1, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, owner)
	require.NoError(t, err)
	_, err = tm.RefreshTokenPair(ctx, pair1.RefreshToken, owner)
	require.NoError(t, err)

	before := countTokens(t, db)
	_, err = tm.RefreshTokenPair(ctx, pair1.RefreshToken, DeviceContext{Fingerprint: "device-b"})
	require.ErrorIs(t, err, ErrDeviceMismatch, "the window never covers another device")
	require.Equal(t, before, countTokens(t, db), "a refused replay mints nothing")
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

// --- A revocation landing mid-rotation takes the new token with it ---------

// TestRefresh_FamilyRevokedDuringRotation drives the exact interleaving that
// made TestRefresh_ConcurrentRotation_OneWinner flaky (#775): the family is
// revoked after the presented token has been claimed but before the new one is
// stored. The rotation must not hand back a token that outlives the revocation.
func TestRefresh_FamilyRevokedDuringRotation(t *testing.T) {
	tm, db, _ := newTokenHarness(t)
	ctx := context.Background()
	userID, orgID := uuid.New(), uuid.New()

	pair, err := tm.GenerateTokenPair(ctx, userID, orgID, nil, []string{"*"}, nil, DeviceContext{})
	require.NoError(t, err)

	var family uuid.UUID
	require.NoError(t, db.Model(&RefreshToken{}).Select("family_id").Row().Scan(&family))

	// The org resolver runs between the claim and the insert, which is where a
	// concurrent reuse detection would sweep the family.
	tm.SetOrgSessionResolver(func(_ context.Context, _ uuid.UUID, org uuid.UUID) (*SessionClaims, error) {
		require.NoError(t, db.Where("family_id = ?", family).Delete(&RefreshToken{}).Error)
		return &SessionClaims{TenantID: org, Permissions: []string{"*"}}, nil
	})

	_, err = tm.RefreshTokenPair(ctx, pair.RefreshToken, DeviceContext{})
	require.ErrorIs(t, err, ErrRefreshTokenReuse, "a rotation that raced a revocation must refuse")
	require.Equal(t, int64(0), countTokens(t, db), "no token may survive the revocation it raced")
}
