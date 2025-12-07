package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// RequirePermissions middleware checks if the user has the required permission(s)
// It should be used after the Auth middleware which sets the user context
func RequirePermissions(ps *services.PermissionService, required ...domain.Permission) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user claims from context (set by Auth middleware)
		claims, ok := c.Locals("user").(*domain.UserClaims)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "user context not found")
		}

		// Check if any of the required permissions are met
		hasPermission := false
		for _, perm := range required {
			if ps.CheckPermission(claims.ID.String(), claims.RoleName, perm) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			requiredStr := ""
			for i, perm := range required {
				if i > 0 {
					requiredStr += " OR "
				}
				requiredStr += perm.String()
			}
			return fiber.NewError(
				fiber.StatusForbidden,
				fmt.Sprintf("insufficient permissions: required one of [%s]", requiredStr),
			)
		}

		return c.Next()
	}
}

// RequireAllPermissions middleware checks if the user has ALL required permissions
func RequireAllPermissions(ps *services.PermissionService, required ...domain.Permission) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user claims from context
		claims, ok := c.Locals("user").(*domain.UserClaims)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "user context not found")
		}

		// Check if all required permissions are met
		if !ps.CheckPermissionAll(claims.ID.String(), claims.RoleName, required) {
			requiredStr := ""
			for i, perm := range required {
				if i > 0 {
					requiredStr += " AND "
				}
				requiredStr += perm.String()
			}
			return fiber.NewError(
				fiber.StatusForbidden,
				fmt.Sprintf("insufficient permissions: required all of [%s]", requiredStr),
			)
		}

		return c.Next()
	}
}

// RequireResourcePermission checks if the user has permission for a specific resource with context
// This supports scope checking (own vs team vs any)
func RequireResourcePermission(ps *services.PermissionService, resource domain.PermissionResource, action domain.PermissionAction) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user claims from context
		claims, ok := c.Locals("user").(*domain.UserClaims)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "user context not found")
		}

		// Get the resource owner from context (usually set by the handler)
		resourceOwnerID := c.Locals("resourceOwnerID")

		// Determine scope based on ownership
		scope := domain.PermissionScopeAny
		if resourceOwnerID != nil {
			if ownerID, ok := resourceOwnerID.(string); ok {
				if ownerID == claims.ID.String() {
					scope = domain.PermissionScopeOwn
				} else {
					scope = domain.PermissionScopeTeam
				}
			}
		}

		// Check permission with determined scope
		required := domain.Permission{
			Resource: resource,
			Action:   action,
			Scope:    scope,
		}

		if !ps.CheckPermission(claims.ID.String(), claims.RoleName, required) {
			return fiber.NewError(
				fiber.StatusForbidden,
				fmt.Sprintf("insufficient permissions: required %s", required.String()),
			)
		}

		return c.Next()
	}
}

// PermissionMiddlewareFactory provides middleware functions for permission checking
type PermissionMiddlewareFactory struct {
	permissionService *services.PermissionService
}

// NewPermissionMiddlewareFactory creates a new permission middleware factory
func NewPermissionMiddlewareFactory(ps *services.PermissionService) *PermissionMiddlewareFactory {
	return &PermissionMiddlewareFactory{
		permissionService: ps,
	}
}

// CreatePermissionMiddleware creates a middleware that requires specific permissions
func (pmf *PermissionMiddlewareFactory) CreatePermissionMiddleware(required ...domain.Permission) fiber.Handler {
	return RequirePermissions(pmf.permissionService, required...)
}

// CreateAllPermissionsMiddleware creates a middleware that requires all permissions
func (pmf *PermissionMiddlewareFactory) CreateAllPermissionsMiddleware(required ...domain.Permission) fiber.Handler {
	return RequireAllPermissions(pmf.permissionService, required...)
}

// CreateResourcePermissionMiddleware creates a middleware for resource-specific permissions
func (pmf *PermissionMiddlewareFactory) CreateResourcePermissionMiddleware(resource domain.PermissionResource, action domain.PermissionAction) fiber.Handler {
	return RequireResourcePermission(pmf.permissionService, resource, action)
}
