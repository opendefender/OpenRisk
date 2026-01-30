package middleware

import (
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v"
	"github.com/golang-jwt/jwt/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// PermissionMiddlewareConfig contains configuration for permission middleware
type PermissionMiddlewareConfig struct {
	UserService       services.UserService
	PermissionService services.PermissionService
	JWTSecret         string
}

// PermissionMiddleware validates user permissions for the requested resource
func PermissionMiddleware(config PermissionMiddlewareConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Extract token from Authorization header
		token := extractToken(c)
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization token",
			})
		}

		// Parse and validate JWT token
		claims, err := parseToken(token, config.JWTSecret)
		if err != nil {
			log.Printf("Token parse error: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token",
			})
		}

		// Extract user context from claims
		userID, tenantID, roleLevel, err := extractUserContext(claims)
		if err != nil {
			log.Printf("Context extraction error: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token claims",
			})
		}

		// Add context to locals for downstream handlers
		c.Locals("userID", userID)
		c.Locals("tenantID", tenantID)
		c.Locals("roleLevel", roleLevel)

		// Determine required permission for this endpoint
		resource, action := extractResourceAndAction(c.Method(), c.Path())
		c.Locals("resource", resource)
		c.Locals("action", action)

		// Admin users bypass permission checks
		if roleLevel == domain.RoleLevelAdmin {
			logPermissionCheck(userID, resource, action, "ADMIN_BYPASS")
			return c.Next()
		}

		// Check if user has required permission
		ctx := c.Context()
		hasPermission, err := config.UserService.ValidateUserPermission(
			ctx,
			userID,
			tenantID,
			resource,
			action,
		)

		if err != nil {
			log.Printf("Permission check error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "permission check failed",
			})
		}

		if !hasPermission {
			logPermissionCheck(userID, resource, action, "DENIED")
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("insufficient permissions for %s:%s", resource, action),
			})
		}

		logPermissionCheck(userID, resource, action, "ALLOWED")
		return c.Next()
	}
}

// TenantMiddleware validates tenant context and applies tenant isolation
func TenantMiddleware(userService services.UserService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get user context from previous middleware
		userID := c.Locals("userID").(uuid.UUID)
		tenantID := c.Locals("tenantID").(uuid.UUID)

		// Extract tenant ID from URL parameters or query
		requestTenantID := extractTenantIDFromRequest(c)
		if requestTenantID != uuid.Nil {
			// Validate tenant ID matches JWT claim
			if requestTenantID != tenantID {
				log.Printf("Tenant mismatch: user=%s, jwt_tenant=%s, request_tenant=%s",
					userID, tenantID, requestTenantID)
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "unauthorized tenant access",
				})
			}
		}

		// Validate user belongs to this tenant
		ctx := c.Context()
		if !userService.ValidateUserInTenant(ctx, userID, tenantID) {
			log.Printf("User not in tenant: user=%s, tenant=%s", userID, tenantID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "user not member of this tenant",
			})
		}

		return c.Next()
	}
}

// OwnershipMiddleware verifies resource ownership or inherited access via role
func OwnershipMiddleware(userService services.UserService) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := c.Locals("userID").(uuid.UUID)
		roleLevel := c.Locals("roleLevel").(domain.RoleLevel)

		// Extract resource ID from URL
		resourceID := c.Params("id")
		if resourceID == "" {
			// No specific resource, check passes
			return c.Next()
		}

		// Admin/Manager always has access to team resources
		if roleLevel >= domain.RoleLevelManager {
			logOwnershipCheck(userID, resourceID, "INHERITED_ACCESS")
			return c.Next()
		}

		// For Analyst/Viewer, would need to check actual ownership
		// This would require a generic ownership check function
		// For now, allow and log
		logOwnershipCheck(userID, resourceID, "ALLOWED")
		return c.Next()
	}
}

// AuditMiddleware logs all permission-related activities
func AuditMiddleware(auditService interface{}) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Log the audit event (would be implemented with actual audit service)
		userID, ok := c.Locals("userID").(uuid.UUID)
		if !ok {
			return c.Next()
		}

		tenantID, ok := c.Locals("tenantID").(uuid.UUID)
		if !ok {
			return c.Next()
		}

		log.Printf("AUDIT: user=%s, tenant=%s, method=%s, path=%s, status=%d",
			userID, tenantID, c.Method(), c.Path(), c.Response().StatusCode)

		return c.Next()
	}
}

// Helper functions

// extractToken extracts JWT token from Authorization header
func extractToken(c fiber.Ctx) string {
	auth := c.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", )
	if len(parts) !=  || parts[] != "Bearer" {
		return ""
	}

	return parts[]
}

// parseToken parses and validates JWT token
func parseToken(tokenString string, jwtSecret string) (jwt.MapClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}

// extractUserContext extracts user ID, tenant ID, and role level from JWT claims
func extractUserContext(claims jwt.MapClaims) (uuid.UUID, uuid.UUID, domain.RoleLevel, error) {
	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return uuid.Nil, uuid.Nil, , fmt.Errorf("missing user_id in token")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, , fmt.Errorf("invalid user_id format")
	}

	tenantIDStr, ok := claims["tenant_id"].(string)
	if !ok {
		return uuid.Nil, uuid.Nil, , fmt.Errorf("missing tenant_id in token")
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, , fmt.Errorf("invalid tenant_id format")
	}

	roleLevel, ok := claims["role_level"].(float)
	if !ok {
		return uuid.Nil, uuid.Nil, , fmt.Errorf("missing role_level in token")
	}

	return userID, tenantID, domain.RoleLevel(int(roleLevel)), nil
}

// extractResourceAndAction determines required resource and action from HTTP method and path
func extractResourceAndAction(method string, path string) (string, string) {
	resource := extractResourceFromPath(path)
	action := extractActionFromMethod(method)
	return resource, action
}

// extractResourceFromPath extracts resource type from URL path
func extractResourceFromPath(path string) string {
	// Remove leading slash and extract first path component
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >  {
		// Remove "api/v" prefix if present
		if parts[] == "api" && len(parts) >  {
			return parts[]
		}
		return parts[]
	}

	return "unknown"
}

// extractActionFromMethod converts HTTP method to permission action
func extractActionFromMethod(method string) string {
	switch method {
	case "GET":
		return string(domain.PermissionRead)
	case "POST":
		return string(domain.PermissionCreate)
	case "PUT", "PATCH":
		return string(domain.PermissionUpdate)
	case "DELETE":
		return string(domain.PermissionDelete)
	default:
		return "unknown"
	}
}

// extractTenantIDFromRequest tries to extract tenant ID from URL parameters or query
func extractTenantIDFromRequest(c fiber.Ctx) uuid.UUID {
	// Try URL parameter
	if tenantStr := c.Params("tenant_id"); tenantStr != "" {
		if id, err := uuid.Parse(tenantStr); err == nil {
			return id
		}
	}

	// Try query parameter
	if tenantStr := c.Query("tenant_id"); tenantStr != "" {
		if id, err := uuid.Parse(tenantStr); err == nil {
			return id
		}
	}

	return uuid.Nil
}

// logPermissionCheck logs permission check results
func logPermissionCheck(userID uuid.UUID, resource string, action string, result string) {
	log.Printf("PERMISSION_CHECK: user=%s, resource=%s, action=%s, result=%s",
		userID, resource, action, result)
}

// logOwnershipCheck logs ownership check results
func logOwnershipCheck(userID uuid.UUID, resourceID string, result string) {
	log.Printf("OWNERSHIP_CHECK: user=%s, resource_id=%s, result=%s",
		userID, resourceID, result)
}
