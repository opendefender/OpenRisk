package handlers

import (
	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/internal/services"
)

func GetMyGamificationProfile(c fiber.Ctx) error {
	// R√cup√rer l'ID utilisateur depuis le Token JWT (inject√ par middleware Protected)
	userID := c.Locals("user_id").(string)

	if userID == "" {
		return c.Status().JSON(fiber.Map{"error": "User not found in token"})
	}

	service := services.NewGamificationService()
	stats, err := service.GetUserStats(userID)

	if err != nil {
		return c.Status().JSON(fiber.Map{"error": "Failed to calculate stats", "details": err.Error()})
	}

	return c.JSON(stats)
}
