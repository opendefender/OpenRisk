package handlers

import (
	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// GetAssets : Liste avec pagination optionnelle (simplifi√e ici)
func GetAssets(c fiber.Ctx) error {
	var assets []domain.Asset
	// On pr√charge les risques pour afficher le nombre de risques par asset
	if err := database.DB.Preload("Risks").Find(&assets).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Could not fetch assets"})
	}
	return c.JSON(assets)
}

// CreateAsset : Manuel (ou via Sync)
func CreateAsset(c fiber.Ctx) error {
	asset := new(domain.Asset)
	if err := c.BodyParser(asset); err != nil {
		return c.Status().JSON(fiber.Map{"error": "Invalid input"})
	}

	if err := database.DB.Create(asset).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Could not create asset"})
	}
	return c.Status().JSON(asset)
}