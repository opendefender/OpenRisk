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
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// GHSA-45mr-j2w6-m4jv — switching organisation runs the sign-in MFA gate for
// the target organisation. The gate is the real LoginUseCase one.
// ---------------------------------------------------------------------------

// noSecondFactor is a gate with nothing to ask: the switch tests that are not
// about MFA.
type noSecondFactor struct{}

func (noSecondFactor) SecondFactorForOrg(context.Context, uuid.UUID, uuid.UUID, *domain.OrganizationMember) (*SecondFactorOutcome, error) {
	return &SecondFactorOutcome{}, nil
}

type failingSecondFactor struct{}

func (failingSecondFactor) SecondFactorForOrg(context.Context, uuid.UUID, uuid.UUID, *domain.OrganizationMember) (*SecondFactorOutcome, error) {
	return nil, errors.New("mfa store unavailable")
}

// switchIntoB makes the harness user a member of a second organisation, B,
// with the given role, and switches into it.
func switchIntoB(t *testing.T, gate SecondFactorGate, userID, orgB uuid.UUID, role domain.MemberRole) (*SwitchOrganizationOutput, error) {
	t.Helper()
	anchor := loginNow
	stub := &memberStub{byOrg: map[uuid.UUID]*domain.OrganizationMember{
		orgB: {ID: uuid.New(), UserID: userID, OrganizationID: orgB, Role: role, IsActive: true,
			JoinedAt: anchor, MFAGraceStartedAt: &anchor,
			Organization: &domain.Organization{ID: orgB, Name: "Org B", Slug: "org-b"}},
	}}
	uc := NewSwitchOrganizationUseCase(stub, newSwitchTokenManager(t), gate)
	return uc.Execute(context.Background(), SwitchOrganizationInput{UserID: userID, TargetOrgID: orgB})
}

func TestSwitchOrganization_Success_NothingOwedInTarget(t *testing.T) {
	login, users, _, _ := newLoginHarness(t, domain.RoleUser)

	out, err := switchIntoB(t, login, users.user.ID, uuid.New(), domain.RoleUser)
	require.NoError(t, err)
	require.Nil(t, out.SecondFactor)
	require.NotNil(t, out.TokenPair)
}

// The reported bypass: an authenticator enrolled in B only. Signing in to A
// never asks for it, so the switch must.
func TestSwitchOrganization_Unauthorized_TargetAuthenticatorIsChallenged(t *testing.T) {
	login, users, mfaRepo, keys := newLoginHarness(t, domain.RoleUser)
	orgB := uuid.New()
	enrolVerifiedMFA(t, mfaRepo, users.user.ID, orgB)

	out, err := switchIntoB(t, login, users.user.ID, orgB, domain.RoleUser)
	require.NoError(t, err)
	require.Nil(t, out.TokenPair, "no session before the target's factor is proved")
	require.NotNil(t, out.SecondFactor)
	require.Equal(t, SecondFactorChallenge, out.SecondFactor.Step)
	require.Equal(t, "Org B", out.Organization.Name)

	// The challenge token names B: the challenge checks B's secret and issues
	// the session for B.
	claims, err := authpkg.ValidateToken(keys, out.SecondFactor.MFAToken, nil)
	require.NoError(t, err)
	require.Equal(t, authpkg.TokenTypeMFARequired, claims.Type)
	require.Equal(t, orgB, claims.TenantID)
}

// D-066: a factor enrolled in the default organisation says nothing about B.
// B's role policy, past its grace window, still demands enrolment in B.
func TestSwitchOrganization_Unauthorized_FactorOfAnotherOrgDoesNotSatisfyTargetPolicy(t *testing.T) {
	login, users, mfaRepo, keys := newLoginHarness(t, domain.RoleUser)
	enrolVerifiedMFA(t, mfaRepo, users.user.ID, users.org.ID)
	orgB := uuid.New()

	anchor := loginNow.AddDate(0, 0, -30)
	stub := &memberStub{byOrg: map[uuid.UUID]*domain.OrganizationMember{
		orgB: {ID: uuid.New(), UserID: users.user.ID, OrganizationID: orgB, Role: domain.RoleAdmin, IsActive: true,
			JoinedAt: anchor, MFAGraceStartedAt: &anchor},
	}}
	uc := NewSwitchOrganizationUseCase(stub, newSwitchTokenManager(t), login)
	out, err := uc.Execute(context.Background(), SwitchOrganizationInput{UserID: users.user.ID, TargetOrgID: orgB})
	require.NoError(t, err)
	require.Nil(t, out.TokenPair)
	require.NotNil(t, out.SecondFactor)
	require.Equal(t, SecondFactorEnrollment, out.SecondFactor.Step)
	require.True(t, out.SecondFactor.MFAStatus.Required)

	claims, err := authpkg.ValidateToken(keys, out.SecondFactor.MFAToken, nil)
	require.NoError(t, err)
	require.Equal(t, authpkg.TokenTypeMFAEnrollment, claims.Type)
	require.Equal(t, orgB, claims.TenantID)
}

// The gate is asked after the membership check: a non-member learns nothing
// about the target's MFA posture. A missing membership answers Forbidden, like
// a deactivated one, on purpose.
func TestSwitchOrganization_Forbidden_NonMemberIsRefusedBeforeTheGate(t *testing.T) {
	stub := &memberStub{byOrg: map[uuid.UUID]*domain.OrganizationMember{}}
	uc := NewSwitchOrganizationUseCase(stub, newSwitchTokenManager(t), failingSecondFactor{})
	_, err := uc.Execute(context.Background(), SwitchOrganizationInput{UserID: uuid.New(), TargetOrgID: uuid.New()})
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, domain.ErrForbidden, appErr.Err)
}

func TestSwitchOrganization_GateErrorRefusesTheSwitch(t *testing.T) {
	out, err := switchIntoB(t, failingSecondFactor{}, uuid.New(), uuid.New(), domain.RoleUser)
	require.Error(t, err)
	require.Nil(t, out)
}

func TestSwitchOrganization_NoGateRefusesTheSwitch(t *testing.T) {
	out, err := switchIntoB(t, nil, uuid.New(), uuid.New(), domain.RoleUser)
	require.ErrorIs(t, err, errNoSecondFactorGate)
	require.Nil(t, out)
}
