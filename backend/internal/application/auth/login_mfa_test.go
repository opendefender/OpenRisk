// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// loginUsers is a UserRepository for the login use case.
type loginUsers struct {
	user   *domain.User
	org    *domain.Organization
	member *domain.OrganizationMember
}

func (l *loginUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if l.user != nil && domain.NormaliseEmail(l.user.Email) == domain.NormaliseEmail(email) {
		return l.user, nil
	}
	return nil, nil
}
func (l *loginUsers) GetByUsername(context.Context, string) (*domain.User, error) { return nil, nil }
func (l *loginUsers) GetUserDefaultOrganization(context.Context, uuid.UUID) (*domain.Organization, error) {
	return l.org, nil
}
func (l *loginUsers) GetOrganizationMember(context.Context, uuid.UUID, uuid.UUID) (*domain.OrganizationMember, error) {
	return l.member, nil
}
func (l *loginUsers) Create(context.Context, *domain.User) error { return nil }
func (l *loginUsers) Update(context.Context, *domain.User) error { return nil }
func (l *loginUsers) CreateOrganizationMember(context.Context, *domain.OrganizationMember) error {
	return nil
}

// stubMFAPolicies is a MFAPolicyReader returning one fixed policy (or an error,
// to prove a failed lookup does not become a bypass).
type stubMFAPolicies struct {
	policy *domain.MFAPolicy
	err    error
}

func (s stubMFAPolicies) GetMFAPolicy(context.Context, uuid.UUID) (*domain.MFAPolicy, error) {
	return s.policy, s.err
}

// loginNow is the fixed instant the grace arithmetic is evaluated at.
var loginNow = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

