// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// Protected middleware verifies JWT token validity (RS256) and checks the
// Redis-backed JTI blacklist. Use this to protect routes that require
// authentication.
func Protected(rsaKeys *authpkg.RSAKeys, blacklistChecker func(jti string) (bool, error)) fiber.Handler {
	return AuthMiddlewareRS256(rsaKeys, blacklistChecker)
}

// RequireRole middleware checks if user has required role(s).
//
// NOTE: this used to read c.Locals("role") as a flat string — a key
// AuthMiddlewareRS256 never sets (it sets "org_roles", a map[uuid.UUID]string,
// instead; see auth.go). Every route guarded by RequireRole therefore returned 401
// "No role in token" unconditionally, for every caller, regardless of their actual
// role — this broke every mitigation/incident/risk-management write route wholesale.
// Fixed to read "org_roles" like RoleGuard already correctly does, and to match if
// ANY of the caller's per-organization roles is in the allowed list.
func RequireRole(roleNames ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		orgRoles, ok := c.Locals("org_roles").(map[uuid.UUID]string)
		if !ok || len(orgRoles) == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No role in token",
			})
		}

		for _, role := range orgRoles {
			// "root" is the platform superuser (the seeded admin's role) — it
			// satisfies every role gate. Without this, root users get 403 on
			// admin-only routes (RequireRole("admin")) even though they outrank
			// admin, which silently broke Users/RBAC/Audit-log/Tenant management.
			if role == "root" {
				return c.Next()
			}
			for _, allowed := range roleNames {
				if role == allowed {
					return c.Next()
				}
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Insufficient permissions",
		})
	}
}
