package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/services"
)

// TokenAuth provides token-based authentication middleware
type TokenAuth struct {
	tokenService *services.TokenService
}

// NewTokenAuth creates a new token authentication middleware
func NewTokenAuth(tokenService *services.TokenService) *TokenAuth {
	return &TokenAuth{
		tokenService: tokenService,
	}
}

// ExtractTokenFromRequest extracts the token from the request
// Supports Bearer token in Authorization header: "Bearer <token>"
func (ta *TokenAuth) ExtractTokenFromRequest(c *fiber.Ctx) (string, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header is missing")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return "", errors.New("authorization header format is invalid")
	}

	if parts[0] != "Bearer" {
		return "", errors.New("authorization scheme must be Bearer")
	}

	token := parts[1]
	if token == "" {
		return "", errors.New("authorization token is empty")
	}

	return token, nil
}

// Verify is a middleware that verifies API tokens
// Usage: app.Use(tokenAuth.Verify)
func (ta *TokenAuth) Verify(c *fiber.Ctx) error {
	tokenValue, err := ta.ExtractTokenFromRequest(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid authorization header",
		})
	}

	// Verify token
	token, err := ta.tokenService.VerifyToken(tokenValue)
	if err != nil || token == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired token",
		})
	}

	// Check if token is valid
	if !token.IsValid() {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "token is not valid for use",
		})
	}

	// Check IP whitelist if configured
	if len(token.IPWhitelist) > 0 && !token.IsIPAllowed(c.IP()) {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"error": "client IP is not whitelisted for this token",
		})
	}

	// Note: Last used timestamp is already updated by VerifyToken

	// Store in context for handlers
	c.Locals("userID", token.UserID)
	c.Locals("tokenID", token.ID)
	c.Locals("tokenPermissions", token.Permissions)
	c.Locals("tokenType", token.Type)

	return c.Next()
}

// RequireTokenPermission checks if the token has a specific permission
// Usage: app.Get("/resource", tokenAuth.RequireTokenPermission("resource:action:scope"))
func (ta *TokenAuth) RequireTokenPermission(requiredPermission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenID, ok := c.Locals("tokenID").(uuid.UUID)
		if !ok {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "token not authenticated",
			})
		}

		token, err := ta.tokenService.GetToken(tokenID)
		if err != nil || token == nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "token not found",
			})
		}

		if !token.HasPermission(requiredPermission) {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": "token does not have required permission",
			})
		}

		return c.Next()
	}
}

// RequireTokenScope checks if the token has a specific scope
// Usage: app.Get("/risks", tokenAuth.RequireTokenScope("risk"))
func (ta *TokenAuth) RequireTokenScope(requiredScope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenID, ok := c.Locals("tokenID").(uuid.UUID)
		if !ok {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "token not authenticated",
			})
		}

		token, err := ta.tokenService.GetToken(tokenID)
		if err != nil || token == nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "token not found",
			})
		}

		if !token.HasScope(requiredScope) {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": "token does not have required scope",
			})
		}

		return c.Next()
	}
}

// Combined middleware: Verify + RequireTokenPermission
// Usage: app.Get("/resource", tokenAuth.VerifyAndRequirePermission("resource:action:scope"))
func (ta *TokenAuth) VerifyAndRequirePermission(requiredPermission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// First verify token
		if err := ta.Verify(c); err != nil {
			return err
		}

		// Then check permission
		return ta.RequireTokenPermission(requiredPermission)(c)
	}
}

// Combined middleware: Verify + RequireTokenScope
// Usage: app.Get("/risks", tokenAuth.VerifyAndRequireScope("risk"))
func (ta *TokenAuth) VerifyAndRequireScope(requiredScope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// First verify token
		if err := ta.Verify(c); err != nil {
			return err
		}

		// Then check scope
		return ta.RequireTokenScope(requiredScope)(c)
	}
}
