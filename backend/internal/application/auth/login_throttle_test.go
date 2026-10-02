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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

// exactUsers matches the address byte for byte, as the database used to, and
// records what it was asked for.
type exactUsers struct {
	*loginUsers
	asked []string
}

func (e *exactUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	e.asked = append(e.asked, email)
	if e.user != nil && e.user.Email == email {
		return e.user, nil
	}
	return nil, nil
}

// countingHasher records every hash Verify was asked to compare against.
type countingHasher struct {
	fakeHasher
	mu       sync.Mutex
	verified []string
}

func (h *countingHasher) Verify(hash, plain string) bool {
	h.mu.Lock()
	h.verified = append(h.verified, hash)
	h.mu.Unlock()
	return h.fakeHasher.Verify(hash, plain)
}

func newThrottleHarness(t *testing.T) (*LoginUseCase, *loginUsers, *countingHasher) {
	t.Helper()
	base, users, _, _ := newLoginHarness(t, domain.RoleUser)
	hasher := &countingHasher{}
	uc := NewLoginUseCase(users, base.tokenManager, hasher).WithClock(base.now)
	return uc, users, hasher
}

// #688 reproduction 4: login is reached by the normalised address.
func TestLogin_EmailCaseInsensitive(t *testing.T) {
	base, users, _, _ := newLoginHarness(t, domain.RoleUser)
	exact := &exactUsers{loginUsers: users}
	uc := NewLoginUseCase(exact, base.tokenManager, fakeHasher{}).WithClock(base.now)

	out, err := uc.Execute(context.Background(), LoginInput{
		Email: "  Admin@OpenDefender.IO ", Password: "Ancre-Vitrail7-Cobalt",
	})
	require.NoError(t, err)
	require.NotNil(t, out.TokenPair)
	assert.Equal(t, []string{"admin@opendefender.io"}, exact.asked)
}

// #688 reproduction 2: an unknown address used to answer before any hashing,
// and a disabled account likewise, so the response time told which addresses
// held an account. Every refusal for bad credentials now costs one comparison.
func TestLogin_UnknownAddressCostsAHashComparison(t *testing.T) {
	ctx := context.Background()

	t.Run("unknown address", func(t *testing.T) {
		uc, _, hasher := newThrottleHarness(t)
		_, err := uc.Execute(ctx, LoginInput{Email: "nobody@opendefender.io", Password: "whatever"})
		require.Error(t, err)
		require.Len(t, hasher.verified, 1, "one comparison, as for a real account")
		assert.Equal(t, "hashed:"+loginTimingDummyPassword, hasher.verified[0],
			"compared against a hash made with the live hasher, so it costs the same")
	})

	t.Run("disabled account", func(t *testing.T) {
		uc, users, hasher := newThrottleHarness(t)
		users.user.IsActive = false
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: "Ancre-Vitrail7-Cobalt"})
		require.Error(t, err, "a disabled account is refused even with the right password")
		assert.Equal(t, []string{users.user.Password}, hasher.verified)
	})

	t.Run("wrong password", func(t *testing.T) {
		uc, users, hasher := newThrottleHarness(t)
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: "wrong"})
		require.Error(t, err)
		assert.Len(t, hasher.verified, 1)
	})
}

func TestLogin_Success(t *testing.T) {
	uc, users, _ := newThrottleHarness(t)
	out, err := uc.Execute(context.Background(), LoginInput{Email: users.user.Email, Password: "Ancre-Vitrail7-Cobalt"})
	require.NoError(t, err)
	assert.NotNil(t, out.TokenPair)
}

