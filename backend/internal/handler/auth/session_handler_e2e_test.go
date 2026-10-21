// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #727 — "sign out other devices" through the real HTTP stack: the auth
// middleware, the session handler, the GORM session projection, the token
// manager and the refresh handler, mounted as main.go mounts them.
//
// Requests carry only what a browser would send to each route: the access
// cookie everywhere, the refresh cookie only to /api/v1/auth/refresh, where
// its Path scopes it.
// ---------------------------------------------------------------------------

type sessionStack struct {
	app    *fiber.App
	db     *gorm.DB
	tokens *coreauth.TokenManager
	userID uuid.UUID
	orgID  uuid.UUID
}

func newSessionStack(t *testing.T) *sessionStack {
	t.Helper()

	dsn := "file:sessions_others_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
			expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
			created_at DATETIME, updated_at DATETIME
		)`).Error)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}

	s := &sessionStack{db: db, userID: uuid.New(), orgID: uuid.New()}
	s.tokens = coreauth.NewTokenManager(db, keys)
	// What main.go's org resolver derives from an active membership, reduced to
	// the one account these tests use.
	s.tokens.SetOrgSessionResolver(func(_ context.Context, _, orgID uuid.UUID) (*coreauth.SessionClaims, error) {
		return &coreauth.SessionClaims{TenantID: orgID, OrgRoles: map[uuid.UUID]string{orgID: "user"}}, nil
	})

	sessionRepo := repository.NewGormSessionRepository(db)
	sessions := authhandler.NewSessionHandler(
		appauth.NewListSessionsUseCase(sessionRepo),
		appauth.NewRevokeSessionUseCase(sessionRepo),
		appauth.NewRevokeOtherSessionsUseCase(sessionRepo).WithReissue(s.tokens),
		nil,
	)
	h := authhandler.NewHandler(nil, nil, appauth.NewRefreshTokenUseCase(s.tokens), nil, nil, nil)

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Post("/auth/refresh", h.RefreshToken)
	protected := api.Use(middleware.Protected(keys, func(string) (bool, error) { return false, nil }))
	protected.Get("/auth/sessions", sessions.ListSessions)
	protected.Delete("/auth/sessions/others", sessions.RevokeOtherSessions)
	s.app = app
	return s
}

// signIn opens a session on a named device, as login does.
func (s *sessionStack) signIn(t *testing.T, device string) *coreauth.TokenPair {
	t.Helper()
	pair, err := s.tokens.IssueSessionForOrg(context.Background(), s.userID, s.orgID, coreauth.DeviceContext{UserAgent: device})
	require.NoError(t, err)
	return pair
}

type sessionResponse struct {
	status  int
	body    map[string]any
	cookies map[string]*http.Cookie
}

func (s *sessionStack) send(t *testing.T, method, path string, cookies map[string]string, headers map[string]string) sessionResponse {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := s.app.Test(req, 10000)
	require.NoError(t, err)
	raw, _ := io.ReadAll(res.Body)
	out := sessionResponse{status: res.StatusCode, cookies: map[string]*http.Cookie{}}
	_ = json.Unmarshal(raw, &out.body)
	for _, c := range res.Cookies() {
		out.cookies[c.Name] = c
	}
	return out
}

// refresh is the browser's refresh: the refresh cookie and nothing else.
func (s *sessionStack) refresh(t *testing.T, refreshToken string) sessionResponse {
	t.Helper()
	return s.send(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{middleware.RefreshTokenCookie: refreshToken}, nil)
}

func (s *sessionStack) liveSessions(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Table("refresh_tokens").Where("user_id = ? AND rotated_at IS NULL", s.userID).Count(&n).Error)
	return n
}

// browserCookies is what the SPA's browser attaches to an authenticated API
// call: the access cookie and the CSRF cookie, whose value the SPA echoes in
// the header (see browserHeaders).
func browserCookies(pair *coreauth.TokenPair) map[string]string {
	return map[string]string{middleware.AccessTokenCookie: pair.AccessToken, middleware.CSRFCookie: "csrf-727"}
}

var browserHeaders = map[string]string{middleware.CSRFHeader: "csrf-727"}

// Criterion 1: the other device is signed out, the caller is not.
func TestRevokeOtherSessionsE2E_BrowserCallerStaysSignedIn(t *testing.T) {
	s := newSessionStack(t)
	laptop := s.signIn(t, "laptop")
	phone := s.signIn(t, "phone")

	// The browser sends the access cookie here; the refresh cookie's Path keeps
	// it off this route.
	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", browserCookies(laptop), browserHeaders)
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.Equal(t, false, res.body["reauthenticate"])
	assert.EqualValues(t, 2, res.body["revoked"], "the phone, and the laptop's session it replaced")
	assert.NotEmpty(t, res.body["csrf_token"])

	issued := res.cookies[middleware.RefreshTokenCookie]
	require.NotNil(t, issued, "the caller's device must be handed its new refresh cookie")
	assert.Equal(t, "/api/v1/auth/refresh", issued.Path,
		"same Path as at sign-in, so it replaces the cookie the browser holds")
	assert.Equal(t, int64(1), s.liveSessions(t), "exactly one session survives: the caller's")

	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, phone.RefreshToken).status,
		"the other device can no longer refresh")

	next := s.refresh(t, issued.Value)
	require.Equal(t, http.StatusOK, next.status, "the caller can still refresh: %v", next.body)

	// And keeps doing so: the lineage rotates on as any other session would.
	rotated := next.cookies[middleware.RefreshTokenCookie]
	require.NotNil(t, rotated)
	assert.Equal(t, http.StatusOK, s.refresh(t, rotated.Value).status)
}

// An API client that names its session in X-Refresh-Token keeps that very
// session; nothing is re-issued and no cookie is set.
func TestRevokeOtherSessionsE2E_PresentedSessionIsKept(t *testing.T) {
	s := newSessionStack(t)
	client := s.signIn(t, "ci-runner")
	other := s.signIn(t, "phone")

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", browserCookies(client),
		map[string]string{middleware.CSRFHeader: "csrf-727", "X-Refresh-Token": client.RefreshToken})
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.EqualValues(t, 1, res.body["revoked"])
	assert.Nil(t, res.body["token_pair"])
	assert.Nil(t, res.cookies[middleware.RefreshTokenCookie])

	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, other.RefreshToken).status)
	assert.Equal(t, http.StatusOK, s.refresh(t, client.RefreshToken).status)
}

func TestRevokeOtherSessionsE2E_Unauthorized(t *testing.T) {
	s := newSessionStack(t)
	phone := s.signIn(t, "phone")

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, res.status)
	assert.Equal(t, int64(1), s.liveSessions(t), "an anonymous call revokes nothing")
	assert.Equal(t, http.StatusOK, s.refresh(t, phone.RefreshToken).status)
}
