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

// stubMembers is an MFAMemberLookup over one membership, counting reads so the
// cache can be observed rather than assumed.
type stubMembers struct {
	member *domain.OrganizationMember
	err    error
	calls  int
}

func (s *stubMembers) GetOrganizationMember(context.Context, uuid.UUID, uuid.UUID) (*domain.OrganizationMember, error) {
	s.calls++
	return s.member, s.err
}

type resolverFixture struct {
	resolver *MFAStatusResolver
	mfa      *MockMFARepository
	members  *stubMembers
	user     uuid.UUID
	tenant   uuid.UUID
}

func newResolverFixture(t *testing.T, role domain.MemberRole, business domain.BusinessRoleKey, anchorDaysAgo int) *resolverFixture {
	t.Helper()

	user, tenant := uuid.New(), uuid.New()
	anchor := loginNow.Add(-time.Duration(anchorDaysAgo) * 24 * time.Hour)
	members := &stubMembers{member: &domain.OrganizationMember{
		ID: uuid.New(), UserID: user, OrganizationID: tenant,
		Role: role, BusinessRole: business, IsActive: true,
		JoinedAt: anchor, MFAGraceStartedAt: &anchor,
	}}
	mfa := NewMockMFARepository()

	orgRoles, businessRoles := domain.DefaultMFAPrivilegeRoles()
	r := NewMFAStatusResolver(mfa, members, orgRoles, businessRoles).
		WithClock(func() time.Time { return loginNow })

	return &resolverFixture{resolver: r, mfa: mfa, members: members, user: user, tenant: tenant}
}

func (f *resolverFixture) resolve(t *testing.T, hint string) domain.MFADecision {
	t.Helper()
	d, err := f.resolver.Resolve(context.Background(), f.user, f.tenant, hint)
	require.NoError(t, err)
	return d
}

func TestResolver_StandardMemberIsOnlyEverRecommended(t *testing.T) {
	f := newResolverFixture(t, domain.RoleUser, "", 3650)
	assert.Equal(t, domain.MFAStateRecommended, f.resolve(t, "user").State)
}

func TestResolver_PrivilegedInsideAndOutsideTheWindow(t *testing.T) {
	inside := newResolverFixture(t, domain.RoleAdmin, "", 2)
	d := inside.resolve(t, "admin")
	assert.Equal(t, domain.MFAStateGraceActive, d.State)
	assert.False(t, d.Required)

	outside := newResolverFixture(t, domain.RoleAdmin, "", 9)
	d = outside.resolve(t, "admin")
	assert.Equal(t, domain.MFAStateRequired, d.State)
	assert.True(t, d.Required)
}

func TestResolver_EnrolledIsConfigured(t *testing.T) {
	f := newResolverFixture(t, domain.RoleAdmin, "", 9)
	require.NoError(t, f.mfa.CreateMFASecret(context.Background(), &domain.MFASecret{
		ID: uuid.New(), UserID: f.user, TenantID: f.tenant, SecretEncrypted: "x", IsVerified: true,
	}))

	d := f.resolve(t, "admin")
	assert.Equal(t, domain.MFAStateConfigured, d.State)
	assert.False(t, d.Required, "an enrolled admin is never blocked, however old the account")
}

func TestResolver_UnverifiedSecretIsNotEnrolment(t *testing.T) {
	f := newResolverFixture(t, domain.RoleAdmin, "", 9)
	require.NoError(t, f.mfa.CreateMFASecret(context.Background(), &domain.MFASecret{
		ID: uuid.New(), UserID: f.user, TenantID: f.tenant, SecretEncrypted: "x", IsVerified: false,
	}))
	assert.True(t, f.resolve(t, "admin").Required, "a half-finished enrolment is not protection")
}

func TestResolver_TokenHintWidensButNeverNarrows(t *testing.T) {
	// A missing membership row must not exempt an administrator whose signed
	// token says they are one. The anchor is then unknown, which fails closed.
	f := newResolverFixture(t, domain.RoleUser, "", 1)
	f.members.member = nil

	assert.True(t, f.resolve(t, "admin").Required,
		"the signed token widens the privileged set when the membership row is gone")

	// And the reverse: a token claiming 'user' cannot talk an admin membership
	// out of the requirement.
	g := newResolverFixture(t, domain.RoleAdmin, "", 9)
	assert.True(t, g.resolve(t, "user").Required,
		"the membership row decides; the hint can only add")
}

