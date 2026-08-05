// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var (
	errNoCredentials  = errors.New("Missing authorization header")
	errBadAuthHeader  = errors.New("Invalid authorization header format")
	errCSRFMissing    = errors.New("Missing CSRF token")
	errCSRFMismatch   = errors.New("CSRF token mismatch")
	errCSRFNoCookieID = errors.New("Missing CSRF cookie")
)

// extractAccessToken pulls the access token from the Authorization header or,
// failing that, the HttpOnly session cookie.
//
// The header wins deliberately. Existing non-browser callers — personal access
// tokens, scanner agents, CI scripts, the OpenAPI client — all send a bearer
// header, and a cookie left over in a shared browser profile must never override
// an explicitly supplied credential.
//
// The second return value reports whether the credential came from the cookie,
// which is what decides if CSRF applies.
//
// The error message for a missing credential is unchanged from the header-only
// era on purpose: clients and tests match on it, and a browser session failing
// this check is a client bug, not something a new message would help diagnose.
func extractAccessToken(c *fiber.Ctx) (token string, fromCookie bool, err error) {
	if authHeader := c.Get("Authorization"); authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return "", false, errBadAuthHeader
		}
		return parts[1], false, nil
	}

	if cookie := c.Cookies(AccessTokenCookie); cookie != "" {
		return cookie, true, nil
	}

	return "", false, errNoCredentials
}

// safeMethods do not change state, so CSRF does not apply to them.
//
// This is the standard set. A GET that mutates would be exempt here, but such a
// handler is already a defect on its own terms (proxies and prefetchers replay
// safe methods freely), so the right fix is the handler, not a broader check.
var safeMethods = map[string]bool{
	fiber.MethodGet:     true,
	fiber.MethodHead:    true,
	fiber.MethodOptions: true,
}

// verifyCSRF enforces the double-submit check on cookie-authenticated requests.
//
// The browser attaches the CSRF cookie to a cross-site request just as it does
// the session cookie, so the cookie alone proves nothing. What the attacker
// cannot do is *read* that cookie from another origin and copy it into a header:
// the same-origin policy forbids it. Requiring the two to match is therefore a
// proof that the request came from our own page.
func verifyCSRF(c *fiber.Ctx) error {
	if safeMethods[c.Method()] {
		return nil
	}

	cookieToken := c.Cookies(CSRFCookie)
	if cookieToken == "" {
		return errCSRFNoCookieID
	}

	headerToken := c.Get(CSRFHeader)
	if headerToken == "" {
		return errCSRFMissing
	}

	// Constant-time: a timing-variable comparison would let an attacker recover
	// the token byte by byte across many requests.
	if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
		return errCSRFMismatch
	}

	return nil
}

// RefreshTokenFromRequest returns the refresh token supplied by cookie, for
// handlers that still accept it in the body as well.
func RefreshTokenFromRequest(c *fiber.Ctx) string {
	return c.Cookies(RefreshTokenCookie)
}
