// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// GHSA-rv5c-pr27-gpxh — the SSO gate is the password gate: same decisions, same
// tokens, for a user whose first factor an identity provider proved.

func TestSSOSecondFactor_Success(t *testing.T) {
	uc, users, mfaRepo, keys := newLoginHarness(t, domain.RoleAdmin)
	enrolVerifiedMFA(t, mfaRepo, users.user.ID, users.org.ID)

	out, err := uc.SSOSecondFactor(context.Background(), users.user.ID)
	require.NoError(t, err)
	require.Equal(t, SecondFactorChallenge, out.Step, "an enrolled authenticator must be asked for after SSO")

	claims, err := authpkg.ValidateToken(keys, out.MFAToken, nil)
	require.NoError(t, err)
	assert.Equal(t, authpkg.TokenTypeMFARequired, claims.Type)
	assert.Equal(t, users.org.ID, claims.TenantID)
	assert.Empty(t, claims.Permissions)
}

func TestSSOSecondFactor_RolePolicyPastGrace(t *testing.T) {
	uc, users, _, keys := newLoginHarness(t, domain.RoleAdmin)
	backdateGrace(users, 30)

	out, err := uc.SSOSecondFactor(context.Background(), users.user.ID)
	require.NoError(t, err)
	require.Equal(t, SecondFactorEnrollment, out.Step)
	assert.True(t, out.MFAStatus.Required)

	claims, err := authpkg.ValidateToken(keys, out.MFAToken, nil)
	require.NoError(t, err)
	assert.Equal(t, authpkg.TokenTypeMFAEnrollment, claims.Type)
}

func TestSSOSecondFactor_OrdinaryMemberWithoutFactorSignsIn(t *testing.T) {
	uc, users, _, _ := newLoginHarness(t, domain.RoleUser)
	backdateGrace(users, 30)

	out, err := uc.SSOSecondFactor(context.Background(), users.user.ID)
	require.NoError(t, err)
	assert.Equal(t, SecondFactorNone, out.Step)
	assert.Empty(t, out.MFAToken)
}

// Same decision as /auth/login for the same account, whichever factor is set.
func TestSSOSecondFactor_MatchesPasswordLogin(t *testing.T) {
	for name, setup := range map[string]func(*loginUsers, *MockMFARepository){
		"enrolled":     func(u *loginUsers, m *MockMFARepository) { enrolVerifiedMFA(t, m, u.user.ID, u.org.ID) },
		"past grace":   func(u *loginUsers, _ *MockMFARepository) { backdateGrace(u, 30) },
		"inside grace": func(*loginUsers, *MockMFARepository) {},
	} {
		t.Run(name, func(t *testing.T) {
			uc, users, mfaRepo, _ := newLoginHarness(t, domain.RoleAdmin)
			setup(users, mfaRepo)

			pw := login(t, uc)
			sso, err := uc.SSOSecondFactor(context.Background(), users.user.ID)
			require.NoError(t, err)
			assert.Equal(t, pw.MFARequired, sso.Step == SecondFactorChallenge)
			assert.Equal(t, pw.MFAEnrollmentRequired, sso.Step == SecondFactorEnrollment)
			assert.Equal(t, pw.MFAStatus.State, sso.MFAStatus.State)
		})
	}
}

func TestSSOSecondFactor_NotFound(t *testing.T) {
	uc, users, _, _ := newLoginHarness(t, domain.RoleAdmin)
	users.org = nil

	_, err := uc.SSOSecondFactor(context.Background(), users.user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation, "no organisation is a typed refusal")
}

// A revoked membership is refused before any code is asked for.
func TestSSOSecondFactor_Unauthorized(t *testing.T) {
	uc, users, mfaRepo, _ := newLoginHarness(t, domain.RoleAdmin)
	enrolVerifiedMFA(t, mfaRepo, users.user.ID, users.org.ID)
	users.member.IsActive = false

	out, err := uc.SSOSecondFactor(context.Background(), users.user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
	assert.Nil(t, out, "no MFA token for an organisation that withdrew access")
}

// A store that cannot answer must refuse the sign-in, never wave it through.
func TestSSOSecondFactor_StoreFailureRefuses(t *testing.T) {
	uc, users, mfaRepo, _ := newLoginHarness(t, domain.RoleAdmin)
	uc.WithMFA(failingMFASecrets{mfaRepo})

	out, err := uc.SSOSecondFactor(context.Background(), users.user.ID)
	require.Error(t, err)
	assert.Nil(t, out)
}

// failingMFASecrets is a store that cannot read secrets.
type failingMFASecrets struct{ *MockMFARepository }

func (failingMFASecrets) GetMFASecret(context.Context, uuid.UUID, uuid.UUID) (*domain.MFASecret, error) {
	return nil, errors.New("mfa store down")
}
