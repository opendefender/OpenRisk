package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

func safeGetUUIDGamification(c *fiber.Ctx, key string) string {
	val := c.Locals(key)
	if val == nil {
		return ""
	}
	if u, ok := val.(string); ok {
		return u
	}
	// Add other checks if necessary, but returning string representation
	return fmt.Sprintf("%v", val)
}

func GetMyGamificationProfile(c *fiber.Ctx) error {
	// Safe type extraction — no more panic on type assertion
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	userID := claims.ID.String()
	tenantID := safeGetUUIDGamification(c, "tenant_id")

	svc := service.NewGamificationService()
	stats, err := svc.GetUserStats(userID, tenantID)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate stats"})
	}

	return c.JSON(stats)
}
