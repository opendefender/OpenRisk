// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// #1032 — new backup codes take a current authenticator code, plus the
// password when the account has one, and replace the old ones all or nothing.

type regenerateMailer struct {
	to, locale string
	calls      int
}

func (m *regenerateMailer) SendMFABackupCodesRegenerated(_ context.Context, to, _, locale string) error {
	m.to, m.locale = to, locale
	m.calls++
	return nil
}

type regenerateFixture struct {
	uc     *RegenerateBackupCodesUseCase
	repo   *MockMFARepository
	mailer *regenerateMailer
	user   *domain.User
	tenant uuid.UUID
	plain  string
	key    string
}

func newRegenerateFixture(t *testing.T) *regenerateFixture {
	t.Helper()
	tenant := uuid.New()
	user := &domain.User{ID: uuid.New(), Email: "rssi@banque.cm", FullName: "R. Ssi", IsActive: true, Password: "hashed:" + disablePassword}
	users := &disableUsers{users: map[uuid.UUID]*domain.User{user.ID: user}}

	plain, err := otp.GenerateTOTPSecret()
	require.NoError(t, err)
	enc, err := crypto.EncryptAES256GCM(plain, disableTOTPKey)
	require.NoError(t, err)

	repo := NewMockMFARepository()
	require.NoError(t, repo.CreateMFASecret(context.Background(), &domain.MFASecret{
		UserID: user.ID, TenantID: tenant, IsVerified: true, SecretEncrypted: enc,
	}))
	require.NoError(t, repo.SaveBackupCodes(context.Background(), []*domain.MFABackupCode{{UserID: user.ID, TenantID: tenant, CodeHash: "old"}}))

	mailer := &regenerateMailer{}
	uc := NewRegenerateBackupCodesUseCase(repo, users, disableHasher{}, disableTOTPKey).WithMailer(mailer)
	return &regenerateFixture{uc: uc, repo: repo, mailer: mailer, user: user, tenant: tenant, plain: plain,
		key: user.ID.String() + ":" + tenant.String()}
}

func (f *regenerateFixture) code(t *testing.T) string {
	t.Helper()
	c, err := totp.GenerateCode(f.plain, time.Now())
	require.NoError(t, err)
	return c
}

func (f *regenerateFixture) run(password, code string) (*RegenerateBackupCodesOutput, error) {
	return f.uc.Execute(context.Background(), RegenerateBackupCodesInput{
		UserID: f.user.ID, TenantID: f.tenant, Password: password, Code: code, Locale: "en",
	})
}

func (f *regenerateFixture) assertOldCodesKept(t *testing.T) {
	t.Helper()
	codes := f.repo.codes[f.key]
	require.Len(t, codes, 1, "the old codes must survive a refused regeneration")
	assert.Equal(t, "old", codes[0].CodeHash)
	assert.Zero(t, f.mailer.calls, "no notice for codes that were not replaced")
}

func TestRegenerateBackupCodes_Success(t *testing.T) {
	f := newRegenerateFixture(t)

	out, err := f.run(disablePassword, f.code(t))
	require.NoError(t, err)
	require.Len(t, out.BackupCodes, 8)

	stored := f.repo.codes[f.key]
	require.Len(t, stored, 8)
	seen := map[string]bool{}
	for i, c := range out.BackupCodes {
		assert.False(t, seen[c], "codes must be unique")
		seen[c] = true
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored[i].CodeHash), []byte(c)), "stored hashed, never in clear")
		assert.NotEqual(t, c, stored[i].CodeHash)
	}
	assert.Equal(t, 1, f.mailer.calls)
	assert.Equal(t, "rssi@banque.cm", f.mailer.to)
	assert.Equal(t, "en", f.mailer.locale)
}

