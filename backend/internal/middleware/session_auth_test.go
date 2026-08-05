// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// probeExtraction runs extractAccessToken behind a route and reports what it saw.
func probeExtraction(t *testing.T, decorate func(*http.Request)) (token string, fromCookie bool, errMsg string) {
	t.Helper()

	app := fiber.New()
	app.Get("/probe", func(c *fiber.Ctx) error {
		tok, cookie, err := extractAccessToken(c)
		if err != nil {
			return c.JSON(fiber.Map{"err": err.Error()})
		}
		return c.JSON(fiber.Map{"token": tok, "from_cookie": cookie})
	})

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/probe", nil)
	decorate(req)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Token      string `json:"token"`
		FromCookie bool   `json:"from_cookie"`
		Err        string `json:"err"`
	}
	if err := decodeJSON(resp.Body, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Token, body.FromCookie, body.Err
}

func TestExtractAccessToken_HeaderWinsOverCookie(t *testing.T) {
	// A stale cookie in a shared browser profile must never override an
	// explicitly supplied bearer credential.
	token, fromCookie, errMsg := probeExtraction(t, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer header-token")
		r.AddCookie(&http.Cookie{Name: AccessTokenCookie, Value: "cookie-token"})
	})

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if token != "header-token" {
		t.Errorf("got %q, want the header token", token)
	}
	if fromCookie {
		t.Error("header credential must not be flagged as cookie-sourced (it would wrongly trigger CSRF)")
	}
}

func TestExtractAccessToken_FallsBackToCookie(t *testing.T) {
	token, fromCookie, errMsg := probeExtraction(t, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: AccessTokenCookie, Value: "cookie-token"})
	})

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if token != "cookie-token" {
		t.Errorf("got %q, want the cookie token", token)
	}
	if !fromCookie {
		t.Error("cookie credential must be flagged so CSRF applies")
	}
}

func TestExtractAccessToken_Rejections(t *testing.T) {
	t.Run("no credentials", func(t *testing.T) {
		_, _, errMsg := probeExtraction(t, func(*http.Request) {})
		if errMsg != errNoCredentials.Error() {
			t.Errorf("got %q, want %q", errMsg, errNoCredentials)
		}
	})

	t.Run("malformed header is not silently ignored", func(t *testing.T) {
		// Falling through to the cookie here would let a broken client be
		// authenticated by a stale cookie it did not intend to use.
		_, _, errMsg := probeExtraction(t, func(r *http.Request) {
			r.Header.Set("Authorization", "Basic abc123")
			r.AddCookie(&http.Cookie{Name: AccessTokenCookie, Value: "cookie-token"})
		})
		if errMsg != errBadAuthHeader.Error() {
			t.Errorf("got %q, want %q", errMsg, errBadAuthHeader)
		}
	})
}

// csrfProbe exercises verifyCSRF through a route.
func csrfProbe(t *testing.T, method string, cookieToken, headerToken string) int {
	t.Helper()

	app := fiber.New()
	handler := func(c *fiber.Ctx) error {
		if err := verifyCSRF(c); err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendString("ok")
	}
	app.Add(method, "/probe", handler)

	req, _ := http.NewRequest(method, "http://example.com/probe", nil)
	if cookieToken != "" {
		req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: cookieToken})
	}
	if headerToken != "" {
		req.Header.Set(CSRFHeader, headerToken)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// TestVerifyCSRF_BlocksForgedStateChange is the core of the protection: a
// cross-site POST carries the cookies (the browser attaches them) but cannot
// carry the header, because the attacker's origin cannot read our cookie.
func TestVerifyCSRF_BlocksForgedStateChange(t *testing.T) {
	if got := csrfProbe(t, http.MethodPost, "real-token", ""); got != fiber.StatusForbidden {
		t.Errorf("POST with cookie but no header: got %d, want 403", got)
	}
	if got := csrfProbe(t, http.MethodPost, "real-token", "guessed-token"); got != fiber.StatusForbidden {
		t.Errorf("POST with mismatched header: got %d, want 403", got)
	}
	if got := csrfProbe(t, http.MethodDelete, "real-token", ""); got != fiber.StatusForbidden {
		t.Errorf("DELETE with no header: got %d, want 403", got)
	}
}

func TestVerifyCSRF_AllowsMatchingToken(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if got := csrfProbe(t, method, "matching-token", "matching-token"); got != fiber.StatusOK {
			t.Errorf("%s with matching token: got %d, want 200", method, got)
		}
	}
}

