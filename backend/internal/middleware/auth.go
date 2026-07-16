// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// AuthMiddlewareRS256 extracts and validates JWT token (RS256), populates request context.
// Requires Redis client for JTI blacklist checking.
// This replaces the old HMAC-based AuthMiddleware.
func AuthMiddlewareRS256(rsaKeys *authpkg.RSAKeys, redisBlacklistChecker func(jti string) (bool, error)) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip auth for public endpoints
		if isPublicEndpoint(c.Path()) {
			return c.Next()
		}

		// Already authenticated by an upstream PAT middleware (L5). PATs and JWTs
		// coexist: PATMiddleware runs first and authenticates PAT-shaped bearers;
		// this JWT middleware then handles everything else and must not re-validate
		// (and reject) a request a PAT already authorized.
		if c.Locals("is_pat") == true {
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

		tokenString := parts[1]

		// Validate JWT with RS256 signature and JTI blacklist check
		claims, err := authpkg.ValidateAccessToken(rsaKeys, tokenString, redisBlacklistChecker)
		if err != nil {
			switch err {
			case authpkg.ErrTokenExpired:
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"code":    "TOKEN_EXPIRED",
					"message": "Token has expired",
				})
			case authpkg.ErrTokenRevoked:
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"code":    "TOKEN_REVOKED",
					"message": "Token has been revoked",
				})
			case authpkg.ErrTokenInvalid:
				fallthrough
			default:
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"code":    "TOKEN_INVALID",
					"message": "Invalid token",
				})
			}
		}

		// Validate required tenant_id
		if claims.TenantID == uuid.Nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Missing tenant_id in token",
			})
		}

		// Store claims in context for downstream handlers.
		// Both snake_case and camelCase keys are set for the same values: a number of
		// older handlers (analytics_handler.go, enhanced_dashboard_handler.go,
		// rbac_*_handler.go, token_handler.go) read "userID"/"tenantID" while this
		// middleware historically only set "user_id"/"tenant_id" — every request to
		// those handlers silently 401'd regardless of auth state. Setting both avoids
		// having to touch every call site.
		c.Locals("user", claims)
		c.Locals("user_id", claims.Sub)
		c.Locals("userID", claims.Sub)
		c.Locals("tenant_id", claims.TenantID)
		c.Locals("tenantID", claims.TenantID)
		c.Locals("org_roles", claims.OrgRoles)
		c.Locals("permissions", claims.Permissions)
		c.Locals("feature_flags", claims.FeatureFlags)
		c.Locals("jti", claims.JTI)

		// middleware.GetContext(c) is what compliance/risk/asset/mitigation/dashboard/
		// multitenancy handlers actually read their tenant_id/user_id from (a third,
		// separate context mechanism from the c.Locals keys above) — nothing in production
		// ever called SetContext before this, so every one of those handlers silently fell
		// back to uuid.Nil for the tenant, regardless of who was logged in. The nil-tenant
		// guard in GormComplianceRepository.CreateControl is what finally surfaced this as a
		// hard 500 instead of silently mixing every tenant's data into one Nil bucket.
		SetContext(c, &RequestContext{UserID: claims.Sub, OrganizationID: claims.TenantID})

		return c.Next()
	}
}

// RequirePermission checks if user has required permissions.
// Supports wildcards: "risks:*" matches "risks:read", "risks:write", etc.
func RequirePermission(requiredPerms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		permissions, ok := c.Locals("permissions").([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"code":    "FORBIDDEN",
				"message": "No permissions in token",
			})
		}

		// Check if user has any of the required permissions
		hasRequiredPerm := false
		for _, required := range requiredPerms {
			if hasPermission(permissions, required) {
				hasRequiredPerm = true
				break
			}
		}

		if !hasRequiredPerm {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"code":    "FORBIDDEN",
				"message": fmt.Sprintf("Missing required permission: %v", requiredPerms),
			})
		}

		return c.Next()
	}
}

// RoleGuard checks if user has required role (deprecated - use RequirePermission instead).
// Maintained for backward compatibility.
func RoleGuard(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		orgRoles, ok := c.Locals("org_roles").(map[uuid.UUID]string)
		if !ok || len(orgRoles) == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "No role in token",
			})
		}

		// Check if any role is in allowed list
		for _, role := range orgRoles {
			for _, allowed := range allowedRoles {
				if role == allowed {
					return c.Next()
				}
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    "FORBIDDEN",
			"message": fmt.Sprintf("Role not authorized for this operation"),
		})
	}
}

// PermissionGuard is an alias for RequirePermission (deprecated - use RequirePermission).
func PermissionGuard(requiredPermission string) fiber.Handler {
	return RequirePermission(requiredPermission)
}

// GetUserClaims extracts RS256 claims from context.
// Returns nil if claims not found or invalid.
func GetUserClaims(c *fiber.Ctx) *authpkg.Claims {
	claims, ok := c.Locals("user").(*authpkg.Claims)
	if !ok {
		return nil
	}
	return claims
}

// GetUserID extracts user ID from context
func GetUserID(c *fiber.Ctx) (uuid.UUID, error) {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.UUID{}, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}

// GetTenantID extracts tenant ID from context (tenant isolation key).
func GetTenantID(c *fiber.Ctx) (uuid.UUID, error) {
	tenantID, ok := c.Locals("tenant_id").(uuid.UUID)
	if !ok || tenantID == uuid.Nil {
		return uuid.UUID{}, fmt.Errorf("tenant ID not found in context")
	}
	return tenantID, nil
}