func TestLogin_NotFound(t *testing.T) {
	uc, _, _ := newThrottleHarness(t)
	_, err := uc.Execute(context.Background(), LoginInput{Email: "nobody@opendefender.io", Password: "x"})
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestLogin_Unauthorized(t *testing.T) {
	uc, users, _ := newThrottleHarness(t)
	_, err := uc.Execute(context.Background(), LoginInput{Email: users.user.Email, Password: "wrong"})
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func failLogin(t *testing.T, uc *LoginUseCase, email string) error {
	t.Helper()
	_, err := uc.Execute(context.Background(), LoginInput{Email: email, Password: "wrong"})
	require.Error(t, err)
	return err
}

// #688 criterion 3: wrong passwords on one address are bounded across every
// source, not per IP.
func TestLogin_PerAccountBackoff(t *testing.T) {
	ctx := context.Background()
	right := "Ancre-Vitrail7-Cobalt"

	t.Run("ten failures lock the address, even against the right password", func(t *testing.T) {
		uc, users, hasher := newThrottleHarness(t)
		uc.WithAttemptLimits(authmfa.NewMemoryAttemptStore())

		for i := 1; i <= LoginMaxFailuresPerAddress; i++ {
			err := failLogin(t, uc, users.user.Email)
			var locked *LoginLockedError
			require.False(t, errors.As(err, &locked), "failure %d is evaluated, not refused", i)
		}

		before := len(hasher.verified)
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right})
		var locked *LoginLockedError
		require.True(t, errors.As(err, &locked), "got %v", err)
		assert.InDelta(t, LoginLockDuration.Seconds(), locked.RetryAfter.Seconds(), 1)
		assert.Len(t, hasher.verified, before, "a refused attempt never reaches the hasher")
	})

	t.Run("the casing of the address does not buy more attempts", func(t *testing.T) {
		uc, users, _ := newThrottleHarness(t)
		uc.WithAttemptLimits(authmfa.NewMemoryAttemptStore())
		for i := 0; i < LoginMaxFailuresPerAddress; i++ {
			failLogin(t, uc, map[bool]string{true: "ADMIN@opendefender.io", false: "admin@OpenDefender.io"}[i%2 == 0])
		}
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right})
		var locked *LoginLockedError
		assert.True(t, errors.As(err, &locked))
	})

	t.Run("an unknown address locks exactly like a real one", func(t *testing.T) {
		uc, _, _ := newThrottleHarness(t)
		uc.WithAttemptLimits(authmfa.NewMemoryAttemptStore())
		for i := 0; i < LoginMaxFailuresPerAddress; i++ {
			failLogin(t, uc, "nobody@opendefender.io")
		}
		var locked *LoginLockedError
		assert.True(t, errors.As(failLogin(t, uc, "nobody@opendefender.io"), &locked),
			"otherwise a lock would tell which addresses hold an account")
	})

	t.Run("a success clears the count", func(t *testing.T) {
		uc, users, _ := newThrottleHarness(t)
		uc.WithAttemptLimits(authmfa.NewMemoryAttemptStore())
		for i := 0; i < LoginMaxFailuresPerAddress-1; i++ {
			failLogin(t, uc, users.user.Email)
		}
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right})
		require.NoError(t, err)
		for i := 0; i < LoginMaxFailuresPerAddress-1; i++ {
			failLogin(t, uc, users.user.Email)
		}
		_, err = uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right})
		assert.NoError(t, err, "nine failures after a success are not ten")
	})

	t.Run("the lock is bounded: refused attempts do not extend it", func(t *testing.T) {
		uc, users, _ := newThrottleHarness(t)
		store := authmfa.NewMemoryAttemptStore()
		uc.WithAttemptLimits(store)
		for i := 0; i < LoginMaxFailuresPerAddress; i++ {
			failLogin(t, uc, users.user.Email)
		}
		first, err := store.LockedFor(ctx, loginLockKey(users.user.Email))
		require.NoError(t, err)
		time.Sleep(1100 * time.Millisecond)
		for i := 0; i < 5; i++ {
			failLogin(t, uc, users.user.Email)
		}
		later, err := store.LockedFor(ctx, loginLockKey(users.user.Email))
		require.NoError(t, err)
		assert.Less(t, later, first, "the lock only runs down")
		assert.LessOrEqual(t, first, LoginLockDuration)
	})
}

// A password reset proves control of the mailbox, which is what the owner of a
// locked address needs to get back in at once.
func TestConfirmPasswordReset_ClearsLoginLock(t *testing.T) {
	ctx := context.Background()
	user := activeUser("real@opendefender.io")
	user.Password = "hashed:old"
	store := authmfa.NewMemoryAttemptStore()

	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	uc, _, _ := newThrottleHarness(t)
	uc.userRepo = &loginUsers{user: user}
	uc.WithAttemptLimits(store)
	for i := 0; i < LoginMaxFailuresPerAddress; i++ {
		failLogin(t, uc, "Real@OpenDefender.io")
	}
	locked, err := store.LockedFor(ctx, loginLockKey(user.Email))
	require.NoError(t, err)
	require.Positive(t, locked)

	secret := issueToken(t, users, tokens, user.Email)
	confirm := NewConfirmPasswordResetUseCase(users, tokens, fakeHasher{}, pwpolicy.New(), &fakeRevoker{}, &fakeMailer{}).
		WithLoginLockClearer(store)
	_, err = confirm.Execute(ctx, ConfirmPasswordResetInput{Token: secret, NewPassword: "Ancre-Vitrail7-Cobalt"})
	require.NoError(t, err)

	locked, err = store.LockedFor(ctx, loginLockKey(user.Email))
	require.NoError(t, err)
	assert.Zero(t, locked)
	n, err := store.Count(ctx, loginFailKey(user.Email))
	require.NoError(t, err)
	assert.Zero(t, n)
}
