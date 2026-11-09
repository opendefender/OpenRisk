// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/middleware"
)

// #1314 — /auth/refresh sits outside Protected and reads a cookie the browser
// attaches on its own, so it carries the double-submit check itself. A forged
// refresh would rotate the victim's session from another page.

func (s *refreshStack) sendCrossOrigin(t *testing.T, token string, csrfCookie, csrfHeader string) refreshResult {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.AddCookie(&http.Cookie{Name: middleware.RefreshTokenCookie, Value: token})
	if csrfCookie != "" {
		req.AddCookie(&http.Cookie{Name: middleware.CSRFCookie, Value: csrfCookie})
	}
	if csrfHeader != "" {
		req.Header.Set(middleware.CSRFHeader, csrfHeader)
	}
	resp, err := s.app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	return refreshResult{status: resp.StatusCode, code: decodeCode(t, resp), resp: resp}
}

func decodeCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(raw, &body)
	return body.Code
}

func TestRefreshCSRF_Success(t *testing.T) {
	s := newRefreshStack(t)
	token := s.loginRefreshCookie(t)

	r := s.refresh(t, token)
	assert.Equal(t, http.StatusOK, r.status)
	assert.NotNil(t, cookieNamed(r.resp, middleware.RefreshTokenCookie), "a refresh with the CSRF token must rotate")
}

func TestRefreshCSRF_Unauthorized(t *testing.T) {
	for name, tc := range map[string]struct{ cookie, header string }{
		"no CSRF cookie or header": {},
		"cookie without header":    {cookie: "csrf-a"},
		"header without cookie":    {header: "csrf-a"},
		"mismatch":                 {cookie: "csrf-a", header: "csrf-b"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newRefreshStack(t)
			token := s.loginRefreshCookie(t)

			r := s.sendCrossOrigin(t, token, tc.cookie, tc.header)
			assert.Equal(t, http.StatusForbidden, r.status)
			assert.Equal(t, "CSRF_FAILED", r.code)
			assert.Nil(t, cookieNamed(r.resp, middleware.RefreshTokenCookie), "a refused refresh must not rotate")
			assert.Empty(t, clearedCookies(r.resp), "a forged request must not sign the victim out either")

			// The refusal spent nothing: the browser's own refresh still works.
			assert.Equal(t, http.StatusOK, s.refresh(t, token).status)
		})
	}
}

// A token in the body was put there by its caller, which a cross-site page
// cannot do with the victim's token: no CSRF applies, as for a bearer header.
// An unknown body token is refused by the use case, not by the CSRF check.
func TestRefreshCSRF_NotFound(t *testing.T) {
	s := newRefreshStack(t)
	token := s.loginRefreshCookie(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh",
		jsonReader(t, map[string]string{"refresh_token": token}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.app.Test(req, -1)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh",
		jsonReader(t, map[string]string{"refresh_token": "unknown"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = s.app.Test(req, -1)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