func TestResolver_RSSIIsPrivilegedWithoutAnAdminOrgRole(t *testing.T) {
	f := newResolverFixture(t, domain.RoleUser, domain.BusinessRoleRSSI, 9)
	assert.True(t, f.resolve(t, "user").Required)
}

func TestResolver_CachesPerUserAndTenant(t *testing.T) {
	// Three lookups per authenticated request would be the wrong trade for a
	// guard that runs on every route.
	f := newResolverFixture(t, domain.RoleAdmin, "", 2)
	f.resolve(t, "admin")
	f.resolve(t, "admin")
	f.resolve(t, "admin")
	assert.Equal(t, 1, f.members.calls, "the decision is resolved once per TTL")

	// A different tenant is a different question — a shared entry would let one
	// tenant's policy decide another tenant's access.
	_, err := f.resolver.Resolve(context.Background(), f.user, uuid.New(), "admin")
	require.NoError(t, err)
	assert.Equal(t, 2, f.members.calls)
}

func TestResolver_InvalidateMakesEnrolmentTakeEffectAtOnce(t *testing.T) {
	f := newResolverFixture(t, domain.RoleAdmin, "", 9)
	assert.True(t, f.resolve(t, "admin").Required)

	require.NoError(t, f.mfa.CreateMFASecret(context.Background(), &domain.MFASecret{
		ID: uuid.New(), UserID: f.user, TenantID: f.tenant, SecretEncrypted: "x", IsVerified: true,
	}))
	assert.True(t, f.resolve(t, "admin").Required, "still cached — that is the TTL doing its job")

	f.resolver.Invalidate(f.user, f.tenant)
	assert.Equal(t, domain.MFAStateConfigured, f.resolve(t, "admin").State,
		"scanning the QR code must unblock the very next request")
}

func TestResolver_InvalidateTenantDropsEveryMemberOfThatTenantOnly(t *testing.T) {
	f := newResolverFixture(t, domain.RoleAdmin, "", 2)
	other := uuid.New()
	f.resolve(t, "admin")
	_, err := f.resolver.Resolve(context.Background(), f.user, other, "admin")
	require.NoError(t, err)
	require.Equal(t, 2, f.members.calls)

	f.resolver.InvalidateTenant(f.tenant)

	f.resolve(t, "admin")
	assert.Equal(t, 3, f.members.calls, "this tenant's entry was dropped")

	_, err = f.resolver.Resolve(context.Background(), f.user, other, "admin")
	require.NoError(t, err)
	assert.Equal(t, 3, f.members.calls, "the other tenant's entry survived — policies do not leak")
}

func TestResolver_PolicyErrorFallsBackToTheDefaultWindow(t *testing.T) {
	f := newResolverFixture(t, domain.RoleAdmin, "", 9)
	f.resolver.WithPolicies(stubMFAPolicies{err: errors.New("database down")})
	d := f.resolve(t, "admin")
	assert.True(t, d.Required)
	assert.Equal(t, domain.MFAGraceDaysDefault, d.GraceDays)
}

func TestResolver_TenantPolicyIsHonoured(t *testing.T) {
	f := newResolverFixture(t, domain.RoleAdmin, "", 2)
	f.resolver.WithPolicies(stubMFAPolicies{policy: &domain.MFAPolicy{GraceDays: 1}})
	d := f.resolve(t, "admin")
	assert.True(t, d.Required, "one-day window, two days elapsed")
	assert.Equal(t, 1, d.GraceDays)
}

func TestResolver_MembershipReadErrorIsReportedNotSwallowed(t *testing.T) {
	// The caller has to be able to tell "not required" from "could not tell",
	// because the guard treats the second as blocking.
	f := newResolverFixture(t, domain.RoleAdmin, "", 2)
	f.members.err = errors.New("database down")
	_, err := f.resolver.Resolve(context.Background(), f.user, f.tenant, "admin")
	require.Error(t, err)
}

func TestResolver_RejectsAnIncompleteIdentity(t *testing.T) {
	f := newResolverFixture(t, domain.RoleAdmin, "", 2)
	_, err := f.resolver.Resolve(context.Background(), uuid.Nil, f.tenant, "admin")
	require.Error(t, err)
	_, err = f.resolver.Resolve(context.Background(), f.user, uuid.Nil, "admin")
	require.Error(t, err)
}
