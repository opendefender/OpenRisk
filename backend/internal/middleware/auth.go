package middleware

import (
	"fmt"
	"strings"

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

		// Store claims in context for downstream handlers
		c.Locals("user", claims)
		c.Locals("user_id", claims.Sub)
		c.Locals("tenant_id", claims.TenantID)
		c.Locals("org_roles", claims.OrgRoles)
		c.Locals("permissions", claims.Permissions)
		c.Locals("feature_flags", claims.FeatureFlags)
		c.Locals("jti", claims.JTI)

		return c.Next()
	}
}

// TenantMiddleware ensures tenant_id is present in context.
// Must be placed AFTER AuthMiddlewareRS256.
func TenantMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenantID, ok := c.Locals("tenant_id").(uuid.UUID)
		if !ok || tenantID == uuid.Nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "UNAUTHORIZED",
				"message": "Missing tenant context",
			})
		}
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
	}

	for _, public := range publicPaths {
		if strings.HasPrefix(path, public) {
			return true
		}
	}
	return false
}
