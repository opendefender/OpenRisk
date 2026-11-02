// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// GHSA-45mr-j2w6-m4jv — a member signs in to their default organisation, A,
// with the password alone, then switches to B, where they enrolled an
// authenticator. Real stack: login, switch, MFA_REQUIRED middleware, challenge,
// GORM repositories, both session resolvers.
// ---------------------------------------------------------------------------

const switchPath = "/api/v1/auth/switch-org"

// newSwitchMFAStack mounts the routes on the deferred fixture's database, in
// the order main.go uses. memberA also becomes a plain member of tenantB.
func newSwitchMFAStack(t *testing.T, roleInB domain.MemberRole, anchorInB time.Time) (*deferredFixture, *domain.OrganizationMember) {
	t.Helper()
	f := newDeferredFixture(t)
	// Production's unique user_id (domain.MFASecret), which the enrolment
	// upsert targets. #897 will make it (user_id, tenant_id).
	require.NoError(t, f.db.Exec(`CREATE UNIQUE INDEX idx_mfa_secrets_user_id ON mfa_secrets (user_id)`).Error)
	inB := &domain.OrganizationMember{
		ID: uuid.New(), UserID: f.memberA.UserID, OrganizationID: f.tenantB, Role: roleInB,
		IsActive: true, Status: domain.MembershipActive, JoinedAt: anchorInB, MFAGraceStartedAt: &anchorInB,
	}
	require.NoError(t, f.db.Create(inB).Error)

	userRepo := repository.NewGormUserRepository(f.db)
	mfaRepo := repository.NewGormMFARepository(f.db)
	tokens := coreauth.NewTokenManager(f.db, f.keys)
	// main.go's resolvers, reduced: an active membership in the named
	// organisation, the default one when none is named.
	forOrg := func(ctx context.Context, uid, orgID uuid.UUID) (*coreauth.SessionClaims, error) {
		m, err := userRepo.GetOrganizationMember(ctx, uid, orgID)
		if err != nil {
			return nil, err
		}
		if m == nil || !m.IsActive {
			return nil, domain.NewForbiddenError("not an active member")
		}
		return &coreauth.SessionClaims{TenantID: orgID, OrgRoles: map[uuid.UUID]string{orgID: string(m.Role)}, Permissions: m.EffectivePermissions()}, nil
	}
	tokens.SetOrgSessionResolver(forOrg)
	tokens.SetSessionResolver(func(ctx context.Context, uid uuid.UUID) (*coreauth.SessionClaims, error) {
		org, err := userRepo.GetUserDefaultOrganization(ctx, uid)
		if err != nil || org == nil {
			return nil, domain.NewForbiddenError("no organization")
		}
		return forOrg(ctx, uid, org.ID)
	})

	orgRoles, businessRoles := domain.DefaultMFAPrivilegeRoles()
	loginUC := appauth.NewLoginUseCase(userRepo, tokens, deferredHasher{}).
		WithMFA(mfaRepo).
		RequireMFAForRoles(orgRoles, businessRoles).
		WithMFAPolicies(f.policyRepo).
		WithClock(func() time.Time { return f.now })
	h := authhandler.NewHandler(loginUC, nil, nil, nil, deferredHasher{}, nil).WithUserLookup(userRepo)
	switcher := authhandler.NewSwitchHandler(nil, appauth.NewSwitchOrganizationUseCase(userRepo, tokens, loginUC), nil)
	challengeUC := appauth.NewChallengeMFAUseCase(mfaRepo, deferredTOTPKey).
		WithAttemptLimits(authmfa.NewMemoryAttemptStore(), nil)
	mfaHandler := authhandler.NewMFAHandler(
		appauth.NewSetupMFAUseCase(mfaRepo, deferredTOTPKey),
		appauth.NewVerifyMFAUseCase(mfaRepo, *userRepo, deferredTOTPKey),
		nil, challengeUC, tokens, userRepo, nil,
	).WithMFAStatus(f.resolver)

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Post("/auth/login", h.Login)
	api.Post("/auth/mfa/challenge", middleware.MFATokenMiddleware(f.keys, nil), mfaHandler.Challenge)
	enrolment := middleware.MFAEnrollmentMiddleware(f.keys, nil)
	api.Post("/auth/mfa/setup", enrolment, mfaHandler.Setup)
	api.Post("/auth/mfa/verify", enrolment, mfaHandler.Verify)
	protected := api.Use(middleware.Protected(f.keys, nil))
	protected.Post("/auth/switch-org", switcher.SwitchOrganization)
	f.app = app
	return f, inB
}

