// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// rehashUsers records every Update and can be told to fail the first one, which
// is the rehash write (Execute writes LastLogin afterwards).
type rehashUsers struct {
	*loginUsers
	failFirstUpdate bool
	written         []string
}

func (r *rehashUsers) Update(_ context.Context, u *domain.User) error {
	if r.failFirstUpdate && len(r.written) == 0 {
		r.written = append(r.written, "<failed>")
		return errors.New("database unavailable")
	}
	r.written = append(r.written, u.Password)
	return nil
}

const rehashPassword = "Ancre-Vitrail7-Cobalt"

// newRehashHarness stores the password as Argon2id at t=2 and signs in through
// a hasher that now writes t=3: the situation after an operator raises the cost.
func newRehashHarness(t *testing.T) (*LoginUseCase, *rehashUsers) {
	t.Helper()
	uc, users, _, _ := newLoginHarness(t, domain.RoleUser)
	older := coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{
		Time: 2, Memory: 19456, Threads: 1,
	})
	stored, err := older.Hash(rehashPassword)
	require.NoError(t, err)
	users.user.Password = stored

	repo := &rehashUsers{loginUsers: users}
	uc.userRepo = repo
	uc.passwordHasher = coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{
		Time: 3, Memory: 19456, Threads: 1,
	})
	return uc, repo
}

func TestLoginRehash_Success(t *testing.T) {
	uc, repo := newRehashHarness(t)

	out, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: rehashPassword})
	require.NoError(t, err)
	require.NotNil(t, out)

	require.NotEmpty(t, repo.written)
	assert.Contains(t, repo.written[0], "$argon2id$v=19$m=19456,t=3,p=1$", "the first write is the rehash at today's cost")
}

func TestLoginRehash_NotFound(t *testing.T) {
	uc, repo := newRehashHarness(t)

	_, err := uc.Execute(context.Background(), LoginInput{Email: "nobody@opendefender.io", Password: rehashPassword})
	require.Error(t, err)
	assert.Empty(t, repo.written)
}

func TestLoginRehash_Unauthorized(t *testing.T) {
	uc, repo := newRehashHarness(t)

	_, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: "wrong-password"})
	require.Error(t, err)
	assert.Empty(t, repo.written, "no rehash without the password")
}

// D-048: a first-release SHA-256 digest no longer buys a session, even with
// the right password, and nothing is written for it. The account resets.
func TestLogin_LegacySHA256IsRefused(t *testing.T) {
	uc, repo := newRehashHarness(t)
	sum := sha256.Sum256([]byte(rehashPassword))
	repo.user.Password = hex.EncodeToString(sum[:])

	_, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: rehashPassword})
	require.Error(t, err)
	assert.Empty(t, repo.written)
}

// A failed rehash write must cost one more legacy login, not the sign-in — and
// the in-memory row must go back to the stored hash, so the LastLogin write
// that follows cannot smuggle the new hash in behind a failure already logged.
func TestLoginRehash_FailedWriteDoesNotBlockSignIn(t *testing.T) {
	uc, repo := newRehashHarness(t)
	repo.failFirstUpdate = true
	legacy := repo.user.Password

	out, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: rehashPassword})
	require.NoError(t, err)
	require.NotNil(t, out)

	require.GreaterOrEqual(t, len(repo.written), 2, "the LastLogin write must follow the failed rehash")
	for _, w := range repo.written[1:] {
		assert.Equal(t, legacy, w, "later writes must carry the stored hash, not the unpersisted one")
	}
}
