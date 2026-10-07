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
	"sync"
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
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

// #700 — two tabs of one browser share a cookie jar. When both renew the session
// at once, the losing refresh used to answer REFRESH_REUSE_DETECTED and clear the
// session cookies, wiping the ones the winner had just set. D-048 (#777) made a
// replay inside the grace window idempotent in the token manager; these tests
// hold the HTTP contract the browser actually sees.

type refreshStack struct {
	*deferredFixture
}

func newRefreshStack(t *testing.T) *refreshStack {
	t.Helper()
	f := newDeferredFixture(t)
	// ":memory:" is per connection: a burst must not open a second, empty one.
	sqlDB, err := f.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	tokens := coreauth.NewTokenManager(f.db, f.keys)
	tokens.SetSessionResolver(func(_ context.Context, _ uuid.UUID) (*coreauth.SessionClaims, error) {
		return &coreauth.SessionClaims{
			TenantID: f.tenantA,
			OrgRoles: map[uuid.UUID]string{f.tenantA: string(domain.RoleUser)},
		}, nil
	})
	loginUC := appauth.NewLoginUseCase(repository.NewGormUserRepository(f.db), tokens, deferredHasher{})
	h := authhandler.NewHandler(loginUC, nil, appauth.NewRefreshTokenUseCase(tokens), nil, deferredHasher{}, nil)

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Post("/auth/login", h.Login)
	api.Post("/auth/refresh", h.RefreshToken)
	f.app = app
	return &refreshStack{deferredFixture: f}
}

// loginRefreshCookie signs in and returns the refresh cookie the browser holds.
func (s *refreshStack) loginRefreshCookie(t *testing.T) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		jsonReader(t, map[string]string{"email": s.memberA.User.Email, "password": deferredPassword}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.app.Test(req, -1)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	refresh := cookieNamed(resp, middleware.RefreshTokenCookie)
	require.NotNil(t, refresh, "login must set the refresh cookie")
	return refresh.Value
}

type refreshResult struct {
	status int
	code   string
	resp   *http.Response
}

// send replays what the SPA sends: the refresh cookie and no body. It touches
// no *testing.T, so a burst can call it from its own goroutines.
func (s *refreshStack) send(token string) (refreshResult, error) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: middleware.RefreshTokenCookie, Value: token})
	resp, err := s.app.Test(req, -1)
	if err != nil {
		return refreshResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return refreshResult{}, err
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(raw, &body)
	return refreshResult{status: resp.StatusCode, code: body.Code, resp: resp}, nil
}

func (s *refreshStack) refresh(t *testing.T, token string) refreshResult {
	t.Helper()
	r, err := s.send(token)
	require.NoError(t, err)
	return r
}

func cookieNamed(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// clearedCookies lists the session cookies a response tells the browser to drop.
func clearedCookies(resp *http.Response) []string {
	var out []string
	for _, c := range resp.Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			out = append(out, c.Name)
		}
	}
	return out
}

func jsonReader(t *testing.T, v any) io.Reader {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return strings.NewReader(string(raw))
}

// TestRefreshHandler_ConcurrentBurst_Success — every request of a burst on one
// refresh cookie succeeds, re-issues the SAME refresh cookie, and none clears the
// session the others are setting.
func TestRefreshHandler_ConcurrentBurst_Success(t *testing.T) {
	s := newRefreshStack(t)
	token := s.loginRefreshCookie(t)

	const n = 6
	results := make([]refreshResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = s.send(token)
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "request %d", i)
	}

	first := cookieNamed(results[0].resp, middleware.RefreshTokenCookie)
	require.NotNil(t, first)
	for i, r := range results {
		require.Equal(t, http.StatusOK, r.status, "request %d (%s): a burst is not a theft", i, r.code)
		assert.Empty(t, clearedCookies(r.resp), "request %d cleared the session the others are setting", i)
		got := cookieNamed(r.resp, middleware.RefreshTokenCookie)
		require.NotNil(t, got, "request %d set no refresh cookie", i)
		assert.Equal(t, first.Value, got.Value, "request %d was handed a token of its own: the lineage forked", i)
		assert.NotNil(t, cookieNamed(r.resp, middleware.AccessTokenCookie), "request %d set no access cookie", i)
	}

	// Whatever order the responses landed in, the jar holds a live session.
	assert.Equal(t, http.StatusOK, s.refresh(t, first.Value).status)
}

// TestRefreshHandler_UnknownToken_NotFound — a token the server never issued is
// refused as a plain failure, not as reuse.
func TestRefreshHandler_UnknownToken_NotFound(t *testing.T) {
	s := newRefreshStack(t)
	r := s.refresh(t, "not-a-token-we-issued")
	assert.Equal(t, http.StatusUnauthorized, r.status)
	assert.Empty(t, r.code)
}

// TestRefreshHandler_ReplayAfterWindow_Unauthorized — the grace window is not a
// loophole: a rotated token replayed after it is still reuse. The family dies,
// the cookies are cleared and the code tells the client why (criterion 2).
func TestRefreshHandler_ReplayAfterWindow_Unauthorized(t *testing.T) {
	s := newRefreshStack(t)
	original := s.loginRefreshCookie(t)

	rotated := s.refresh(t, original)
	require.Equal(t, http.StatusOK, rotated.status)
	successor := cookieNamed(rotated.resp, middleware.RefreshTokenCookie).Value

	require.NoError(t, s.db.Model(&coreauth.RefreshToken{}).
		Where("rotated_at IS NOT NULL").
		Update("rotated_at", time.Now().Add(-coreauth.RotationGracePeriod-time.Minute)).Error)

	replay := s.refresh(t, original)
	assert.Equal(t, http.StatusUnauthorized, replay.status)
	assert.Equal(t, "REFRESH_REUSE_DETECTED", replay.code)
	assert.ElementsMatch(t,
		[]string{middleware.AccessTokenCookie, middleware.RefreshTokenCookie, middleware.CSRFCookie},
		clearedCookies(replay.resp))

	// The family is gone: the legitimate holder of the successor is signed out too.
	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, successor).status)
}
