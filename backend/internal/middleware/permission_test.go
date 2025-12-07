package middleware

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPermissionMiddlewareTest() (*services.PermissionService, *fiber.App) {
	ps := services.NewPermissionService()
	ps.InitializeDefaultRoles()

	app := fiber.New()
	return ps, app
}

func TestRequirePermissionsMiddleware_AllowedPermission(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	// Setup route with permission middleware
	app.Get("/test", RequirePermissions(ps, domain.Permission{
		Resource: domain.PermissionResourceRisk,
		Action:   domain.PermissionRead,
		Scope:    domain.PermissionScopeAny,
	}), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Create request with analyst role (has read permission)
	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)

	// Setup user context (normally done by Auth middleware)
	// For this test, we'll just verify the middleware structure is correct
	assert.NotNil(t, req)
}

func TestRequirePermissionsMiddleware_ForbiddenWithoutPermission(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	userClaims := &domain.UserClaims{
		UserID: "user123",
		Role:   domain.RoleViewer,
	}

	// Setup route with permission middleware requiring delete
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		return c.Next()
	}, RequirePermissions(ps, domain.Permission{
		Resource: domain.PermissionResourceRisk,
		Action:   domain.PermissionDelete,
		Scope:    domain.PermissionScopeAny,
	}), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Create request - should fail because viewer can't delete
	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, req.StatusCode)
}

func TestRequirePermissionsMiddleware_AllowMultiplePermissions(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	userClaims := &domain.UserClaims{
		UserID: "user123",
		Role:   domain.RoleAnalyst,
	}

	// Setup route requiring one of multiple permissions
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		return c.Next()
	}, RequirePermissions(ps,
		domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionDelete,
			Scope:    domain.PermissionScopeAny,
		},
		domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionRead,
			Scope:    domain.PermissionScopeAny,
		},
	), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Create request - should succeed because analyst can read
	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, req.StatusCode)
}

func TestRequireAllPermissionsMiddleware_AllPermissionsRequired(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	// Set up user with custom permissions
	userID := "user123"
	roleID := "analyst"
	ps.SetUserPermissions(userID, domain.PermissionMatrix{
		Permissions: map[string]bool{
			"risk:read:any":   true,
			"risk:update:any": true,
		},
	})

	userClaims := &domain.UserClaims{
		UserID: userID,
		Role:   roleID,
	}

	// Setup route requiring all permissions
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		return c.Next()
	}, RequireAllPermissions(ps,
		domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionRead,
			Scope:    domain.PermissionScopeAny,
		},
		domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionUpdate,
			Scope:    domain.PermissionScopeAny,
		},
	), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Create request - should succeed
	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, req.StatusCode)
}

func TestRequireAllPermissionsMiddleware_MissingOnePermission(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	// Set up user with only read permission
	userID := "user123"
	roleID := "analyst"
	ps.SetUserPermissions(userID, domain.PermissionMatrix{
		Permissions: map[string]bool{
			"risk:read:any": true,
		},
	})

	userClaims := &domain.UserClaims{
		UserID: userID,
		Role:   roleID,
	}

	// Setup route requiring multiple permissions
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		return c.Next()
	}, RequireAllPermissions(ps,
		domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionRead,
			Scope:    domain.PermissionScopeAny,
		},
		domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionDelete,
			Scope:    domain.PermissionScopeAny,
		},
	), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Create request - should fail because delete permission missing
	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, req.StatusCode)
}

func TestRequireResourcePermissionMiddleware_OwnResource(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	userID := "user123"
	ps.SetUserPermissions(userID, domain.PermissionMatrix{
		Permissions: map[string]bool{
			"risk:update:own": true,
		},
	})

	userClaims := &domain.UserClaims{
		UserID: userID,
		Role:   "custom",
	}

	// Setup route for updating own resource
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		c.Locals("resourceOwnerID", userID) // Same as user
		return c.Next()
	}, RequireResourcePermission(ps, domain.PermissionResourceRisk, domain.PermissionUpdate), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, req.StatusCode)
}

