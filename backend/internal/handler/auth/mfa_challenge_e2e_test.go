// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
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
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// ---------------------------------------------------------------------------
// #689 — the MFA challenge through the real HTTP stack: per-IP limiter, the
// MFA_REQUIRED token middleware with a JTI blacklist, handler, use case and
// GORM repository, mounted in the order main.go uses.
// ---------------------------------------------------------------------------

const challengePath = "/api/v1/auth/mfa/challenge"

// memoryBlacklist stands in for Redis behind authpkg.TokenBlacklistManager.
type memoryBlacklist struct {
	mu   sync.Mutex
	keys map[string]string
}

func (m *memoryBlacklist) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[key] = value
	return nil
}

func (m *memoryBlacklist) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.keys[key], nil
}

func (m *memoryBlacklist) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.keys[key]
	return ok, nil
}

// challengeStack adds the challenge (and logout) routes to a fixture's
// database and keys, on a fresh app.
type challengeStack struct {
	*deferredFixture
	ipLimit int
}

func newChallengeStack(t *testing.T, ipLimit int) *challengeStack {
	t.Helper()
	f := newDeferredFixture(t)
	userRepo := repository.NewGormUserRepository(f.db)
	mfaRepo := repository.NewGormMFARepository(f.db)
	tokens := coreauth.NewTokenManager(f.db, f.keys)
	// What main.go's resolver derives from the default membership, reduced to
	// the one account these tests sign in with.
	tokens.SetSessionResolver(func(_ context.Context, userID uuid.UUID) (*coreauth.SessionClaims, error) {
		return &coreauth.SessionClaims{
			TenantID: f.tenantA,
			OrgRoles: map[uuid.UUID]string{f.tenantA: string(domain.RoleUser)},
		}, nil
	})
	orgRoles, businessRoles := domain.DefaultMFAPrivilegeRoles()

	blacklist := authpkg.NewTokenBlacklistManager(&memoryBlacklist{keys: map[string]string{}})
	checker := blacklist.CheckJTIBlacklist(context.Background())

	loginUC := appauth.NewLoginUseCase(userRepo, tokens, deferredHasher{}).
		WithMFA(mfaRepo).
		RequireMFAForRoles(orgRoles, businessRoles).
		WithMFAPolicies(f.policyRepo).
		WithClock(func() time.Time { return f.now })
	logoutUC := appauth.NewLogoutUseCase(tokens)
	h := authhandler.NewHandler(loginUC, nil, nil, logoutUC, deferredHasher{}, nil).
		WithUserLookup(userRepo).
		WithMFAStatus(f.resolver)

	challengeUC := appauth.NewChallengeMFAUseCase(mfaRepo, deferredTOTPKey).
		WithAttemptLimits(authmfa.NewMemoryAttemptStore(), blacklist)
	mfaHandler := authhandler.NewMFAHandler(nil, nil, nil, challengeUC, tokens, userRepo, nil)

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Post("/auth/login", h.Login)
	api.Post("/auth/logout", h.Logout)
	ipLimiter := middleware.RateLimit(middleware.RateLimitConfig{
		MaxRequests: ipLimit,
		WindowSize:  5 * time.Minute,
		Store:       middleware.NewRateLimitStore(),
	})
	api.Post("/auth/mfa/challenge", ipLimiter, middleware.MFATokenMiddleware(f.keys, checker), mfaHandler.Challenge)
	protected := api.Use(middleware.Protected(f.keys, checker))
	protected.Get("/auth/me", h.Me)

	f.app = app
	return &challengeStack{deferredFixture: f, ipLimit: ipLimit}
}

// enrolWithSecret gives a member a working authenticator and returns its seed.
func (s *challengeStack) enrolWithSecret(t *testing.T, m *domain.OrganizationMember) string {
	t.Helper()
	plain, err := otp.GenerateTOTPSecret()
	require.NoError(t, err)
	enc, err := crypto.EncryptAES256GCM(plain, deferredTOTPKey)
	require.NoError(t, err)
	require.NoError(t, s.db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: m.UserID, TenantID: m.OrganizationID, SecretEncrypted: enc, IsVerified: true,
	}).Error)
	return plain
}