// newLoginHarness builds a login use case over an in-memory refresh-token store
// and a real TokenManager, so the tokens it mints can actually be parsed.
//
// The membership is anchored at loginNow so a privileged account starts with its
// full window ahead of it; tests that care about the deadline move the anchor.
func newLoginHarness(t *testing.T, role domain.MemberRole) (*LoginUseCase, *loginUsers, *MockMFARepository, *authpkg.RSAKeys) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT,
			ip_address TEXT,
			user_agent TEXT,
			expires_at DATETIME NOT NULL,
			rotated_at DATETIME,
			last_used_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`).Error)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}

	tm := coreauth.NewTokenManager(db, keys)

	org := &domain.Organization{ID: uuid.New(), Name: "Acme", IsActive: true}
	user := &domain.User{
		ID: uuid.New(), Email: "admin@opendefender.io", Username: "admin",
		FullName: "Admin Person", IsActive: true,
		Password: "hashed:Ancre-Vitrail7-Cobalt", DefaultOrgID: &org.ID,
	}
	anchor := loginNow
	member := &domain.OrganizationMember{
		ID: uuid.New(), UserID: user.ID, OrganizationID: org.ID, Role: role, IsActive: true,
		JoinedAt: anchor, MFAGraceStartedAt: &anchor,
	}

	users := &loginUsers{user: user, org: org, member: member}
	mfaRepo := NewMockMFARepository()

	orgRoles, businessRoles := domain.DefaultMFAPrivilegeRoles()
	uc := NewLoginUseCase(users, tm, fakeHasher{}).
		WithMFA(mfaRepo).
		RequireMFAForRoles(orgRoles, businessRoles).
		WithClock(func() time.Time { return loginNow })

	return uc, users, mfaRepo, keys
}

// backdateGrace moves the member's anchor `days` into the past.
func backdateGrace(users *loginUsers, days int) {
	at := loginNow.Add(-time.Duration(days) * 24 * time.Hour)
	users.member.MFAGraceStartedAt = &at
	users.member.JoinedAt = at
}

func enrolVerifiedMFA(t *testing.T, repo *MockMFARepository, userID, tenantID uuid.UUID) {
	t.Helper()
	require.NoError(t, repo.CreateMFASecret(context.Background(), &domain.MFASecret{
		ID: uuid.New(), UserID: userID, TenantID: tenantID,
		SecretEncrypted: "encrypted", IsVerified: true,
	}))
}

func login(t *testing.T, uc *LoginUseCase) *LoginOutput {
	t.Helper()
	out, err := uc.Execute(context.Background(), LoginInput{
		Email: "admin@opendefender.io", Password: "Ancre-Vitrail7-Cobalt",
	})
	require.NoError(t, err)
	return out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLogin_AdminWithinGracePeriodGetsASession(t *testing.T) {
	// OR26-03. A fresh administrator — the person evaluating the product — reaches
	// the dashboard. The requirement is still mandatory, just not instantaneous.
	uc, _, _, _ := newLoginHarness(t, domain.MemberRole("admin"))

	out := login(t, uc)

	assert.False(t, out.MFAEnrollmentRequired, "a QR code before the first screen is a wall, not a gate")
	require.NotNil(t, out.TokenPair, "the grace window means a real session")
	assert.Equal(t, domain.MFAStateGraceActive, out.MFAStatus.State)
	assert.True(t, out.MFAStatus.Privileged)
	assert.True(t, out.MFAStatus.GraceActive)
	require.NotNil(t, out.MFAStatus.Deadline)
	assert.Equal(t, loginNow.Add(7*24*time.Hour), *out.MFAStatus.Deadline,
		"the default window is seven days")
}

func TestLogin_AdminPastTheGracePeriodGetsNoSession(t *testing.T) {
	// Invariant 3: a privileged account cannot stay without MFA indefinitely.
	uc, users, _, keys := newLoginHarness(t, domain.MemberRole("admin"))
	backdateGrace(users, 8)

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired, "an admin with no authenticator must be stopped at enrolment")
	assert.Nil(t, out.TokenPair, "no session may be issued before MFA is enrolled")
	require.NotEmpty(t, out.MFAToken)

	// The token must be an ENROLMENT token, not a challenge token: only the
	// enrolment endpoints accept it, and it is minted solely for accounts with no
	// verified secret, so it cannot overwrite an existing authenticator.
	claims, err := authpkg.ValidateAccessToken(keys, out.MFAToken, nil)
	require.NoError(t, err)
	assert.Equal(t, authpkg.TokenTypeMFAEnrollment, claims.Type)
	assert.Equal(t, users.user.ID, claims.Sub)
	assert.Empty(t, claims.Permissions, "an enrolment token must carry no permissions")
}

func TestLogin_RootPastTheGracePeriodGetsNoSession(t *testing.T) {
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("root"))
	backdateGrace(users, 8)

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired)
	assert.Nil(t, out.TokenPair)
	assert.Equal(t, domain.MFAStateRequired, out.MFAStatus.State)
}

func TestLogin_RSSIIsPrivilegedThroughTheBusinessRole(t *testing.T) {
	// The issue names "Admin/RSSI". The security officer holds org role `user`,
	// so a check on the org role alone would have exempted exactly the account
	// the requirement is written for.
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("user"))
	users.member.BusinessRole = domain.BusinessRoleRSSI

	out := login(t, uc)
	assert.False(t, out.MFAEnrollmentRequired, "inside the window the RSSI still works")
	assert.True(t, out.MFAStatus.Privileged)
	assert.Equal(t, domain.MFAStateGraceActive, out.MFAStatus.State)

	backdateGrace(users, 8)
	out = login(t, uc)
	assert.True(t, out.MFAEnrollmentRequired, "past the window the RSSI is stopped like any admin")
	assert.Nil(t, out.TokenPair)
}

func TestLogin_GraceEscalatesInTheFinalStretch(t *testing.T) {
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	backdateGrace(users, 6) // 24h left

	out := login(t, uc)

	require.NotNil(t, out.TokenPair, "escalation is louder copy, not a lockout")
	assert.Equal(t, domain.MFAStateGraceExpiring, out.MFAStatus.State)
}

func TestLogin_TenantPolicyOverridesTheDefaultWindow(t *testing.T) {
	// A tenant that sets one day must lock its administrators out on day two,
	// even though the shipped default would still have four days to run.
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	uc.WithMFAPolicies(stubMFAPolicies{policy: &domain.MFAPolicy{GraceDays: 1}})
	backdateGrace(users, 3)

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired)
	assert.Equal(t, 1, out.MFAStatus.GraceDays)
}

func TestLogin_ZeroDayPolicyRequiresEnrolmentImmediately(t *testing.T) {
	// The pre-OR26-03 behaviour, still available to deployments that want it.
	uc, _, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	uc.WithMFAPolicies(stubMFAPolicies{policy: &domain.MFAPolicy{GraceDays: 0}})

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired)
	assert.Nil(t, out.TokenPair)
}

func TestLogin_FailedPolicyLookupFallsBackToTheDefaultNotToNoRequirement(t *testing.T) {
	// A policy read that errors must never be the thing that lets a privileged
	// account past its deadline. The fallback is seven days, not forever.
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	uc.WithMFAPolicies(stubMFAPolicies{err: errors.New("database down")})
	backdateGrace(users, 30)

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired)
	assert.Equal(t, domain.MFAGraceDaysDefault, out.MFAStatus.GraceDays)
}

func TestLogin_OutOfBandPolicyValueIsClamped(t *testing.T) {
	// A row edited straight in SQL must not be able to express "never".
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	uc.WithMFAPolicies(stubMFAPolicies{policy: &domain.MFAPolicy{GraceDays: 100000}})
	backdateGrace(users, 365)

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired, "clamped to the 90-day ceiling, which a year has passed")
}

func TestLogin_MissingAnchorFailsClosedForAPrivilegedAccount(t *testing.T) {
	// A NULL anchor on a privileged membership is the state someone would try to
	// produce to buy unlimited grace. It reads as "enrol now".
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	users.member.MFAGraceStartedAt = nil
	users.member.JoinedAt = time.Time{}
	users.member.CreatedAt = time.Time{}

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired)
	assert.Nil(t, out.TokenPair)
}

func TestLogin_OrdinaryMemberWithoutMFASignsInNormally(t *testing.T) {
	// Mandating MFA for everyone in a product sold into organisations that may not
	// all have authenticators is how you get shared accounts. Members keep it
	// optional.
	uc, _, _, _ := newLoginHarness(t, domain.MemberRole("user"))

	out := login(t, uc)

	assert.False(t, out.MFAEnrollmentRequired)
	assert.False(t, out.MFARequired)
	require.NotNil(t, out.TokenPair, "a member without MFA should get a session")
	assert.NotEmpty(t, out.TokenPair.AccessToken)
	assert.Equal(t, domain.MFAStateRecommended, out.MFAStatus.State)
	assert.False(t, out.MFAStatus.Privileged)
	assert.Nil(t, out.MFAStatus.Deadline)
}

func TestLogin_OrdinaryMemberIsNeverRequiredHoweverOldTheAccount(t *testing.T) {
	// The deadline belongs to privileged accounts. An ordinary member who has
	// been around for a decade still only ever sees a banner.
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("user"))
	backdateGrace(users, 3650)

	out := login(t, uc)

	require.NotNil(t, out.TokenPair)
	assert.Equal(t, domain.MFAStateRecommended, out.MFAStatus.State)
}

func TestLogin_DeactivatedMembershipIsRejected(t *testing.T) {
	// A user removed from an organization keeps their account but loses access to
	// that org: a password must stop granting a session there. No token is issued.
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("user"))
	users.member.IsActive = false

	out, err := uc.Execute(context.Background(), LoginInput{
		Email:    users.user.Email,
		Password: "Ancre-Vitrail7-Cobalt",
	})
	require.Error(t, err)
	require.Nil(t, out)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Err)
}

func TestLogin_AdminWithMFAGetsAChallengeNotAnEnrolment(t *testing.T) {
	// Once enrolled, the admin takes the ordinary second-factor path. Issuing an
	// enrolment token here would be the bypass: it would let a stolen password
	// register a fresh authenticator over the real one.
	uc, users, mfaRepo, keys := newLoginHarness(t, domain.MemberRole("admin"))
	enrolVerifiedMFA(t, mfaRepo, users.user.ID, users.org.ID)

	out := login(t, uc)

	assert.True(t, out.MFARequired, "expected the challenge path")
	assert.False(t, out.MFAEnrollmentRequired, "an enrolled admin must never get an enrolment token")
	assert.Nil(t, out.TokenPair)

	claims, err := authpkg.ValidateAccessToken(keys, out.MFAToken, nil)
	require.NoError(t, err)
	assert.Equal(t, authpkg.TokenTypeMFARequired, claims.Type,
		"an enrolled account must get MFA_REQUIRED, never MFA_ENROLLMENT")
}

func TestLogin_MemberWithMFAStillGetsChallenged(t *testing.T) {
	// Voluntary enrolment must be honoured: a member who turned MFA on is
	// challenged like anyone else.
	uc, users, mfaRepo, _ := newLoginHarness(t, domain.MemberRole("user"))
	enrolVerifiedMFA(t, mfaRepo, users.user.ID, users.org.ID)

	out := login(t, uc)

	assert.True(t, out.MFARequired)
	assert.Nil(t, out.TokenPair)
}

func TestLogin_UnverifiedSecretDoesNotCountAsEnrolled(t *testing.T) {
	// A half-finished enrolment (secret generated, code never confirmed) is not
	// protection. The admin must be sent back to finish it, not challenged for a
	// code they cannot produce — which would lock them out entirely.
	uc, users, mfaRepo, _ := newLoginHarness(t, domain.MemberRole("admin"))
	backdateGrace(users, 8)
	require.NoError(t, mfaRepo.CreateMFASecret(context.Background(), &domain.MFASecret{
		ID: uuid.New(), UserID: users.user.ID, TenantID: users.org.ID,
		SecretEncrypted: "encrypted", IsVerified: false,
	}))

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired, "an unconfirmed secret must send the admin back to enrolment")
	assert.False(t, out.MFARequired)
}

func TestLogin_MFARequirementIsOffWhenNoRolesAreNamed(t *testing.T) {
	// A deployment that names no privileged roles has switched mandatory
	// enrolment off entirely — even for an account long past any deadline.
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	uc.RequireMFAForRoles(nil, nil)
	backdateGrace(users, 3650)

	out := login(t, uc)

	assert.False(t, out.MFAEnrollmentRequired)
	require.NotNil(t, out.TokenPair)
	assert.Equal(t, domain.MFAStateRecommended, out.MFAStatus.State)
}

func TestLogin_RoleMatchingIsCaseInsensitive(t *testing.T) {
	uc, users, _, _ := newLoginHarness(t, domain.MemberRole("ADMIN"))
	backdateGrace(users, 8)

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired, "role comparison must not depend on casing")
}

func TestLogin_RecordsDeviceProvenanceOnTheSession(t *testing.T) {
	// The device list in Settings is a projection of refresh_tokens, so login has
	// to write something recognisable there.
	uc, _, _, _ := newLoginHarness(t, domain.MemberRole("user"))

	out, err := uc.Execute(context.Background(), LoginInput{
		Email: "admin@opendefender.io", Password: "Ancre-Vitrail7-Cobalt",
		DeviceFingerprint: "fp-abc", IP: "41.202.10.7",
		UserAgent: "Mozilla/5.0 (Macintosh) Chrome/120 Safari/537",
	})
	require.NoError(t, err)
	require.NotNil(t, out.TokenPair)
}
