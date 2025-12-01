package handlers

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// CreateRiskInput : DTO pour séparer la logique API de la logique DB
// Permet de recevoir une liste d'IDs d'assets (strings) au lieu d'objets complets
type CreateRiskInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Impact      int      `json:"impact"`
	Probability int      `json:"probability"`
	Tags        []string `json:"tags"`
	AssetIDs    []string `json:"asset_ids"` // Liste des UUIDs des assets concernés
}

// UpdateRiskInput : DTO pour la mise à jour partielle
type UpdateRiskInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Impact      int      `json:"impact"`
	Probability int      `json:"probability"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
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

	// 2. Mapping DTO -> Domain Entity
	risk := domain.Risk{
		Title:       input.Title,
		Description: input.Description,
		Impact:      input.Impact,
		Probability: input.Probability,
		Tags:        input.Tags,
		Status:      domain.StatusDraft, // Statut par défaut
		Source:      "MANUAL",           // Créé via l'UI
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

	// 4. Sauvegarde en base
	// Note: Le Hook BeforeSave dans le modèle Risk calculera automatiquement le Score
	if err := database.DB.Create(&risk).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not create risk"})
	}

	// 5. Retourne l'objet créé avec son ID et son Score calculé
	// On recharge pour être sûr d'avoir les relations si besoin (optionnel ici, mais propre)
	return c.Status(201).JSON(risk)
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
		Preload("Assets").
		Order("score desc")

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
		if v, err := strconv.Atoi(minScoreStr); err == nil {
			db = db.Where("score >= ?", v)
		}
	}

	if maxScoreStr != "" {
		if v, err := strconv.Atoi(maxScoreStr); err == nil {
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

	// Si Impact ou Proba change, le hook BeforeSave recalculera le Score
	if input.Impact != 0 {
		risk.Impact = input.Impact
	}
	if input.Probability != 0 {
		risk.Probability = input.Probability
	}

	// 4. Sauvegarde
	if err := database.DB.Save(&risk).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not update risk"})
	}

	return c.JSON(risk)
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
