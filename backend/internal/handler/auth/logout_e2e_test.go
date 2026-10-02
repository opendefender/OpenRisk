// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/middleware"
)

// #689 — logout ends the access token too, not just the refresh token.

func TestLogout_AccessTokenRejectedAfterLogout(t *testing.T) {
	s := newChallengeStack(t, 100)
	_, access := s.login(t, s.memberA.User.Email)
	require.NotEmpty(t, access)

	status, _ := s.do(t, http.MethodGet, "/api/v1/auth/me", access, nil)
	require.Equal(t, http.StatusOK, status, "the token works before logout")

	status, body := s.do(t, http.MethodPost, "/api/v1/auth/logout", access, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)

	status, body = s.do(t, http.MethodGet, "/api/v1/auth/me", access, nil)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "TOKEN_REVOKED", body["code"])
}

// A browser holds the token in the session cookie, not in a header.
func TestLogout_CookieSessionAccessTokenRejectedAfterLogout(t *testing.T) {
	s := newChallengeStack(t, 100)
	_, access := s.login(t, s.memberA.User.Email)
	require.NotEmpty(t, access)

	withCookie := func(method, path string) *http.Response {
		req := httptest.NewRequest(method, path, strings.NewReader(""))
		req.AddCookie(&http.Cookie{Name: middleware.AccessTokenCookie, Value: access})
		resp, err := s.app.Test(req, -1)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp
	}

	require.Equal(t, http.StatusOK, withCookie(http.MethodGet, "/api/v1/auth/me").StatusCode)
	require.Equal(t, http.StatusOK, withCookie(http.MethodPost, "/api/v1/auth/logout").StatusCode)
	assert.Equal(t, http.StatusUnauthorized, withCookie(http.MethodGet, "/api/v1/auth/me").StatusCode)
}

// A forged token at logout revokes nothing and still answers 200: the caller
// loses their cookies either way.
func TestLogout_ForgedTokenIsIgnored(t *testing.T) {
	s := newChallengeStack(t, 100)
	_, access := s.login(t, s.memberA.User.Email)

	forged := access[:len(access)-4] + "AAAA"
	status, _ := s.do(t, http.MethodPost, "/api/v1/auth/logout", forged, nil)
	assert.Equal(t, http.StatusOK, status)

	status, _ = s.do(t, http.MethodGet, "/api/v1/auth/me", access, nil)
	assert.Equal(t, http.StatusOK, status, "the real token was not revoked by a forgery")
}
