package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/repositories"
)

// CreateRisk godoc
// @Summary Créer un nouveau risque
// @Description Ajoute un risque et calcule automatiquement son score.
func CreateRisk(c *fiber.Ctx) error {
	risk := new(domain.Risk)

	// Parsing du JSON reçu
	if err := c.BodyParser(risk); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input", "details": err.Error()})
	}

	// Sauvegarde (Le Hook BeforeSave calculera le score)
	if err := repositories.CreateRisk(risk); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not create risk"})
	}

	// Retourne le risque créé avec son score calculé
	return c.Status(201).JSON(risk)
}

// GetRisks godoc
// @Summary Lister les risques
// @Description Retourne la liste des risques triés par criticité.
func GetRisks(c *fiber.Ctx) error {
	risks, err := repositories.GetAllRisks()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not fetch risks"})
	}
	return c.JSON(risks)
}