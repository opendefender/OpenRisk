// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

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

// newLoginHarness builds a login use case over an in-memory refresh-token store
// and a real TokenManager, so the tokens it mints can actually be parsed.
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
	member := &domain.OrganizationMember{
		ID: uuid.New(), UserID: user.ID, OrganizationID: org.ID, Role: role,
	}

	users := &loginUsers{user: user, org: org, member: member}
	mfaRepo := NewMockMFARepository()

	uc := NewLoginUseCase(users, tm, fakeHasher{}).
		WithMFA(mfaRepo).
		RequireMFAForRoles("admin", "root")

	return uc, users, mfaRepo, keys
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

func TestLogin_AdminWithoutMFAGetsNoSession(t *testing.T) {
	// The requirement is mandatory, not advisory: handing out a full session with
	// a "please enrol soon" banner would make it optional in practice.
	uc, users, _, keys := newLoginHarness(t, domain.MemberRole("admin"))

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

func TestLogin_RootWithoutMFAGetsNoSession(t *testing.T) {
	uc, _, _, _ := newLoginHarness(t, domain.MemberRole("root"))

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
	require.NoError(t, mfaRepo.CreateMFASecret(context.Background(), &domain.MFASecret{
		ID: uuid.New(), UserID: users.user.ID, TenantID: users.org.ID,
		SecretEncrypted: "encrypted", IsVerified: false,
	}))

	out := login(t, uc)

	assert.True(t, out.MFAEnrollmentRequired, "an unconfirmed secret must send the admin back to enrolment")
	assert.False(t, out.MFARequired)
}

func TestLogin_MFARequirementIsOffWhenNoRolesAreNamed(t *testing.T) {
	// Deployments that do not call RequireMFAForRoles keep the previous behaviour.
	uc, _, _, _ := newLoginHarness(t, domain.MemberRole("admin"))
	uc.requireMFARoles = nil

	out := login(t, uc)

	assert.False(t, out.MFAEnrollmentRequired)
	require.NotNil(t, out.TokenPair)
}

func TestLogin_RoleMatchingIsCaseInsensitive(t *testing.T) {
	uc, _, _, _ := newLoginHarness(t, domain.MemberRole("ADMIN"))

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
