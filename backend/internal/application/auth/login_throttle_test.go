// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
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
