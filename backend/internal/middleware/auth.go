package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/golang-jwt/jwt/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// AuthMiddleware extracts and validates JWT token, populates request context with user claims
func AuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Skip auth for public endpoints
		if isPublicEndpoint(c.Path()) {
			return c.Next()
		}

		// Extract token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization header",
			})
		}

		// Parse "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) !=  || parts[] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}

		tokenString := parts[]

		// Parse and validate JWT token
		claims := &domain.UserClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token jwt.Token) (interface{}, error) {
			// Verify signing method
			if _, ok := token.Method.(jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Check token expiration
		if claims.ExpiresAt < time.Now().Unix() {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token expired",
			})
		}

		// Store user claims in context
		c.Locals("user", claims)
		c.Locals("user_id", claims.ID)
		c.Locals("role", claims.RoleName)
		c.Locals("permissions", claims.Permissions)

		return c.Next()
	}
}

// RoleGuard middleware checks if user has required role
func RoleGuard(allowedRoles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok || role == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No role in token",
			})
		}

		// Check if role is in allowed list
		for _, allowed := range allowedRoles {
			if role == allowed {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fmt.Sprintf("Role '%s' is not authorized for this operation", role),
		})
	}
}

// PermissionGuard middleware checks if user has required permission
func PermissionGuard(requiredPermission string) fiber.Handler {
	return func(c fiber.Ctx) error {
		permissions, ok := c.Locals("permissions").([]string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No permissions in token",
			})
		}

		// Check if user has permission
		if !hasPermission(permissions, requiredPermission) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("Missing required permission: %s", requiredPermission),
			})
		}

		return c.Next()
	}
}

// GetUserClaims extracts user claims from context
func GetUserClaims(c fiber.Ctx) domain.UserClaims {
	claims, ok := c.Locals("user").(domain.UserClaims)
	if !ok {
		return nil
	}
	return claims
}

// GetUserID extracts user ID from context
func GetUserID(c fiber.Ctx) (uuid.UUID, error) {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return uuid.UUID{}, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}

// hasPermission checks if permissions array contains required permission
func hasPermission(permissions []string, required string) bool {
	for _, perm := range permissions {
		// Exact match or admin wildcard
		if perm == required || perm == "" {
			return true
		}
		// Resource-level wildcard (e.g., "risk:" matches "risk:read")
		if len(perm) >  && perm[len(perm)-:] == ":" {
			resourceWildcard := perm[:len(perm)-]
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
		"/api/v/health",
		"/api/v/auth/login",
		"/api/v/auth/register",
		"/api/v/auth/refresh",
	}

	for _, public := range publicPaths {
		if strings.HasPrefix(path, public) {
			return true
		}
	}
	return false
}