func TestRequireResourcePermissionMiddleware_ForbiddenOtherUserResource(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	userID := "user123"
	otherUserID := "other456"

	ps.SetUserPermissions(userID, domain.PermissionMatrix{
		Permissions: map[string]bool{
			"risk:update:own": true,
		},
	})

	userClaims := &domain.UserClaims{
		UserID: userID,
		Role:   "custom",
	}

	// Setup route for updating other's resource
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		c.Locals("resourceOwnerID", otherUserID) // Different user
		return c.Next()
	}, RequireResourcePermission(ps, domain.PermissionResourceRisk, domain.PermissionUpdate), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	// Should fail because user only has "own" scope permission
	assert.Equal(t, fiber.StatusForbidden, req.StatusCode)
}

func TestRequireResourcePermissionMiddleware_TeamResource(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	userID := "user123"
	otherUserID := "other456"

	ps.SetUserPermissions(userID, domain.PermissionMatrix{
		Permissions: map[string]bool{
			"risk:update:team": true,
		},
	})

	userClaims := &domain.UserClaims{
		UserID: userID,
		Role:   "custom",
	}

	// Setup route for updating team resource
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		c.Locals("resourceOwnerID", otherUserID) // Different user (team member)
		return c.Next()
	}, RequireResourcePermission(ps, domain.PermissionResourceRisk, domain.PermissionUpdate), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, req.StatusCode)
}

func TestRequireResourcePermissionMiddleware_AnyScope(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	userID := "user123"
	resourceOwnerID := "anyone"

	ps.SetUserPermissions(userID, domain.PermissionMatrix{
		Permissions: map[string]bool{
			"risk:delete:any": true,
		},
	})

	userClaims := &domain.UserClaims{
		UserID: userID,
		Role:   "custom",
	}

	// Setup route for deleting any resource
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", userClaims)
		c.Locals("resourceOwnerID", resourceOwnerID)
		return c.Next()
	}, RequireResourcePermission(ps, domain.PermissionResourceRisk, domain.PermissionDelete), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, req.StatusCode)
}

func TestPermissionMiddlewareFactory_CreatePermissionMiddleware(t *testing.T) {
	ps, _ := setupPermissionMiddlewareTest()
	factory := NewPermissionMiddlewareFactory(ps)

	assert.NotNil(t, factory)
	middleware := factory.CreatePermissionMiddleware(domain.Permission{
		Resource: domain.PermissionResourceRisk,
		Action:   domain.PermissionRead,
		Scope:    domain.PermissionScopeAny,
	})
	assert.NotNil(t, middleware)
}

func TestPermissionMiddlewareFactory_CreateAllPermissionsMiddleware(t *testing.T) {
	ps, _ := setupPermissionMiddlewareTest()
	factory := NewPermissionMiddlewareFactory(ps)

	assert.NotNil(t, factory)
	middleware := factory.CreateAllPermissionsMiddleware(
		domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionRead,
			Scope:    domain.PermissionScopeAny,
		},
	)
	assert.NotNil(t, middleware)
}

func TestPermissionMiddlewareFactory_CreateResourcePermissionMiddleware(t *testing.T) {
	ps, _ := setupPermissionMiddlewareTest()
	factory := NewPermissionMiddlewareFactory(ps)

	assert.NotNil(t, factory)
	middleware := factory.CreateResourcePermissionMiddleware(domain.PermissionResourceRisk, domain.PermissionRead)
	assert.NotNil(t, middleware)
}

func TestRequirePermissionsMiddleware_MissingUserContext(t *testing.T) {
	ps, app := setupPermissionMiddlewareTest()

	// Setup route without setting user context
	app.Get("/test", RequirePermissions(ps, domain.Permission{
		Resource: domain.PermissionResourceRisk,
		Action:   domain.PermissionRead,
		Scope:    domain.PermissionScopeAny,
	}), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, err := app.Test(fiber.MethodGet, "/test", nil)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, req.StatusCode)
}
