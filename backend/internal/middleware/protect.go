// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package middleware

import (
	"github.com/gofiber/fiber/v2"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// Protected middleware verifies JWT token validity (RS256) and checks the
// Redis-backed JTI blacklist. Use this to protect routes that require
// authentication.
func Protected(rsaKeys *authpkg.RSAKeys, blacklistChecker func(jti string) (bool, error)) fiber.Handler {
	return AuthMiddlewareRS256(rsaKeys, blacklistChecker)
}

// RequireRole middleware checks if user has required role(s)
func RequireRole(roleNames ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok || role == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No role in token",
			})
		}

		// Check if role is in allowed list
		for _, allowed := range roleNames {
			if role == allowed {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Insufficient permissions",
		})
	}
}