// mfaToken signs in with the password and returns the MFA_REQUIRED token.
func (s *challengeStack) mfaToken(t *testing.T, email string) string {
	t.Helper()
	body, _ := s.login(t, email)
	tok, _ := body["mfa_token"].(string)
	require.NotEmpty(t, tok, "the login must stop at the code step: %v", body)
	return tok
}

func wrongCode(t *testing.T, plain string, seed int) string {
	t.Helper()
	for i := seed; ; i++ {
		c := fmt.Sprintf("%06d", (i*104729+7)%1000000)
		if !otp.VerifyTOTP(plain, c) {
			return c
		}
	}
}

func liveCode(t *testing.T, plain string) string {
	t.Helper()
	c, err := totp.GenerateCode(plain, time.Now())
	require.NoError(t, err)
	return c
}

func TestMFAChallengeE2E_CorrectCodeOpensASession(t *testing.T) {
	s := newChallengeStack(t, 100)
	plain := s.enrolWithSecret(t, s.memberA)

	status, body := s.do(t, http.MethodPost, challengePath, s.mfaToken(t, s.memberA.User.Email), jsonBody{"code": liveCode(t, plain)})
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.NotNil(t, body["token_pair"])
}

func TestMFAChallengeE2E_TokenIsRevokedAfterFiveWrongCodes(t *testing.T) {
	s := newChallengeStack(t, 100)
	plain := s.enrolWithSecret(t, s.memberA)
	tok := s.mfaToken(t, s.memberA.User.Email)

	for i := 1; i <= 4; i++ {
		status, _ := s.do(t, http.MethodPost, challengePath, tok, jsonBody{"code": wrongCode(t, plain, i)})
		require.Equal(t, http.StatusBadRequest, status, "wrong code %d", i)
	}
	status, body := s.do(t, http.MethodPost, challengePath, tok, jsonBody{"code": wrongCode(t, plain, 40)})
	require.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "MFA_CHALLENGE_EXHAUSTED", body["code"])

	// Sixth attempt, with the right code: the middleware refuses the token
	// before the handler reads anything.
	status, body = s.do(t, http.MethodPost, challengePath, tok, jsonBody{"code": liveCode(t, plain)})
	require.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "TOKEN_REVOKED", body["code"])
}

func TestMFAChallengeE2E_AccountLocksAcrossTokensWithRetryAfter(t *testing.T) {
	s := newChallengeStack(t, 100)
	plain := s.enrolWithSecret(t, s.memberA)

	n := 0
	for range 2 {
		tok := s.mfaToken(t, s.memberA.User.Email)
		for range 4 {
			n++
			status, _ := s.do(t, http.MethodPost, challengePath, tok, jsonBody{"code": wrongCode(t, plain, n)})
			require.Equal(t, http.StatusBadRequest, status, "failure %d", n)
		}
	}
	tok := s.mfaToken(t, s.memberA.User.Email)
	status, _ := s.do(t, http.MethodPost, challengePath, tok, jsonBody{"code": wrongCode(t, plain, 90)})
	require.Equal(t, http.StatusBadRequest, status, "failure 9")

	resp, body := s.raw(t, tok, wrongCode(t, plain, 91))
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "the tenth failure locks: %v", body)
	assert.Equal(t, "MFA_LOCKED", body["code"])
	assert.Equal(t, "900", resp.Header.Get("Retry-After"))

	// A fresh login and the right code: still refused while the lock holds.
	resp, body = s.raw(t, s.mfaToken(t, s.memberA.User.Email), liveCode(t, plain))
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "MFA_LOCKED", body["code"])
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
}

func TestMFAChallengeE2E_IPLimitRunsBeforeTheToken(t *testing.T) {
	s := newChallengeStack(t, 3)
	for i := range 3 {
		status, _ := s.do(t, http.MethodPost, challengePath, "not-a-token", jsonBody{"code": "123456"})
		require.Equal(t, http.StatusUnauthorized, status, "request %d reaches the token check", i+1)
	}
	status, _ := s.do(t, http.MethodPost, challengePath, "not-a-token", jsonBody{"code": "123456"})
	assert.Equal(t, http.StatusTooManyRequests, status, "the fourth is refused before the token is parsed")
}

// raw posts a code and keeps the response headers.
func (s *challengeStack) raw(t *testing.T, token, code string) (*http.Response, jsonBody) {
	t.Helper()
	raw, err := json.Marshal(jsonBody{"code": code})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, challengePath, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out := jsonBody{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}
