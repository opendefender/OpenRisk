package handler

import (
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

// AddMitigation ajoute une action corrective à un risque
func AddMitigation(c *fiber.Ctx) error {
	riskID := c.Params("id")

	// Validation UUID
	if _, err := uuid.Parse(riskID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid Risk ID"})
	}

	// NEW: Get organization context for multi-tenancy
	ctx := middleware.GetContext(c)

	// NEW: Verify risk exists in the organization
	var risk domain.Risk
	query := database.DB
	if ctx != nil {
		query = query.Where("organization_id = ?", ctx.OrganizationID)
	}
	if err := query.First(&risk, "id = ?", riskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Risk not found"})
	}

	mitigation := new(domain.Mitigation)
	if err := c.BodyParser(mitigation); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Lier au risque
	mitigation.RiskID = uuid.MustParse(riskID)
	mitigation.Status = domain.MitigationPlanned
	// NEW: Add organization_id
	if ctx != nil {
		mitigation.OrganizationID = ctx.OrganizationID
	}

	if err := database.DB.Create(mitigation).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not create mitigation"})
	}

	return c.Status(201).JSON(mitigation)
}

// ToggleMitigationStatus change le statut (PLANNED <-> DONE)
func ToggleMitigationStatus(c *fiber.Ctx) error {
	mitigationID := c.Params("mitigationId")
	var mitigation domain.Mitigation

	// NEW: Get organization context for multi-tenancy
	ctx := middleware.GetContext(c)

	// NEW: Filter by organization_id if available
	query := database.DB
	if ctx != nil {
		query = query.Where("organization_id = ?", ctx.OrganizationID)
	}
	if err := query.First(&mitigation, "id = ?", mitigationID).Error; err != nil {
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
	service := service.NewRecommendationService()

	ctx := middleware.GetContext(c)
	tenantID := ""
	if ctx != nil && ctx.OrganizationID != uuid.Nil {
		tenantID = ctx.OrganizationID.String()
	} else {
		// Fallback for unified tenant approach
		val := c.Locals("tenant_id")
		if u, ok := val.(uuid.UUID); ok {
			tenantID = u.String()
		} else if s, ok := val.(string); ok {
			tenantID = s
		}
	}

	// 1. Récupérer et calculer les priorités
	mitigations, err := service.GetPrioritizedMitigations(tenantID)
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

// UpdateMitigation met à jour les champs éditables d'une mitigation
func UpdateMitigation(c *fiber.Ctx) error {
	mitigationID := c.Params("mitigationId")
	var mitigation domain.Mitigation

	// NEW: Get organization context for multi-tenancy
	ctx := middleware.GetContext(c)

	// NEW: Filter by organization_id if available
	query := database.DB
	if ctx != nil {
		query = query.Where("organization_id = ?", ctx.OrganizationID)
	}
	if err := query.First(&mitigation, "id = ?", mitigationID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mitigation not found"})
	}

	// Parse payload
	payload := struct {
		Title          *string `json:"title"`
		Assignee       *string `json:"assignee"`
		Status         *string `json:"status"`
		Progress       *int    `json:"progress"`
		DueDate        *string `json:"due_date"`
		Cost           *int    `json:"cost"`
		MitigationTime *int    `json:"mitigation_time"`
	}{}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
	}

	if payload.Title != nil {
		mitigation.Title = *payload.Title
	}
	if payload.Assignee != nil {
		mitigation.Assignee = *payload.Assignee
	}
	if payload.Status != nil {
		mitigation.Status = domain.MitigationStatus(*payload.Status)
	}
	if payload.Progress != nil {
		mitigation.Progress = *payload.Progress
	}
	if payload.Cost != nil {
		mitigation.Cost = *payload.Cost
	}
	if payload.MitigationTime != nil {
		mitigation.MitigationTime = *payload.MitigationTime
	}
	if payload.DueDate != nil {
		// try parse RFC3339
		if t, err := time.Parse(time.RFC3339, *payload.DueDate); err == nil {
			mitigation.DueDate = t
		}
	}

	if err := database.DB.Save(&mitigation).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not update mitigation"})
	}

	return c.JSON(mitigation)
}

// CreateMitigationSubAction ajoute une sous-action (checklist) à une mitigation
func CreateMitigationSubAction(c *fiber.Ctx) error {
	mitigationID := c.Params("id")
	if _, err := uuid.Parse(mitigationID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid mitigation ID"})
	}

	// NEW: Get organization context for multi-tenancy
	ctx := middleware.GetContext(c)

	payload := struct {
		Title string `json:"title"`
	}{}
	if err := c.BodyParser(&payload); err != nil || payload.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
	}

	// Ensure mitigation exists
	var m domain.Mitigation
	query := database.DB
	if ctx != nil {
		query = query.Where("organization_id = ?", ctx.OrganizationID)
	}
	if err := query.First(&m, "id = ?", mitigationID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mitigation not found"})
	}

	sa := domain.MitigationSubAction{
		MitigationID: uuid.MustParse(mitigationID),
		Title:        payload.Title,
	}

	if err := database.DB.Create(&sa).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not create sub-action"})
	}

	return c.Status(201).JSON(sa)
}

// ToggleMitigationSubAction bascule l'état d'une sous-action
func ToggleMitigationSubAction(c *fiber.Ctx) error {
	subID := c.Params("subactionId")
	// NEW: Get organization context for multi-tenancy
	ctx := middleware.GetContext(c)

	var sa domain.MitigationSubAction
	if err := database.DB.First(&sa, "id = ?", subID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Sub-action not found"})
	}

	// NEW: Verify mitigation belongs to the organization
	if ctx != nil {
		var mitigation domain.Mitigation
		if err := database.DB.Where("organization_id = ?", ctx.OrganizationID).First(&mitigation, "id = ?", sa.MitigationID).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Sub-action not found"})
		}
	}

	// If route contains mitigation id, verify ownership to avoid mismatch
	if mid := c.Params("id"); mid != "" {
		if _, err := uuid.Parse(mid); err == nil {
			if sa.MitigationID.String() != mid {
				return c.Status(404).JSON(fiber.Map{"error": "Sub-action not found for given mitigation"})
			}
		}
	}

	sa.Completed = !sa.Completed
	if err := database.DB.Save(&sa).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not toggle sub-action"})
	}

	return c.JSON(sa)
}

// DeleteMitigationSubAction supprime une sous-action
func DeleteMitigationSubAction(c *fiber.Ctx) error {
	subID := c.Params("subactionId")

	// Verify ownership if mitigation id present in path
	if mid := c.Params("id"); mid != "" {
		if _, err := uuid.Parse(mid); err == nil {
			var sa domain.MitigationSubAction
			if err := database.DB.First(&sa, "id = ?", subID).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "Sub-action not found"})
			}
			if sa.MitigationID.String() != mid {
				return c.Status(404).JSON(fiber.Map{"error": "Sub-action not found for given mitigation"})
			}
		}
	}

	if result := database.DB.Delete(&domain.MitigationSubAction{}, "id = ?", subID); result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not delete sub-action"})
	} else if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Sub-action not found"})
	}
	return c.SendStatus(204)
}
