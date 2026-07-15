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

// PATMiddleware authenticates Personal Access Tokens (L5) and is designed to run
// BEFORE the RS256 JWT middleware, alongside it:
//
//   - If the bearer credential is a PAT ("<prefix>_<secret>", no dots) and valid,
//     it populates the SAME request context a JWT login would (user_id, tenant_id,
//     org_roles, permissions, RequestContext) — but with permissions narrowed to
//     the PAT's scopes — then continues.
//   - For anything that is not a valid PAT (a JWT, a malformed value, or no header),
//     it is a NO-OP: it calls Next() and lets AuthMiddlewareRS256 authenticate (or
//     reject) the request. It never 401s a JWT.
//
// The JWT middleware, in turn, skips when a PAT has already authenticated the
// request (c.Locals("is_pat") == true), so the two coexist cleanly.
func PATMiddleware(patService *auth.PersonalAccessTokenService, resolve auth.SessionResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Already authenticated (defensive) — don't double-process.
		if c.Locals("user_id") != nil {
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next() // no credential → let the JWT middleware decide
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Next()
		}
		raw := parts[1]

		// A JWT is dot-delimited base64url; a PAT is "<8-hex>_<hex>" with no dots.
		// Only attempt PAT validation on the PAT shape so we never eat a JWT.
		if !looksLikePAT(raw) {
			return c.Next()
		}

		pat, err := patService.ValidateToken(c.UserContext(), raw)
		if err != nil {
			// Shaped like a PAT but invalid/expired/unknown — fall through so the
			// JWT middleware produces the canonical 401.
			return c.Next()
		}

		// PAT is valid — resolve the owning user's tenant + permissions so the token
		// carries a real tenant context (without this, every tenant-scoped handler
		// would fall back to uuid.Nil).
		if resolve == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"code": "PAT_MISCONFIGURED", "message": "PAT resolver not configured"})
		}
		sc, err := resolve(c.UserContext(), pat.UserID)
		if err != nil || sc == nil || sc.TenantID == uuid.Nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"code": "UNAUTHORIZED", "message": "token owner has no organization"})
		}

		// Respect PAT scopes: the effective permission set is the INTERSECTION of the
		// owner's permissions and the PAT scopes — a PAT can never exceed its owner,
		// and never exceed its scopes. We take each scope the owner is actually
		// entitled to (so an owner with "*" keeps exactly the scoped permissions,
		// while a limited owner cannot widen their PAT beyond what they hold).
		scopes := patService.GetScopes(pat)
		effective := make([]string, 0, len(scopes))
		for _, scope := range scopes {
			if permsGrant(sc.Permissions, scope) {
				effective = append(effective, scope)
			}
		}

		c.Locals("user_id", pat.UserID)
		c.Locals("userID", pat.UserID)
		c.Locals("tenant_id", sc.TenantID)
		c.Locals("tenantID", sc.TenantID)
		c.Locals("org_roles", sc.OrgRoles)
		c.Locals("permissions", effective)
		c.Locals("feature_flags", sc.FeatureFlags)
		c.Locals("token_id", pat.ID)
		c.Locals("token_scopes", scopes)
		c.Locals("is_pat", true)
		SetContext(c, &RequestContext{UserID: pat.UserID, OrganizationID: sc.TenantID})

		return c.Next()
	}
}

// permsGrant reports whether a permission list grants `required`, honoring the
// "*" admin wildcard and "resource:*" scoped wildcards (same semantics as the
// hasPermission helper used by the RBAC middleware).
func permsGrant(perms []string, required string) bool {
	for _, p := range perms {
		if p == required || p == "*" {
			return true
		}
		if strings.HasSuffix(p, ":*") {
			if strings.HasPrefix(required, strings.TrimSuffix(p, "*")) {
				return true
			}
		}
	}
	return false
}

// looksLikePAT reports whether a bearer value has the PAT shape "<8-hex>_<secret>"
// and is therefore not a JWT (which is always dot-delimited).
func looksLikePAT(raw string) bool {
	if strings.Contains(raw, ".") {
		return false
	}
	parts := strings.Split(raw, "_")
	return len(parts) == 2 && len(parts[0]) == 8
}

// RequireTokenScope checks if PAT has required scope
func RequireTokenScope(requiredScopes ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only apply to PAT-authenticated requests
		if c.Locals("is_pat") == nil {
			return c.Next()
		}

		_, ok := c.Locals("token_id").(uuid.UUID)
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
				}
			}
		}

		if !hasRequiredScope {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"code":    "FORBIDDEN",
				"message": "Token lacks required scope",
			})
		}

		return c.Next()
	}
}
