package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"github.com/opendefender/openrisk/internal/validation"
)

// CreateRiskInput : DTO pour séparer la logique API de la logique DB
// Permet de recevoir une liste d'IDs d'assets (strings) au lieu d'objets complets
type CreateRiskInput struct {
	Title       string   `json:"title" validate:"required"`
	Description string   `json:"description"`
	Impact      int      `json:"impact" validate:"required,min=1,max=5"`
	Probability int      `json:"probability" validate:"required,min=1,max=5"`
	Tags        []string `json:"tags"`
	AssetIDs    []string `json:"asset_ids"` // Liste des UUIDs des assets concernés
	// New validation tags will be added here
	// Example: Tags        []string `json:"tags" validate:"omitempty,dive,required"`
	// Example: AssetIDs    []string `json:"asset_ids" validate:"omitempty,dive,uuid4"`
}

// UpdateRiskInput : DTO pour la mise à jour partielle
type UpdateRiskInput struct {
	Title       string   `json:"title" validate:"omitempty"`
	Description string   `json:"description" validate:"omitempty"`
	Impact      int      `json:"impact" validate:"omitempty,min=1,max=5"`
	Probability int      `json:"probability" validate:"omitempty,min=1,max=5"`
	Status      string   `json:"status" validate:"omitempty"`
	Tags        []string `json:"tags" validate:"omitempty,dive,required"`
	AssetIDs    []string `json:"asset_ids" validate:"omitempty,dive,uuid4"`
	// New validation tags will be added here
	// Example: Tags        []string `json:"tags" validate:"omitempty,dive,required"`
	// Example: AssetIDs    []string `json:"asset_ids" validate:"omitempty,dive,uuid4"`
}

// CreateRisk godoc
// @Summary Créer un nouveau risque
// @Description Ajoute un risque, calcule son score et lie les assets.
func CreateRisk(c *fiber.Ctx) error {
	input := new(CreateRiskInput)

	// 1. Validation de l'input JSON
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   "Invalid input format",
			"details": err.Error(),
		})
	}

	// 1b. Structured validation using validator tags
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	// 2. Basic validation
	if input.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Title is required"})
	}

	if input.Impact < 1 || input.Impact > 5 {
		return c.Status(400).JSON(fiber.Map{"error": "Impact must be between 1 and 5"})
	}
	if input.Probability < 1 || input.Probability > 5 {
		return c.Status(400).JSON(fiber.Map{"error": "Probability must be between 1 and 5"})
	}

	// 3. Mapping DTO -> Domain Entity
	risk := domain.Risk{
		Title:       input.Title,
		Description: input.Description,
		Impact:      input.Impact,
		Probability: input.Probability,
		Status:      domain.StatusDraft, // Statut par défaut
	}

	// Only set Tags if provided to avoid inserting NULL into databases that
	// do not have the tags column (tests using sqlite in-memory).
	if len(input.Tags) > 0 {
		risk.Tags = input.Tags
	}

	// 3. Gestion des relations Assets (Many-to-Many)
	if len(input.AssetIDs) > 0 {
		var assets []*domain.Asset
		// GORM est intelligent : "id IN ?" fonctionne avec un slice de strings
		result := database.DB.Where("id IN ?", input.AssetIDs).Find(&assets)
		if result.Error != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to verify assets"})
		}

		// On associe les objets Assets trouvés au Risque
		risk.Assets = assets
	}

	// 4. Compute final score using asset criticality and save
	final := services.ComputeRiskScore(risk.Impact, risk.Probability, risk.Assets)
	risk.Score = final

	// Build a list of optional columns to omit when empty to support sqlite test schema
	omit := []string{}
	if len(input.Tags) == 0 {
		omit = append(omit, "tags")
	}
	if risk.Owner == "" {
		omit = append(omit, "owner")
	}
	if risk.ExternalID == "" {
		omit = append(omit, "external_id")
	}
	if len(risk.Frameworks) == 0 {
		omit = append(omit, "frameworks")
	}
	// custom_fields is datatypes.JSON in production; omit when nil/empty
	omit = append(omit, "custom_fields")

	if len(omit) > 0 {
		if err := database.DB.Omit(omit...).Create(&risk).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not create risk"})
		}
	} else {
		if err := database.DB.Create(&risk).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not create risk"})
		}
	}

	// 5. Reload with relations for the response
	var out domain.Risk
	if err := database.DB.Preload("Mitigations").Preload("Assets").First(&out, "id = ?", risk.ID).Error; err != nil {
		return c.Status(201).JSON(risk) // fallback
	}

	return c.Status(201).JSON(out)
}

