// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/auth"
)

// PATMiddleware validates Personal Access Tokens
// Must be used after AuthMiddlewareRS256 for JWT tokens
func PATMiddleware(patService *auth.PersonalAccessTokenService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip if already authenticated via JWT
		if c.Locals("user_id") != nil {
			return c.Next()
		}

		// Extract token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Missing authorization header",
			})
		}

		// Parse "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Invalid authorization header format",
			})
		}

		tokenValue := parts[1]

		// Validate PAT
		token, err := patService.ValidateToken(c.Context(), tokenValue)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "TOKEN_INVALID",
				"message": "Invalid or expired personal access token",
			})
		}

		// Store token info in context
		c.Locals("user_id", token.UserID)
		c.Locals("token_id", token.ID)
		c.Locals("token_scopes", token.Scopes)
		c.Locals("is_pat", true)

		// For PAT, we need to determine tenant context
		// This should be set by the handler or a subsequent middleware
		// For now, we'll assume single-tenant or handler sets it

		return c.Next()
	}
}

// RequireTokenScope checks if PAT has required scope
func RequireTokenScope(requiredScopes ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only apply to PAT-authenticated requests
		if c.Locals("is_pat") == nil {
			return c.Next()
		}

		tokenID, ok := c.Locals("token_id").(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Token not authenticated",
			})
		}

		// Get token scopes from context
		scopes, ok := c.Locals("token_scopes").([]string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Token scopes not available",
			})
		}

		// Check if token has any of the required scopes
		hasRequiredScope := false
		for _, required := range requiredScopes {
			for _, scope := range scopes {
				if scope == required || scope == "*" {
					hasRequiredScope = true
					break
				}
				// Support wildcards
				if strings.HasSuffix(scope, ":*") {
					resourceWildcard := scope[:len(scope)-1]
					if len(required) > len(resourceWildcard) && required[:len(resourceWildcard)] == resourceWildcard {
						hasRequiredScope = true
						break
					}
				}
			}
			if hasRequiredScope {
				break
			}
		}

		if !hasRequiredScope {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"code":    "FORBIDDEN",
				"message": "Token does not have required scope",
			})
		}

		return c.Next()
	}