func TestRegenerateBackupCodes_NotFound(t *testing.T) {
	t.Run("no secret", func(t *testing.T) {
		f := newRegenerateFixture(t)
		code := f.code(t)
		require.NoError(t, f.repo.DisableMFA(context.Background(), f.user.ID, f.tenant))

		_, err := f.run(disablePassword, code)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr), "got %v", err)
		assert.ErrorIs(t, appErr.Err, domain.ErrNotFound)
		assert.Zero(t, f.mailer.calls)
	})
	t.Run("unfinished enrolment", func(t *testing.T) {
		f := newRegenerateFixture(t)
		secret, _ := f.repo.GetMFASecret(context.Background(), f.user.ID, f.tenant)
		secret.IsVerified = false

		_, err := f.run(disablePassword, f.code(t))
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr), "got %v", err)
		assert.ErrorIs(t, appErr.Err, domain.ErrNotFound)
		f.assertOldCodesKept(t)
	})
}

func TestRegenerateBackupCodes_Unauthorized(t *testing.T) {
	t.Run("no session", func(t *testing.T) {
		f := newRegenerateFixture(t)
		_, err := f.uc.Execute(context.Background(), RegenerateBackupCodesInput{TenantID: f.tenant, Password: disablePassword, Code: f.code(t)})
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr), "got %v", err)
		assert.ErrorIs(t, appErr.Err, domain.ErrUnauthorized)
		f.assertOldCodesKept(t)
	})
	for name, password := range map[string]string{"wrong password": "guess-1234", "missing password": ""} {
		t.Run(name, func(t *testing.T) {
			f := newRegenerateFixture(t)
			_, err := f.run(password, f.code(t))
			assert.ErrorIs(t, err, ErrBackupCodesPasswordIncorrect)
			f.assertOldCodesKept(t)
		})
	}
}

func TestRegenerateBackupCodes_WrongCode(t *testing.T) {
	for name, code := range map[string]string{"missing": "", "wrong": "not-a-code"} {
		t.Run(name, func(t *testing.T) {
			f := newRegenerateFixture(t)
			_, err := f.run(disablePassword, code)
			assert.ErrorIs(t, err, ErrBackupCodesCodeIncorrect)
			f.assertOldCodesKept(t)
		})
	}
	t.Run("an existing backup code is not the factor", func(t *testing.T) {
		f := newRegenerateFixture(t)
		hash, err := bcrypt.GenerateFromPassword([]byte("ABCD-EFGH"), bcrypt.MinCost)
		require.NoError(t, err)
		f.repo.codes[f.key][0].CodeHash = string(hash)

		_, err = f.run(disablePassword, "ABCD-EFGH")
		assert.ErrorIs(t, err, ErrBackupCodesCodeIncorrect)
		assert.Zero(t, f.mailer.calls)
	})
	t.Run("replayed", func(t *testing.T) {
		f := newRegenerateFixture(t)
		code := f.code(t)
		_, err := f.run(disablePassword, code)
		require.NoError(t, err)

		_, err = f.run(disablePassword, code)
		assert.ErrorIs(t, err, ErrBackupCodesCodeIncorrect)
		assert.Equal(t, 1, f.mailer.calls)
	})
}

func TestRegenerateBackupCodes_RollbackKeepsOldCodes(t *testing.T) {
	f := newRegenerateFixture(t)
	f.repo.replaceErr = errors.New("db down")

	out, err := f.run(disablePassword, f.code(t))
	require.Error(t, err)
	assert.Nil(t, out, "no codes may be shown that were not stored")
	f.assertOldCodesKept(t)
}

func TestRegenerateBackupCodes_AccountWithoutPasswordNeedsOnlyTheCode(t *testing.T) {
	f := newRegenerateFixture(t)
	f.user.Password = ""

	out, err := f.run("", f.code(t))
	require.NoError(t, err)
	assert.Len(t, out.BackupCodes, 8)
}

func TestRegenerateBackupCodes_WithoutKeyRefuses(t *testing.T) {
	f := newRegenerateFixture(t)
	f.uc.totpKey = nil

	_, err := f.run(disablePassword, f.code(t))
	require.Error(t, err)
	f.assertOldCodesKept(t)
}
