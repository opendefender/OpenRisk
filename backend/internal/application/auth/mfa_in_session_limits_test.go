// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
)

// #1392 (D-107 option B) — ten wrong proofs in 24 h across disabling MFA and
// regenerating backup codes lock both for a fixed 24 h, mail the owner once,
// and never touch the sign-in challenge lock (#689).

type inSessionMailer struct {
	mu   sync.Mutex
	sent []string
}

func (m *inSessionMailer) SendMFAInSessionLocked(_ context.Context, to, _, locale string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, to+"|"+locale)
	return nil
}

func (m *inSessionMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// spyStore counts the locks set, to prove refusals never set or extend one.
type spyStore struct {
	*authmfa.MemoryAttemptStore
	mu    sync.Mutex
	locks int
}

func (s *spyStore) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	set, err := s.MemoryAttemptStore.Lock(ctx, key, ttl)
	if set {
		s.mu.Lock()
		s.locks++
		s.mu.Unlock()
	}
	return set, err
}

// inSessionFixture is one identity-provider account (no local password) with
// both use cases sharing one limits instance, as main.go wires them.
type inSessionFixture struct {
	disable    *disableFixture
	regenerate *regenerateFixture
	store      *spyStore
	mailer     *inSessionMailer
	limits     *MFAInSessionLimits
	plain      string
}

func newInSessionFixture(t *testing.T) *inSessionFixture {
	t.Helper()
	d := newDisableFixture(t, domain.RoleUser, "")
	plain := d.ssoAccount(t)
	d.user.Locale = "fr"

	r := newRegenerateFixture(t)
	// Same account, same tenant, same secret: the regenerate fixture reads the
	// disable fixture's user and repository.
	r.user = d.user
	r.tenant = d.tenant
	r.repo = d.repo
	r.plain = plain
	r.key = d.codeKey
	r.uc = NewRegenerateBackupCodesUseCase(d.repo, d.users, disableHasher{}, disableTOTPKey).WithMailer(r.mailer)

	store := &spyStore{MemoryAttemptStore: authmfa.NewMemoryAttemptStore()}
	mailer := &inSessionMailer{}
	limits := NewMFAInSessionLimits(store, d.users, mailer)
	d.uc.WithInSessionLimits(limits)
	r.uc.WithInSessionLimits(limits)
	return &inSessionFixture{disable: d, regenerate: r, store: store, mailer: mailer, limits: limits, plain: plain}
}

func (f *inSessionFixture) goodCode(t *testing.T) string {
	t.Helper()
	c, err := totp.GenerateCode(f.plain, time.Now())
	require.NoError(t, err)
	return c
}

func (f *inSessionFixture) badCode() string {
	// Not 6 digits, so never a valid TOTP whatever the clock says.
	return "00000x"
}

// fail sends one wrong code to disable (even i) or regenerate (odd i).
func (f *inSessionFixture) fail(i int) error {
	if i%2 == 0 {
		return f.disable.runWithCode("", f.badCode())
	}
	_, err := f.regenerate.run("", f.badCode())
	return err
}

func (f *inSessionFixture) failures(t *testing.T) int64 {
	t.Helper()
	n, err := f.store.Count(context.Background(), inSessionFailuresKey(f.disable.user.ID))
	require.NoError(t, err)
	return n
}

func (f *inSessionFixture) lockedFor(t *testing.T) time.Duration {
	t.Helper()
	d, err := f.store.LockedFor(context.Background(), inSessionLockKey(f.disable.user.ID))
	require.NoError(t, err)
	return d
}

func (f *inSessionFixture) lockOut(t *testing.T) {
	t.Helper()
	for i := 0; i < MFAInSessionMaxFailures-1; i++ {
		err := f.fail(i)
		require.True(t, errors.Is(err, ErrMFADisableCodeIncorrect) || errors.Is(err, ErrBackupCodesCodeIncorrect), "attempt %d: %v", i+1, err)
	}
	err := f.fail(MFAInSessionMaxFailures - 1)
	var locked *MFAInSessionLockedError
	require.True(t, errors.As(err, &locked), "the 10th failure sets the lock, got %v", err)
	require.True(t, locked.JustLocked)
}

