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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #807 — the organization login falls back to when the default one withdrew
// access. The end-to-end proof (a real membership write in A, a real sign-in
// landing in B) is handler.TestCrossTenant_AdminOfAWithdrawingAccessLeavesBUntouched.

// listingUsers is loginUsers plus the optional membership listing.
type listingUsers struct {
	loginUsers
	active []*domain.OrganizationMember
	err    error
}

func (l *listingUsers) ListActiveMemberships(context.Context, uuid.UUID) ([]*domain.OrganizationMember, error) {
	return l.active, l.err
}

func activeIn(org *domain.Organization, joined time.Time) *domain.OrganizationMember {
	return &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: org.ID, Organization: org,
		Role: domain.RoleUser, IsActive: true, JoinedAt: joined,
	}
}

func TestLoginFallback_Success_PicksEarliestActiveOtherOrganization(t *testing.T) {
	def := &domain.Organization{ID: uuid.New(), IsActive: true}
	older := &domain.Organization{ID: uuid.New(), IsActive: true}
	newer := &domain.Organization{ID: uuid.New(), IsActive: true}
	closed := &domain.Organization{ID: uuid.New(), IsActive: false}

	repo := &listingUsers{active: []*domain.OrganizationMember{
		activeIn(def, loginNow.Add(-72*time.Hour)), // the default itself is never chosen
		activeIn(newer, loginNow.Add(-1*time.Hour)),
		activeIn(closed, loginNow.Add(-96*time.Hour)), // an inactive organization is skipped
		activeIn(older, loginNow.Add(-48*time.Hour)),
	}}
	uc := &LoginUseCase{userRepo: repo}

	got, err := uc.fallbackMembership(context.Background(), uuid.New(), def.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, older.ID, got.OrganizationID)
}

func TestLoginFallback_NotFound_NoOtherActiveMembership(t *testing.T) {
	def := &domain.Organization{ID: uuid.New(), IsActive: true}
	uc := &LoginUseCase{userRepo: &listingUsers{active: []*domain.OrganizationMember{activeIn(def, loginNow)}}}

	got, err := uc.fallbackMembership(context.Background(), uuid.New(), def.ID)
	require.NoError(t, err)
	assert.Nil(t, got)

	// A repository that cannot list memberships keeps the old refusal.
	uc = &LoginUseCase{userRepo: &loginUsers{}}
	got, err = uc.fallbackMembership(context.Background(), uuid.New(), def.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLoginFallback_Unauthorized_WithdrawnDefaultWithNowhereElseIsRefused(t *testing.T) {
	uc, users, _, _ := newLoginHarness(t, domain.RoleUser)
	users.member.IsActive = false
	users.member.Status = domain.MembershipRevoked

	out, err := uc.Execute(context.Background(), LoginInput{
		Email: "admin@opendefender.io", Password: "Ancre-Vitrail7-Cobalt",
	})
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "revoked")
}

func TestLoginFallback_ListingErrorIsNotASession(t *testing.T) {
	uc := &LoginUseCase{userRepo: &listingUsers{err: errors.New("db down")}}
	got, err := uc.fallbackMembership(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Nil(t, got)
}