// TestVerifyCSRF_SafeMethodsExempt: GET/HEAD/OPTIONS do not change state, and
// requiring a header on them would break ordinary navigation and prefetching.
func TestVerifyCSRF_SafeMethodsExempt(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if got := csrfProbe(t, method, "", ""); got != fiber.StatusOK {
			t.Errorf("%s without any CSRF token: got %d, want 200", method, got)
		}
	}
}

func TestIssueSessionCookies_Attributes(t *testing.T) {
	app := fiber.New()
	app.Get("/login", func(c *fiber.Ctx) error {
		token, err := IssueSessionCookies(c, "access-value", "refresh-value", 15*time.Minute, 24*time.Hour)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"csrf": token})
	})

	resp, err := app.Test(httpGet("http://example.com/login"))
	if err != nil {
		t.Fatalf("login probe: %v", err)
	}
	defer resp.Body.Close()

	byName := map[string]*http.Cookie{}
	for _, ck := range resp.Cookies() {
		byName[ck.Name] = ck
	}

	access, ok := byName[AccessTokenCookie]
	if !ok {
		t.Fatal("access cookie not set")
	}
	if !access.HttpOnly {
		t.Error("access cookie must be HttpOnly — readable tokens are the exposure being removed")
	}
	if access.Value != "access-value" {
		t.Errorf("access cookie value: got %q", access.Value)
	}
	if access.SameSite != http.SameSiteLaxMode {
		t.Errorf("access cookie SameSite: got %v, want Lax", access.SameSite)
	}

	refresh, ok := byName[RefreshTokenCookie]
	if !ok {
		t.Fatal("refresh cookie not set")
	}
	if !refresh.HttpOnly {
		t.Error("refresh cookie must be HttpOnly")
	}
	if refresh.Path != "/api/v1/auth/refresh" {
		t.Errorf("refresh cookie should be scoped to the refresh endpoint, got %q", refresh.Path)
	}

	csrf, ok := byName[CSRFCookie]
	if !ok {
		t.Fatal("csrf cookie not set")
	}
	if csrf.HttpOnly {
		t.Error("CSRF cookie must be readable by the SPA — double-submit depends on it")
	}
	if csrf.Value == "" {
		t.Error("CSRF cookie is empty")
	}
}

func TestNewCSRFToken_IsRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		token, err := NewCSRFToken()
		if err != nil {
			t.Fatalf("NewCSRFToken: %v", err)
		}
		if len(token) < 32 {
			t.Fatalf("token too short to resist guessing: %q", token)
		}
		if seen[token] {
			t.Fatal("duplicate CSRF token generated")
		}
		seen[token] = true
	}
}

func TestClearSessionCookies_ExpiresAllThree(t *testing.T) {
	app := fiber.New()
	app.Get("/logout", func(c *fiber.Ctx) error {
		ClearSessionCookies(c)
		return c.SendString("bye")
	})

	resp, err := app.Test(httpGet("http://example.com/logout"))
	if err != nil {
		t.Fatalf("logout probe: %v", err)
	}
	defer resp.Body.Close()

	cleared := map[string]bool{}
	for _, ck := range resp.Cookies() {
		// Deletion per RFC 6265 is an empty value with an expiry in the past.
		// Fiber emits Expires rather than Max-Age when both are set, so asserting
		// on MaxAge would test the framework's serialisation choice, not the
		// behaviour the browser acts on.
		if ck.Value == "" && ck.Expires.Before(time.Now()) {
			cleared[ck.Name] = true
		}
		// A mismatched Path leaves the browser holding a second, still-valid
		// cookie that logout only appears to have removed.
		if ck.Name == RefreshTokenCookie && ck.Path != "/api/v1/auth/refresh" {
			t.Errorf("refresh cookie cleared with Path %q, must match the Path it was set with", ck.Path)
		}
	}

	for _, name := range []string{AccessTokenCookie, RefreshTokenCookie, CSRFCookie} {
		if !cleared[name] {
			t.Errorf("cookie %q was not expired on logout", name)
		}
	}
}

func httpGet(url string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	return req
}

func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
