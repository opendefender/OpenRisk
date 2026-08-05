// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Session cookie names.
//
// The access and refresh cookies are HttpOnly: script cannot read them, which is
// the entire point — tokens previously lived in localStorage, where any XSS (or
// a malicious dependency, which is the more realistic vector for an SPA with a
// large dependency tree) could exfiltrate a full session.
//
// The CSRF cookie is deliberately NOT HttpOnly. The double-submit pattern
// requires the SPA to read it and echo it in a header, and a value the attacker
// cannot read from another origin is exactly what makes the check meaningful.
const (
	AccessTokenCookie  = "or_access"
	RefreshTokenCookie = "or_refresh"
	CSRFCookie         = "or_csrf"

	// CSRFHeader is where the SPA echoes the CSRF cookie value.
	CSRFHeader = "X-CSRF-Token"
)

// csrfTokenBytes is the entropy of a CSRF token. 32 bytes is far beyond what is
// needed to resist guessing within a session's lifetime.
const csrfTokenBytes = 32

// secureCookies reports whether cookies must carry the Secure attribute.
//
// Tied to the environment rather than to the request scheme: a production
// deployment behind a TLS-terminating proxy sees plain HTTP at the socket, so
// deriving this from c.Protocol() would silently drop Secure exactly where it
// matters most. Development stays non-Secure so the cookie works over
// http://localhost.
func secureCookies() bool { return IsProductionEnv() }

// NewCSRFToken mints a random double-submit token.
func NewCSRFToken() (string, error) {
	buf := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// IssueSessionCookies writes the session pair plus a fresh CSRF token.
//
// SameSite=Lax is the deliberate choice. Strict would break the common
// "click a link in a notification email and land authenticated" flow, while None
// would allow any site to trigger authenticated GETs. Lax blocks cross-site
// POST/PUT/DELETE — the requests that change state — and the CSRF double-submit
// covers what remains.
//
// Returns the CSRF token so the caller can also hand it to the client in the
// response body, sparing the SPA a cookie read on first load.
func IssueSessionCookies(c *fiber.Ctx, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) (string, error) {
	csrfToken, err := NewCSRFToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	secure := secureCookies()

	c.Cookie(&fiber.Cookie{
		Name:     AccessTokenCookie,
		Value:    accessToken,
		Path:     "/",
		Expires:  now.Add(accessTTL),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})

	// Scoped to the refresh endpoint only: a token that can mint new sessions has
	// no business being attached to every API call, where a single logging
	// mishap or misrouted proxy would expose it.
	c.Cookie(&fiber.Cookie{
		Name:     RefreshTokenCookie,
		Value:    refreshToken,
		Path:     "/api/v1/auth/refresh",
		Expires:  now.Add(refreshTTL),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})

	c.Cookie(&fiber.Cookie{
		Name:     CSRFCookie,
		Value:    csrfToken,
		Path:     "/",
		Expires:  now.Add(refreshTTL),
		HTTPOnly: false, // must be readable by the SPA — see the const block
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})

	return csrfToken, nil
}

// ClearSessionCookies expires all three cookies.
//
// Each must be cleared with the same Path it was set with; a mismatched Path
// leaves the browser holding a second, still-valid cookie that logout appears to
// have removed.
func ClearSessionCookies(c *fiber.Ctx) {
	past := time.Now().Add(-time.Hour)
	secure := secureCookies()

	for _, ck := range []struct {
		name string
		path string
		http bool
	}{
		{AccessTokenCookie, "/", true},
		{RefreshTokenCookie, "/api/v1/auth/refresh", true},
		{CSRFCookie, "/", false},
	} {
		c.Cookie(&fiber.Cookie{
			Name:     ck.name,
			Value:    "",
			Path:     ck.path,
			Expires:  past,
			MaxAge:   -1,
			HTTPOnly: ck.http,
			Secure:   secure,
			SameSite: fiber.CookieSameSiteLaxMode,
		})
	}
}
