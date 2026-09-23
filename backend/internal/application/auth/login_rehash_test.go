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
	"time"

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

func newRehashHarness(t *testing.T, cutoff time.Time) (*LoginUseCase, *rehashUsers) {
	t.Helper()
	uc, users, _, _ := newLoginHarness(t, domain.RoleUser)
	sum := sha256.Sum256([]byte(rehashPassword))
	users.user.Password = hex.EncodeToString(sum[:])

	repo := &rehashUsers{loginUsers: users}
	uc.userRepo = repo
	uc.passwordHasher = coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{
		Time: 2, Memory: 19456, Threads: 1,
	})
	uc.WithLegacyHashCutoff(cutoff)
	return uc, repo
}

func TestLoginRehash_Success(t *testing.T) {
	uc, repo := newRehashHarness(t, loginNow.Add(24*time.Hour))

	out, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: rehashPassword})
	require.NoError(t, err)
	require.NotNil(t, out)

	require.NotEmpty(t, repo.written)
	assert.Equal(t, coreauth.AlgorithmArgon2id, coreauth.HashAlgorithm(repo.written[0]), "the first write is the Argon2id rehash")
}

func TestLoginRehash_NotFound(t *testing.T) {
	uc, repo := newRehashHarness(t, loginNow.Add(24*time.Hour))

	_, err := uc.Execute(context.Background(), LoginInput{Email: "nobody@opendefender.io", Password: rehashPassword})
	require.Error(t, err)
	assert.Empty(t, repo.written)
}

func TestLoginRehash_Unauthorized(t *testing.T) {
	uc, repo := newRehashHarness(t, loginNow.Add(24*time.Hour))

	_, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: "wrong-password"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrPasswordResetRequired))
	assert.Empty(t, repo.written, "no rehash without the password")
}

func TestLoginRehash_ExpiredLegacyHashIsRefused(t *testing.T) {
	uc, repo := newRehashHarness(t, loginNow.Add(-time.Second))

	_, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: rehashPassword})
	assert.ErrorIs(t, err, ErrPasswordResetRequired)
	assert.Empty(t, repo.written)
}

// A failed rehash write must cost one more legacy login, not the sign-in — and
// the in-memory row must go back to the stored hash, so the LastLogin write
// that follows cannot smuggle the new hash in behind a failure already logged.
func TestLoginRehash_FailedWriteDoesNotBlockSignIn(t *testing.T) {
	uc, repo := newRehashHarness(t, loginNow.Add(24*time.Hour))
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