func asLocked(t *testing.T, err error) *MFAInSessionLockedError {
	t.Helper()
	var locked *MFAInSessionLockedError
	require.True(t, errors.As(err, &locked), "want the in-session lock, got %v", err)
	return locked
}

func TestMFAInSessionLimits_Success(t *testing.T) {
	f := newInSessionFixture(t)

	require.NoError(t, f.disable.runWithCode("", f.goodCode(t)))
	assert.Zero(t, f.failures(t))
	assert.Zero(t, f.mailer.count())
}

func TestMFAInSessionLimits_NotFound(t *testing.T) {
	// A wrong code on an account without MFA is not a guess at anything: the
	// secret is looked up first and the answer is not-found, uncounted.
	f := newInSessionFixture(t)
	require.NoError(t, f.disable.repo.DisableMFA(context.Background(), f.disable.user.ID, f.disable.tenant))

	err := f.disable.runWithCode("", f.badCode())
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr), "got %v", err)
	assert.ErrorIs(t, appErr.Err, domain.ErrNotFound)
	assert.Zero(t, f.failures(t))
}

func TestMFAInSessionLimits_Unauthorized(t *testing.T) {
	// No session: refused before the lock or the counter is read.
	f := newInSessionFixture(t)
	_, err := f.disable.uc.Execute(context.Background(), DisableMFAInput{TenantID: f.disable.tenant, Code: f.badCode()})
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr), "got %v", err)
	assert.ErrorIs(t, appErr.Err, domain.ErrUnauthorized)
	assert.Zero(t, f.failures(t))
}

// Criterion 1: a failure on either endpoint counts against one account counter,
// whichever organisation the session belongs to; a wrong password counts too.
func TestMFAInSessionLimits_FailuresShareOneCounter(t *testing.T) {
	f := newInSessionFixture(t)

	require.ErrorIs(t, f.disable.runWithCode("", f.badCode()), ErrMFADisableCodeIncorrect)
	_, err := f.regenerate.run("", f.badCode())
	require.ErrorIs(t, err, ErrBackupCodesCodeIncorrect)
	// Another organisation's session for the same person.
	_, err = f.regenerate.uc.Execute(context.Background(), RegenerateBackupCodesInput{
		UserID: f.disable.user.ID, TenantID: uuid.New(), Code: f.badCode(),
	})
	require.Error(t, err)
	assert.Equal(t, int64(2), f.failures(t), "the other organisation has no secret: not-found, not a guess")

	t.Run("a wrong password counts", func(t *testing.T) {
		f := newInSessionFixture(t)
		f.disable.user.Password = "hashed:" + disablePassword
		require.ErrorIs(t, f.disable.run("wrong", ""), ErrMFADisablePasswordIncorrect)
		_, err := f.regenerate.run("wrong", f.goodCode(t))
		require.ErrorIs(t, err, ErrBackupCodesPasswordIncorrect)
		assert.Equal(t, int64(2), f.failures(t))
	})
}

// Criterion 2: the 10th failure locks both endpoints for 24 h and mails the
// owner once, in the locale of their account.
func TestMFAInSessionLimits_TenthFailureLocksBoth(t *testing.T) {
	f := newInSessionFixture(t)
	f.lockOut(t)

	left := f.lockedFor(t)
	assert.Greater(t, left, MFAInSessionLockDuration-time.Minute)
	assert.LessOrEqual(t, left, MFAInSessionLockDuration+time.Second)
	assert.Equal(t, 1, f.store.locks)
	assert.Equal(t, []string{"rssi@banque.cm|fr"}, f.mailer.sent)
	f.disable.assertStillEnrolled(t)
}

// Criterion 3: under the lock the correct code is refused on both endpoints,
// nothing changes and no further mail goes out.
func TestMFAInSessionLimits_CorrectCodeRefusedDuringLock(t *testing.T) {
	f := newInSessionFixture(t)
	f.lockOut(t)

	locked := asLocked(t, f.disable.runWithCode("", f.goodCode(t)))
	assert.False(t, locked.JustLocked, "a refusal is not a new lock")
	assert.Greater(t, locked.RetryAfter, time.Duration(0))

	_, err := f.regenerate.run("", f.goodCode(t))
	assert.False(t, asLocked(t, err).JustLocked)

	f.disable.assertStillEnrolled(t)
	assert.Equal(t, 1, f.mailer.count(), "one mail per lock")
	assert.Zero(t, f.regenerate.mailer.calls)
}

