package middleware

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

// Protected middleware verifies JWT token validity
// Use this to protect routes that require authentication
func Protected() fiber.Handler {
	jwtSecret := os.Getenv("JWT_SECRET")
	return AuthMiddleware(jwtSecret)
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