// hasPermission checks if permissions array contains required permission.
// Supports wildcards: "risk:*" matches "risk:read", "risk:write", etc.
// Supports admin wildcard: "*" matches everything.
func hasPermission(permissions []string, required string) bool {
	for _, perm := range permissions {
		// Exact match or admin wildcard
		if perm == required || perm == "*" {
			return true
		}
		// Resource-level wildcard (e.g., "risk:*" matches "risk:read")
		if len(perm) > 2 && perm[len(perm)-2:] == ":*" {
			resourceWildcard := perm[:len(perm)-1]
			if len(required) > len(resourceWildcard) && required[:len(resourceWildcard)] == resourceWildcard {
				return true
			}
		}
	}
	return false
}

// isPublicEndpoint checks if endpoint should skip authentication
func isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/api/v1/health",
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/refresh",
		"/api/v1/auth/oauth2/google/redirect",
		"/api/v1/auth/oauth2/google/callback",
		"/api/v1/auth/oauth2/github/redirect",
		"/api/v1/auth/oauth2/github/callback",
	}

	for _, public := range publicPaths {
		if strings.HasPrefix(path, public) {
			return true
		}
	}
	return false
}

// MFATokenMiddleware handles MFA_REQUIRED temporary tokens
// Allows access to /auth/mfa/challenge endpoint with temporary token
func MFATokenMiddleware(rsaKeys *authpkg.RSAKeys, redisBlacklistChecker func(jti string) (bool, error)) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only apply to MFA challenge endpoint
		if c.Path() != "/api/v1/auth/mfa/challenge" {
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

		tokenString := parts[1]

		// Validate JWT with RS256 signature and JTI blacklist check
		claims, err := authpkg.ValidateAccessToken(rsaKeys, tokenString, redisBlacklistChecker)
		if err != nil {
			switch err {
			case authpkg.ErrTokenExpired:
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"code":    "TOKEN_EXPIRED",
					"message": "Token has expired",
				})
			case authpkg.ErrTokenRevoked:
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"code":    "TOKEN_REVOKED",
					"message": "Token has been revoked",
				})
			case authpkg.ErrTokenInvalid:
				fallthrough
			default:
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"code":    "TOKEN_INVALID",
					"message": "Invalid token",
				})
			}
		}

		// Check if this is an MFA_REQUIRED token
		if claims.Type != "MFA_REQUIRED" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Invalid token type for MFA challenge",
			})
		}

		// Validate required tenant_id
		if claims.TenantID == uuid.Nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Missing tenant_id in token",
			})
		}

		// Store claims in context for downstream handlers.
		// Both snake_case and camelCase keys are set for the same values: a number of
		// older handlers (analytics_handler.go, enhanced_dashboard_handler.go,
		// rbac_*_handler.go, token_handler.go) read "userID"/"tenantID" while this
		// middleware historically only set "user_id"/"tenant_id" — every request to
		// those handlers silently 401'd regardless of auth state. Setting both avoids
		// having to touch every call site.
		c.Locals("user", claims)
		c.Locals("user_id", claims.Sub)
		c.Locals("userID", claims.Sub)
		c.Locals("tenant_id", claims.TenantID)
		c.Locals("tenantID", claims.TenantID)
		c.Locals("org_roles", claims.OrgRoles)
		c.Locals("permissions", claims.Permissions)
		c.Locals("feature_flags", claims.FeatureFlags)
		c.Locals("jti", claims.JTI)

		// middleware.GetContext(c) is what compliance/risk/asset/mitigation/dashboard/
		// multitenancy handlers actually read their tenant_id/user_id from (a third,
		// separate context mechanism from the c.Locals keys above) — nothing in production
		// ever called SetContext before this, so every one of those handlers silently fell
		// back to uuid.Nil for the tenant, regardless of who was logged in. The nil-tenant
		// guard in GormComplianceRepository.CreateControl is what finally surfaced this as a
		// hard 500 instead of silently mixing every tenant's data into one Nil bucket.
		SetContext(c, &RequestContext{UserID: claims.Sub, OrganizationID: claims.TenantID})
		c.Locals("mfa_required", true) // Flag for MFA challenge

		return c.Next()
	}
}

// MFARateLimit creates rate limiting for MFA endpoints (5 req/min per user)
func MFARateLimit(store *RateLimitStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only apply to MFA endpoints
		if !strings.HasPrefix(c.Path(), "/api/v1/auth/mfa/") {
			return c.Next()
		}

		// Get user ID from context (should be set by auth middleware)
		userID, ok := c.Locals("user_id").(uuid.UUID)
		var key string
		if !ok || userID == uuid.Nil {
			// Fallback to IP if no user ID (for MFA challenge endpoint)
			key = c.IP()
			if forwarded := c.Get("X-Forwarded-For"); forwarded != "" {
				key = forwarded
			}
		} else {
			key = fmt.Sprintf("user:%s", userID.String())
		}

		// Check rate limit: 5 requests per minute
		if !store.IsAllowed(key, 5, 1*time.Minute) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "Too many MFA requests. Please try again later.",
			})
		}

		return c.Next()
	}
}

// OAuthRateLimit creates rate limiting for OAuth callbacks (10 req/min per IP)
func OAuthRateLimit(store *RateLimitStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only apply to OAuth callback endpoints
		if !strings.HasSuffix(c.Path(), "/callback") || !strings.Contains(c.Path(), "/oauth2/") {
			return c.Next()
		}

		// Use IP address for rate limiting
		key := c.IP()
		if forwarded := c.Get("X-Forwarded-For"); forwarded != "" {
			key = forwarded
		}

		// Check rate limit: 10 requests per minute per IP
		if !store.IsAllowed(key, 10, 1*time.Minute) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "Too many OAuth requests. Please try again later.",
			})
		}

		return c.Next()
	}
}
