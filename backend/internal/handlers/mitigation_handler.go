package handlers

import (
	"sort"
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// AddMitigation ajoute une action corrective Ã  un risque
func AddMitigation(c fiber.Ctx) error {
	riskID := c.Params("id")

	// Validation UUID
	if _, err := uuid.Parse(riskID); err != nil {
		return c.Status().JSON(fiber.Map{"error": "Invalid Risk ID"})
	}

	mitigation := new(domain.Mitigation)
	if err := c.BodyParser(mitigation); err != nil {
		return c.Status().JSON(fiber.Map{"error": "Invalid input"})
	}

	// Lier au risque
	mitigation.RiskID = uuid.MustParse(riskID)
	mitigation.Status = domain.MitigationPlanned

	if err := database.DB.Create(mitigation).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Could not create mitigation"})
	}

	return c.Status().JSON(mitigation)
}

// ToggleMitigationStatus change le statut (PLANNED <-> DONE)
func ToggleMitigationStatus(c fiber.Ctx) error {
	mitigationID := c.Params("mitigationId")
	var mitigation domain.Mitigation

	if err := database.DB.First(&mitigation, "id = ?", mitigationID).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Mitigation not found"})
	}

	// Logique de bascule simple
	if mitigation.Status == domain.MitigationDone {
		mitigation.Status = domain.MitigationInProgress
		mitigation.Progress = 
	} else {
		mitigation.Status = domain.MitigationDone
		mitigation.Progress = 
	}

	database.DB.Save(&mitigation)
	return c.JSON(mitigation)
}

// GetRecommendedMitigations expose la liste des mitigations triÃes par SPP.
func GetRecommendedMitigations(c fiber.Ctx) error {
	service := services.NewRecommendationService()

	// . RÃcupÃrer et calculer les prioritÃs
	mitigations, err := service.GetPrioritizedMitigations()
	if err != nil {
		return c.Status().JSON(fiber.Map{"error": "Failed to get prioritized mitigations"})
	}

	// . Trier la liste dans le Handler avant l'envoi (meilleure pratique)
	// On veut le SPP le plus ÃlevÃ en premier.
	sort.Slice(mitigations, func(i, j int) bool {
		return mitigations[i].WeightedPriority > mitigations[j].WeightedPriority
	})

	return c.JSON(mitigations)
}

// UpdateMitigation met Ã  jour les champs Ãditables d'une mitigation
func UpdateMitigation(c fiber.Ctx) error {
	mitigationID := c.Params("mitigationId")
	var mitigation domain.Mitigation

	if err := database.DB.First(&mitigation, "id = ?", mitigationID).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Mitigation not found"})
	}

	// Parse payload
	payload := struct {
		Title          string json:"title"
		Assignee       string json:"assignee"
		Status         string json:"status"
		Progress       int    json:"progress"
		DueDate        string json:"due_date"
		Cost           int    json:"cost"
		MitigationTime int    json:"mitigation_time"
	}{}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status().JSON(fiber.Map{"error": "Invalid payload"})
	}

	if payload.Title != nil {
		mitigation.Title = payload.Title
	}
	if payload.Assignee != nil {
		mitigation.Assignee = payload.Assignee
	}
	if payload.Status != nil {
		mitigation.Status = domain.MitigationStatus(payload.Status)
	}
	if payload.Progress != nil {
		mitigation.Progress = payload.Progress
	}
	if payload.Cost != nil {
		mitigation.Cost = payload.Cost
	}
	if payload.MitigationTime != nil {
		mitigation.MitigationTime = payload.MitigationTime
	}
	if payload.DueDate != nil {
		// try parse RFC
		if t, err := time.Parse(time.RFC, payload.DueDate); err == nil {
			mitigation.DueDate = t
		}
	}

	if err := database.DB.Save(&mitigation).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Could not update mitigation"})
	}

	return c.JSON(mitigation)
}

// CreateMitigationSubAction ajoute une sous-action (checklist) Ã  une mitigation
func CreateMitigationSubAction(c fiber.Ctx) error {
	mitigationID := c.Params("id")
	if _, err := uuid.Parse(mitigationID); err != nil {
		return c.Status().JSON(fiber.Map{"error": "Invalid mitigation ID"})
	}

	payload := struct {
		Title string json:"title"
	}{}
	if err := c.BodyParser(&payload); err != nil || payload.Title == "" {
		return c.Status().JSON(fiber.Map{"error": "Invalid payload"})
	}

	// Ensure mitigation exists
	var m domain.Mitigation
	if err := database.DB.First(&m, "id = ?", mitigationID).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Mitigation not found"})
	}

	sa := domain.MitigationSubAction{
		MitigationID: uuid.MustParse(mitigationID),
		Title:        payload.Title,
	}

	if err := database.DB.Create(&sa).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Could not create sub-action"})
	}

	return c.Status().JSON(sa)
}

// ToggleMitigationSubAction bascule l'Ãtat d'une sous-action
func ToggleMitigationSubAction(c fiber.Ctx) error {
	subID := c.Params("subactionId")
	var sa domain.MitigationSubAction
	if err := database.DB.First(&sa, "id = ?", subID).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Sub-action not found"})
	}

	// If route contains mitigation id, verify ownership to avoid mismatch
	if mid := c.Params("id"); mid != "" {
		if _, err := uuid.Parse(mid); err == nil {
			if sa.MitigationID.String() != mid {
				return c.Status().JSON(fiber.Map{"error": "Sub-action not found for given mitigation"})
			}
		}
	}

	sa.Completed = !sa.Completed
	if err := database.DB.Save(&sa).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Could not toggle sub-action"})
	}

	return c.JSON(sa)
}

// DeleteMitigationSubAction supprime une sous-action
func DeleteMitigationSubAction(c fiber.Ctx) error {
	subID := c.Params("subactionId")
	// Verify ownership if mitigation id present in path
	if mid := c.Params("id"); mid != "" {
		if _, err := uuid.Parse(mid); err == nil {
			var sa domain.MitigationSubAction
			if err := database.DB.First(&sa, "id = ?", subID).Error; err != nil {
				return c.Status().JSON(fiber.Map{"error": "Sub-action not found"})
			}
			if sa.MitigationID.String() != mid {
				return c.Status().JSON(fiber.Map{"error": "Sub-action not found for given mitigation"})
			}
		}
	}

	if result := database.DB.Delete(&domain.MitigationSubAction{}, "id = ?", subID); result.Error != nil {
		return c.Status().JSON(fiber.Map{"error": "Could not delete sub-action"})
	} else if result.RowsAffected ==  {
		return c.Status().JSON(fiber.Map{"error": "Sub-action not found"})
	}
	return c.SendStatus()
}