// GetRisks godoc
// @Summary Lister tous les risques
// @Description Récupère les risques triés par score décroissant (les plus critiques en premier).
func GetRisks(c *fiber.Ctx) error {
	var risks []domain.Risk

	// Supported query params: q, status, min_score, max_score, tag
	q := c.Query("q")
	status := c.Query("status")
	minScoreStr := c.Query("min_score")
	maxScoreStr := c.Query("max_score")
	tag := c.Query("tag")

	db := database.DB.Model(&domain.Risk{}).
		Preload("Mitigations").
		Preload("Assets")

	// Server-side sorting: safe-guard allowed fields and map friendly names
	sortBy := c.Query("sort_by")
	sortDir := strings.ToLower(c.Query("sort_dir"))
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc"
	}

	// Map friendly sort names to actual DB columns
	if sortBy != "" {
		switch strings.ToLower(sortBy) {
		case "score", "title", "created_at", "updated_at", "impact", "probability", "status", "source":
			// allowed as-is
		case "newest":
			sortBy = "created_at"
			sortDir = "desc"
		case "oldest":
			sortBy = "created_at"
			sortDir = "asc"
		case "updated":
			sortBy = "updated_at"
		default:
			// unknown friendly name -> fallback
			sortBy = "score"
			sortDir = "desc"
		}
	}

	// Default ordering
	orderClause := "score desc"
	if sortBy != "" {
		// whitelist sortable columns to avoid injection
		switch sortBy {
		case "score", "title", "created_at", "updated_at", "impact", "probability", "status", "source":
			orderClause = fmt.Sprintf("%s %s", sortBy, sortDir)
		default:
			orderClause = "score desc"
		}
	}
	db = db.Order(orderClause)

	// Pagination
	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	page := 1
	limit := 20
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	if q != "" {
		like := fmt.Sprintf("%%%s%%", q)
		db = db.Where("title ILIKE ? OR description ILIKE ?", like, like)
	}

	if status != "" {
		db = db.Where("status = ?", status)
	}

	if minScoreStr != "" {
		if v, err := strconv.ParseFloat(minScoreStr, 64); err == nil {
			db = db.Where("score >= ?", v)
		}
	}

	if maxScoreStr != "" {
		if v, err := strconv.ParseFloat(maxScoreStr, 64); err == nil {
			db = db.Where("score <= ?", v)
		}
	}

	if tag != "" {
		// check membership in tags array
		db = db.Where("? = ANY(tags)", tag)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not count risks"})
	}

	offset := (page - 1) * limit
	result := db.Limit(limit).Offset(offset).Find(&risks)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not fetch risks"})
	}

	return c.JSON(fiber.Map{"items": risks, "total": total})
}

// GetRisk godoc
// @Summary Récupérer un risque unique
// @Description Détails complets d'un risque par ID.
func GetRisk(c *fiber.Ctx) error {
	id := c.Params("id")
	if _, err := uuid.Parse(id); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}

	var risk domain.Risk
	result := database.DB.
		Preload("Mitigations").
		Preload("Assets").
		First(&risk, "id = ?", id)

	if result.Error != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Risk not found"})
	}

	return c.JSON(risk)
}

// UpdateRisk godoc
// @Summary Mettre à jour un risque
// @Description Mise à jour des champs (Titre, Score, Statut). Recalcule le score automatiquement.
func UpdateRisk(c *fiber.Ctx) error {
	id := c.Params("id")
	var risk domain.Risk

	// 1. Vérifier l'existence
	if err := database.DB.First(&risk, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Risk not found"})
	}

	// 2. Parser les nouvelles données
	input := new(UpdateRiskInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Structured validation for update payload
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	// 3. Mise à jour des champs (uniquement si fournis)
	if input.Title != "" {
		risk.Title = input.Title
	}
	if input.Description != "" {
		risk.Description = input.Description
	}
	if input.Status != "" {
		risk.Status = domain.RiskStatus(input.Status)
	}
	if len(input.Tags) > 0 {
		risk.Tags = input.Tags
	}

	// If AssetIDs provided, reload and attach assets before computing score
	if len(input.AssetIDs) > 0 {
		var assets []*domain.Asset
		if err := database.DB.Where("id IN ?", input.AssetIDs).Find(&assets).Error; err == nil {
			risk.Assets = assets
		}
	}

	// Si Impact ou Proba change, le hook BeforeSave recalculera le Score
	if input.Impact != 0 {
		risk.Impact = input.Impact
	}
	if input.Probability != 0 {
		risk.Probability = input.Probability
	}

	// 4. Recompute score with assets criticality and save
	final := services.ComputeRiskScore(risk.Impact, risk.Probability, risk.Assets)
	risk.Score = final

	omit := []string{}
	if len(input.Tags) == 0 {
		omit = append(omit, "tags")
	}
	if risk.Owner == "" {
		omit = append(omit, "owner")
	}
	if risk.ExternalID == "" {
		omit = append(omit, "external_id")
	}
	if len(risk.Frameworks) == 0 {
		omit = append(omit, "frameworks")
	}
	omit = append(omit, "custom_fields")

	if len(omit) > 0 {
		if err := database.DB.Omit(omit...).Save(&risk).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not update risk"})
		}
	} else {
		if err := database.DB.Save(&risk).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not update risk"})
		}
	}

	// Reload with relations for response
	var out domain.Risk
	if err := database.DB.Preload("Mitigations").Preload("Assets").First(&out, "id = ?", id).Error; err != nil {
		return c.JSON(risk)
	}

	return c.JSON(out)
}

// DeleteRisk godoc
// @Summary Supprimer un risque
// @Description Soft delete d'un risque.
func DeleteRisk(c *fiber.Ctx) error {
	id := c.Params("id")

	// Validation UUID
	if _, err := uuid.Parse(id); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}

	// Delete avec GORM (Soft Delete par défaut grâce au champ DeletedAt dans le modèle)
	result := database.DB.Delete(&domain.Risk{}, "id = ?", id)

	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not delete risk"})
	}

	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Risk not found"})
	}

	return c.SendStatus(204) // No Content
}
