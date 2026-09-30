// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// #849 — a TOTP code is accepted once, wherever it is presented.

var replayKey = []byte("0123456789abcdef0123456789abcdef")

type replayFixture struct {
	repo         *MockMFARepository
	user, tenant uuid.UUID
	plain        string
}

func newReplayFixture(t *testing.T, verified bool) *replayFixture {
	t.Helper()
	plain, err := otp.GenerateTOTPSecret()
	require.NoError(t, err)
	enc, err := crypto.EncryptAES256GCM(plain, replayKey)
	require.NoError(t, err)
	f := &replayFixture{repo: NewMockMFARepository(), user: uuid.New(), tenant: uuid.New(), plain: plain}
	require.NoError(t, f.repo.CreateMFASecret(context.Background(), &domain.MFASecret{
		UserID: f.user, TenantID: f.tenant, SecretEncrypted: enc, IsVerified: verified,
	}))
	return f
}

// codeAt returns the code for the step `offset` steps from now.
func (f *replayFixture) codeAt(t *testing.T, offset int) string {
	t.Helper()
	c, err := totp.GenerateCode(f.plain, time.Now().Add(time.Duration(offset)*30*time.Second))
	require.NoError(t, err)
	return c
}

func (f *replayFixture) challenge(code string) error {
	_, err := NewChallengeMFAUseCase(f.repo, replayKey).Execute(context.Background(),
		ChallengeMFAInput{UserID: f.user, TenantID: f.tenant, Code: code})
	return err
}

func TestChallengeMFA_ReplayedCodeIsRefused(t *testing.T) {
	f := newReplayFixture(t, true)
	code := f.codeAt(t, 0)

	require.NoError(t, f.challenge(code), "the first use opens the session")
	err := f.challenge(code)

	require.Error(t, err, "the same code must not open a second session")
	// Refused exactly like a wrong code: no hint that it was once valid.
	wrong := f.challenge("000000")
	if wrong != nil {
		assert.Equal(t, wrong.Error(), err.Error())
	}
}

func TestChallengeMFA_CodeFromAnEarlierStepIsRefused(t *testing.T) {
	f := newReplayFixture(t, true)
	require.NoError(t, f.challenge(f.codeAt(t, 0)))

	// The previous step's code is still inside the ±30 s window, but older than
	// the one just accepted.
	assert.Error(t, f.challenge(f.codeAt(t, -1)))
}

func TestChallengeMFA_ANewerCodeIsStillAccepted(t *testing.T) {
	f := newReplayFixture(t, true)
	require.NoError(t, f.challenge(f.codeAt(t, 0)))

	assert.NoError(t, f.challenge(f.codeAt(t, 1)), "the next step's code is fresh")
}

func TestVerifyMFA_EnrolmentCodeCannotThenOpenALogin(t *testing.T) {
	f := newReplayFixture(t, false)
	code := f.codeAt(t, 0)

	_, err := NewVerifyMFAUseCase(f.repo, repository.GormUserRepository{}, replayKey).Execute(context.Background(),
		VerifyMFAInput{UserID: f.user, TenantID: f.tenant, Code: code})
	require.NoError(t, err)

	assert.Error(t, f.challenge(code))
}

func TestDisableMFA_ReplayedCodeIsRefused(t *testing.T) {
	f := newDisableFixture(t, domain.RoleUser, "")
	plain := f.ssoAccount(t)
	code, err := totp.GenerateCode(plain, time.Now())
	require.NoError(t, err)

	// The code was just used to sign in...
	_, err = NewChallengeMFAUseCase(f.repo, disableTOTPKey).Execute(context.Background(),
		ChallengeMFAInput{UserID: f.user.ID, TenantID: f.tenant, Code: code})
	require.NoError(t, err)

	// ...so someone who watched it typed cannot turn MFA off with it.
	assert.ErrorIs(t, f.runWithCode("", code), ErrMFADisableCodeIncorrect)
	f.assertStillEnrolled(t)
}
