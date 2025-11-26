package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"sort"
)

// AddMitigation ajoute une action corrective à un risque
func AddMitigation(c *fiber.Ctx) error {
	riskID := c.Params("id")
	
	// Validation UUID
	if _, err := uuid.Parse(riskID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid Risk ID"})
	}

	mitigation := new(domain.Mitigation)
	if err := c.BodyParser(mitigation); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Lier au risque
	mitigation.RiskID = uuid.MustParse(riskID)
	mitigation.Status = domain.MitigationPlanned

	if err := database.DB.Create(mitigation).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not create mitigation"})
	}

	return c.Status(201).JSON(mitigation)
}

// ToggleMitigationStatus change le statut (PLANNED <-> DONE)
func ToggleMitigationStatus(c *fiber.Ctx) error {
	mitigationID := c.Params("mitigationId")
	var mitigation domain.Mitigation

	if err := database.DB.First(&mitigation, "id = ?", mitigationID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mitigation not found"})
	}

	// Logique de bascule simple
	if mitigation.Status == domain.MitigationDone {
		mitigation.Status = domain.MitigationInProgress
		mitigation.Progress = 50
	} else {
		mitigation.Status = domain.MitigationDone
		mitigation.Progress = 100
	}

	database.DB.Save(&mitigation)
	return c.JSON(mitigation)
}

// GetRecommendedMitigations expose la liste des mitigations triées par SPP.
func GetRecommendedMitigations(c *fiber.Ctx) error {
	service := services.NewRecommendationService()
	
	// 1. Récupérer et calculer les priorités
	mitigations, err := service.GetPrioritizedMitigations()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get prioritized mitigations"})
	}

	// 2. Trier la liste dans le Handler avant l'envoi (meilleure pratique)
	// On veut le SPP le plus élevé en premier.
	sort.Slice(mitigations, func(i, j int) bool {
		return mitigations[i].WeightedPriority > mitigations[j].WeightedPriority
	})

	return c.JSON(mitigations)
}