// switchTo posts the switch and also reports whether a session cookie was set.
func switchTo(t *testing.T, f *deferredFixture, token string, org uuid.UUID) (int, jsonBody, bool) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, switchPath, strings.NewReader(`{"organization_id":"`+org.String()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := jsonBody{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out, len(resp.Cookies()) > 0
}

func accessTenant(t *testing.T, f *deferredFixture, body jsonBody) uuid.UUID {
	t.Helper()
	pair, ok := body["token_pair"].(map[string]any)
	require.True(t, ok, "a session must be issued: %v", body)
	access, _ := pair["access_token"].(string)
	claims, err := authpkg.ValidateAccessToken(f.keys, access, nil)
	require.NoError(t, err)
	return claims.TenantID
}

func TestSwitchOrgMFAE2E_AuthenticatorInTargetIsChallengedAndSessionIsForTarget(t *testing.T) {
	f, inB := newSwitchMFAStack(t, domain.RoleUser, time.Now())
	stack := &challengeStack{deferredFixture: f}
	plain := stack.enrolWithSecret(t, inB)

	// A has no factor: the password alone opens a session there.
	_, tokenA := f.login(t, f.memberA.User.Email)
	require.NotEmpty(t, tokenA)

	status, body, cookies := switchTo(t, f, tokenA, f.tenantB)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, true, body["mfa_required"])
	assert.Nil(t, body["token_pair"], "the switch must not mint a session for B")
	assert.False(t, cookies, "no session cookie before B's factor is proved")
	mfaToken, _ := body["mfa_token"].(string)
	require.NotEmpty(t, mfaToken)

	// The challenge checks B's secret and issues the session for B.
	status, body = f.do(t, http.MethodPost, challengePath, mfaToken, jsonBody{"code": liveCode(t, plain)})
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, f.tenantB, accessTenant(t, f, body))
}

func TestSwitchOrgMFAE2E_RolePolicyInTargetRequiresEnrolment(t *testing.T) {
	f, _ := newSwitchMFAStack(t, domain.RoleAdmin, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	_, tokenA := f.login(t, f.memberA.User.Email)

	status, body, cookies := switchTo(t, f, tokenA, f.tenantB)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, true, body["mfa_enrollment_required"])
	assert.Nil(t, body["token_pair"])
	assert.False(t, cookies)
	mfaToken, _ := body["mfa_token"].(string)
	require.NotEmpty(t, mfaToken)

	// Enrolling with that token stores the authenticator in B, and confirming
	// it issues the session for B.
	status, body = f.do(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaToken, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	secret, _ := body["secret"].(string)
	require.NotEmpty(t, secret)
	status, body = f.do(t, http.MethodPost, "/api/v1/auth/mfa/verify", mfaToken, jsonBody{"code": liveCode(t, secret)})
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, f.tenantB, accessTenant(t, f, body))

	var inB int64
	require.NoError(t, f.db.Model(&domain.MFASecret{}).
		Where("user_id = ? AND tenant_id = ? AND is_verified = ?", f.memberA.UserID, f.tenantB, true).Count(&inB).Error)
	assert.Equal(t, int64(1), inB)
}

func TestSwitchOrgMFAE2E_NothingOwedInTargetSwitchesAtOnce(t *testing.T) {
	f, _ := newSwitchMFAStack(t, domain.RoleUser, time.Now())
	_, tokenA := f.login(t, f.memberA.User.Email)

	status, body, cookies := switchTo(t, f, tokenA, f.tenantB)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.True(t, cookies)
	assert.Equal(t, f.tenantB, accessTenant(t, f, body))
}

// The membership is re-checked when the session is issued: one revoked between
// the switch and the code gets no session in B.
func TestSwitchOrgMFAE2E_MembershipRevokedBeforeChallengeIssuesNoSession(t *testing.T) {
	f, inB := newSwitchMFAStack(t, domain.RoleUser, time.Now())
	stack := &challengeStack{deferredFixture: f}
	plain := stack.enrolWithSecret(t, inB)
	_, tokenA := f.login(t, f.memberA.User.Email)

	_, body, _ := switchTo(t, f, tokenA, f.tenantB)
	mfaToken, _ := body["mfa_token"].(string)
	require.NotEmpty(t, mfaToken, "%v", body)
	require.NoError(t, f.db.Model(&domain.OrganizationMember{}).Where("id = ?", inB.ID).Update("is_active", false).Error)

	status, body := f.do(t, http.MethodPost, challengePath, mfaToken, jsonBody{"code": liveCode(t, plain)})
	assert.NotEqual(t, http.StatusOK, status, "%v", body)
	assert.Nil(t, body["token_pair"])
}

func TestSwitchOrgMFAE2E_MembershipRevokedBeforeEnrolmentIssuesNoSession(t *testing.T) {
	f, inB := newSwitchMFAStack(t, domain.RoleAdmin, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	_, tokenA := f.login(t, f.memberA.User.Email)

	_, body, _ := switchTo(t, f, tokenA, f.tenantB)
	mfaToken, _ := body["mfa_token"].(string)
	require.NotEmpty(t, mfaToken, "%v", body)
	status, body := f.do(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaToken, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	secret, _ := body["secret"].(string)
	require.NoError(t, f.db.Model(&domain.OrganizationMember{}).Where("id = ?", inB.ID).Update("is_active", false).Error)

	status, body = f.do(t, http.MethodPost, "/api/v1/auth/mfa/verify", mfaToken, jsonBody{"code": liveCode(t, secret)})
	assert.NotEqual(t, http.StatusOK, status, "%v", body)
	assert.Nil(t, body["token_pair"])
}