// Criterion 4: refused attempts neither count nor move the end of the lock.
func TestMFAInSessionLimits_RefusalsDoNotExtendTheLock(t *testing.T) {
	f := newInSessionFixture(t)
	f.lockOut(t)
	before := f.lockedFor(t)

	for i := 0; i < 25; i++ {
		asLocked(t, f.fail(i))
	}

	assert.LessOrEqual(t, f.lockedFor(t), before)
	assert.Equal(t, 1, f.store.locks, "no refusal sets a lock again")
	assert.Zero(t, f.failures(t), "refusals are not counted")
}

// Criterion 5: the in-session lock and the sign-in challenge lock are separate
// both ways.
func TestMFAInSessionLimits_SignInIsNeverAffected(t *testing.T) {
	f := newInSessionFixture(t)
	f.lockOut(t)

	// The owner signs in with a correct code while the in-session lock holds,
	// on the same shared store, as in production.
	challenge := NewChallengeMFAUseCase(f.disable.repo, disableTOTPKey).
		WithAttemptLimits(f.store, &fakeJTIRevoker{jtis: map[string]time.Duration{}})
	in := ChallengeMFAInput{UserID: f.disable.user.ID, TenantID: f.disable.tenant, ChallengeJTI: "jti-1",
		ChallengeExpiresAt: time.Now().Add(5 * time.Minute)}

	in.Code = f.badCode()
	_, err := challenge.Execute(context.Background(), in)
	require.True(t, isInvalidCode(err), "a wrong sign-in code is just wrong, got %v", err)

	in.Code = f.goodCode(t)
	_, err = challenge.Execute(context.Background(), in)
	require.NoError(t, err, "the in-session lock must not stop sign-in")

	t.Run("a sign-in failure never counts in session", func(t *testing.T) {
		f := newInSessionFixture(t)
		challenge := NewChallengeMFAUseCase(f.disable.repo, disableTOTPKey).
			WithAttemptLimits(f.store, &fakeJTIRevoker{jtis: map[string]time.Duration{}})
		for i := 0; i < 3; i++ {
			_, _ = challenge.Execute(context.Background(), ChallengeMFAInput{UserID: f.disable.user.ID,
				TenantID: f.disable.tenant, Code: f.badCode(), ChallengeJTI: "jti-x",
				ChallengeExpiresAt: time.Now().Add(5 * time.Minute)})
		}
		assert.Zero(t, f.failures(t))
		assert.Zero(t, f.lockedFor(t))
	})
}

// Criterion 6: a success clears the count; it never clears a lock.
func TestMFAInSessionLimits_SuccessClearsTheCount(t *testing.T) {
	f := newInSessionFixture(t)
	for i := 0; i < 3; i++ {
		require.Error(t, f.fail(i))
	}
	require.Equal(t, int64(3), f.failures(t))

	out, err := f.regenerate.run("", f.goodCode(t))
	require.NoError(t, err)
	require.Len(t, out.BackupCodes, 8)
	assert.Zero(t, f.failures(t))

	t.Run("a lock survives", func(t *testing.T) {
		f := newInSessionFixture(t)
		f.lockOut(t)
		f.limits.RecordSuccess(context.Background(), f.disable.user.ID)
		assert.Greater(t, f.lockedFor(t), time.Duration(0))
	})
}

type failingLockStore struct{ *authmfa.MemoryAttemptStore }

func (failingLockStore) LockedFor(context.Context, string) (time.Duration, error) {
	return 0, errors.New("store down")
}

func TestMFAInSessionLimits_StoreErrorFailsClosed(t *testing.T) {
	f := newInSessionFixture(t)
	limits := NewMFAInSessionLimits(failingLockStore{authmfa.NewMemoryAttemptStore()}, nil, nil)
	f.disable.uc.WithInSessionLimits(limits)

	err := f.disable.runWithCode("", f.goodCode(t))
	require.Error(t, err)
	f.disable.assertStillEnrolled(t)
}
