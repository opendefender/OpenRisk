// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// MFAEnrollmentMiddleware guards the MFA enrolment endpoints (/auth/mfa/setup
// and /auth/mfa/verify).
//
// It accepts EITHER credential:
//
//   - a full access token — the ordinary case, a signed-in user turning MFA on
//     voluntarily from Settings;
//   - an MFA_ENROLLMENT token — a user whose role mandates MFA, who has just
//     proved their password but cannot yet hold a session because nothing is
//     enrolled.
//
// The routes it guards are registered BEFORE Protected in main.go (#944):
// Protected accepts a full access token only, so mounting them behind it would
// lock the mandated-enrolment path out. This middleware therefore carries the
// CSRF check for cookie callers itself.
//
// It deliberately does NOT accept MFA_REQUIRED. That token is issued to accounts
// which already HAVE a verified authenticator, and letting it reach enrolment
// would allow someone with a stolen password to register their own authenticator
// over the victim's and walk straight past the second factor. MFA_ENROLLMENT is
// only ever minted when no verified secret exists, which is what makes the split
// safe rather than merely tidy.
func MFAEnrollmentMiddleware(rsaKeys *authpkg.RSAKeys, redisBlacklistChecker func(jti string) (bool, error)) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Header first, then the session cookie, exactly as Protected reads them,
		// so a signed-in user enrolling from Settings works like any other call.
		tokenString, fromCookie, extractErr := extractAccessToken(c)
		if extractErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": extractErr.Error(),
			})
		}

		// These routes are mounted BEFORE Protected (#944), so Protected's CSRF
		// check no longer runs ahead of them. A cookie-borne session is attached
		// by the browser on its own; without the double-submit check any page
		// could start an enrolment in the victim's name.
		if fromCookie {
			if err := verifyCSRF(c); err != nil {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"code":    "CSRF_FAILED",
					"message": err.Error(),
				})
			}
			c.Locals("auth_via_cookie", true)
		}

		// Type-agnostic validation: MFA_ENROLLMENT is precisely the token
		// ValidateAccessToken refuses (#944). The allowed types are asserted below.
		claims, err := authpkg.ValidateToken(rsaKeys, tokenString, redisBlacklistChecker)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "TOKEN_INVALID",
				"message": "Invalid token",
			})
		}

		switch claims.Type {
		case authpkg.TokenTypeAccess, authpkg.TokenTypeMFAEnrollment:
			// Allowed.
		default:
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Invalid token type for MFA enrollment",
			})
		}

		if claims.Sub == uuid.Nil || claims.TenantID == uuid.Nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Incomplete token",
			})
		}

		// Same locals the other auth middlewares publish, in both spellings, so
		// downstream handlers and GetContext behave identically here.
		c.Locals("user", claims)
		c.Locals("user_id", claims.Sub)
		c.Locals("userID", claims.Sub)
		c.Locals("tenant_id", claims.TenantID)
		c.Locals("tenantID", claims.TenantID)
		c.Locals("org_roles", claims.OrgRoles)
		c.Locals("permissions", claims.Permissions)
		c.Locals("jti", claims.JTI)
		// Lets the handler tell a voluntary enrolment from a mandated one.
		c.Locals("mfa_enrollment_token", claims.Type == authpkg.TokenTypeMFAEnrollment)

		SetContext(c, &RequestContext{
			UserID:         claims.Sub,
			OrganizationID: claims.TenantID,
			IPAddress:      c.IP(),
			UserAgent:      c.Get("User-Agent"),
		})
		return c.Next()
	}
}
