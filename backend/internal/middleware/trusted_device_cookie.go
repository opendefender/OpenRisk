// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// The trusted-device cookie marks a browser that completed a sign-in (D-064,
// #879). HttpOnly, and sent only under /api/v1/auth: sign-in reads it, and so
// do the two routes that renew it after untrusting every device.
const (
	trustedDeviceCookieBase = "or_trusted_device"
	trustedDeviceCookiePath = "/api/v1/auth"
)

// TrustedDeviceCookieName is the cookie's name. Over HTTPS it carries the
// __Secure- prefix, so a sibling subdomain cannot plant a cookie of that name
// to shadow the real one (#879 review). __Host- would need Path=/.
func TrustedDeviceCookieName() string {
	if secureCookies() {
		return "__Secure-" + trustedDeviceCookieBase
	}
	return trustedDeviceCookieBase
}

// SetTrustedDeviceCookie plants or renews the cookie.
func SetTrustedDeviceCookie(c *fiber.Ctx, value string, ttl time.Duration) {
	c.Cookie(&fiber.Cookie{
		Name:     TrustedDeviceCookieName(),
		Value:    value,
		Path:     trustedDeviceCookiePath,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HTTPOnly: true,
		Secure:   secureCookies(),
		// Lax like the session cookies: the SPA's sign-in POST is same-site.
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// TrustedDeviceCookieValue reads the cookie, copied: Fiber reuses the request
// buffer once the handler returns.
func TrustedDeviceCookieValue(c *fiber.Ctx) string {
	return utils.CopyString(c.Cookies(TrustedDeviceCookieName()))
}
