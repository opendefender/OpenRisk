package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"gorm.io/gorm"
)

type ScoreEngineHandler struct {
	db      *gorm.DB
	service *services.ScoreEngineService
}

// NewScoreEngineHandler crée un nouvel handler pour le Score Engine
func NewScoreEngineHandler(db *gorm.DB, service *services.ScoreEngineService) *ScoreEngineHandler {
	return &ScoreEngineHandler{
		db:      db,
		service: service,
	}
}

// CreateScoringConfigInput représente la structure pour créer une configuration
type CreateScoringConfigInput struct {
	Name                  string                             `json:"name" validate:"required"`
	Description           string                             `json:"description"`
	BaseFormula           string                             `json:"base_formula" validate:"required"`
	WeightingFactors      map[string]float64                 `json:"weighting_factors"`
	RiskMatrixThresholds  map[string]int                     `json:"risk_matrix_thresholds" validate:"required"`
	AssetCriticalityMult  map[string]float64                 `json:"asset_criticality_mult"`
}

// GetScoringConfigs godoc
// @Summary Récupérer toutes les configurations de scoring
// @Description Retourne la liste des configurations de scoring disponibles
// @Tags Score Engine
// @Accept json
// @Produce json
// @Success 200 {array} services.ScoringConfig
// @Router /api/v1/score-engine/configs [get]
func (h *ScoreEngineHandler) GetScoringConfigs(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Scoring configurations retrieved successfully",
		"default": h.service.DefaultScoringConfig(),
	})
}

// GetScoringConfig godoc
// @Summary Récupérer une configuration de scoring
// @Description Récupère les détails d'une configuration de scoring spécifique
// @Tags Score Engine
// @Accept json
// @Produce json
// @Param id path string true "Configuration ID"
// @Success 200 {object} services.ScoringConfig
// @Router /api/v1/score-engine/configs/{id} [get]
func (h *ScoreEngineHandler) GetScoringConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Configuration ID is required"})
	}

	config := h.service.GetConfig(id)
	if config == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Configuration not found"})
	}

	return c.JSON(config)
}

// CreateScoringConfig godoc
// @Summary Créer une nouvelle configuration de scoring
// @Description Crée une configuration personnalisée pour le calcul de score
// @Tags Score Engine
// @Accept json
// @Produce json
// @Param config body CreateScoringConfigInput true "Configuration data"
// @Success 201 {object} services.ScoringConfig
// @Router /api/v1/score-engine/configs [post]
func (h *ScoreEngineHandler) CreateScoringConfig(c *fiber.Ctx) error {
	input := new(CreateScoringConfigInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate input
	if input.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Configuration name is required"})
	}

	if input.BaseFormula == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Base formula is required"})
	}

	if input.RiskMatrixThresholds == nil || len(input.RiskMatrixThresholds) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Risk matrix thresholds are required"})
	}

	// Create configuration object
	config := &services.ScoringConfig{
		ID:                   c.Query("id", ""),
		Name:                 input.Name,
		Description:          input.Description,
		BaseFormula:          input.BaseFormula,
		WeightingFactors:     input.WeightingFactors,
		RiskMatrixThresholds: input.RiskMatrixThresholds,
	}

	// Convert asset criticality multipliers if provided
	if input.AssetCriticalityMult != nil {
		config.AssetCriticalityMult = make(map[domain.AssetCriticality]float64)
		for key, val := range input.AssetCriticalityMult {
			criticality := domain.AssetCriticality(key)
			config.AssetCriticalityMult[criticality] = val
		}
	}

	// Validate configuration
	if err := h.service.ValidateConfig(config); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Create in service
	if err := h.service.CreateConfig(config); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(config)
}

// UpdateScoringConfig godoc
// @Summary Mettre à jour une configuration de scoring
// @Description Modifie les paramètres d'une configuration existante
// @Tags Score Engine
// @Accept json
// @Produce json
// @Param id path string true "Configuration ID"
// @Param config body CreateScoringConfigInput true "Updated configuration data"
// @Success 200 {object} services.ScoringConfig
// @Router /api/v1/score-engine/configs/{id} [put]
func (h *ScoreEngineHandler) UpdateScoringConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Configuration ID is required"})
	}

	input := new(CreateScoringConfigInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Get existing config
	existing := h.service.GetConfig(id)
	if existing == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Configuration not found"})
	}

	// Create update object
	updates := &services.ScoringConfig{
		BaseFormula:          input.BaseFormula,
		WeightingFactors:     input.WeightingFactors,
		RiskMatrixThresholds: input.RiskMatrixThresholds,
	}

	if input.AssetCriticalityMult != nil {
		updates.AssetCriticalityMult = make(map[domain.AssetCriticality]float64)
		for key, val := range input.AssetCriticalityMult {
			criticality := domain.AssetCriticality(key)
			updates.AssetCriticalityMult[criticality] = val
		}
	}

	// Apply updates
	if err := h.service.UpdateConfig(id, updates); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	updated := h.service.GetConfig(id)
	return c.JSON(updated)
}

// ComputeRiskScoreInput représente les données pour calculer un score
type ComputeRiskScoreInput struct {
	Impact        int      `json:"impact" validate:"required,min=1,max=5"`
	Probability   int      `json:"probability" validate:"required,min=1,max=5"`
	AssetIDs      []string `json:"asset_ids"`
	ConfigID      string   `json:"config_id"`
	ApplyTrend    bool     `json:"apply_trend"`
	TrendFactor   float64  `json:"trend_factor"`
}

// ComputeRiskScore godoc
// @Summary Calculer le score de risque
// @Description Calcule le score de risque en utilisant une configuration spécifiée
// @Tags Score Engine
// @Accept json
// @Produce json
// @Param input body ComputeRiskScoreInput true "Risk calculation input"
// @Success 200 {object} fiber.Map
// @Router /api/v1/score-engine/compute [post]
func (h *ScoreEngineHandler) ComputeRiskScore(c *fiber.Ctx) error {
	input := new(ComputeRiskScoreInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate input
	if input.Impact < 1 || input.Impact > 5 {
		return c.Status(400).JSON(fiber.Map{"error": "Impact must be between 1 and 5"})
	}
	if input.Probability < 1 || input.Probability > 5 {
		return c.Status(400).JSON(fiber.Map{"error": "Probability must be between 1 and 5"})
	}

	// Get configuration
	config := h.service.GetConfig(input.ConfigID)
	if config == nil {
		config = h.service.DefaultScoringConfig()
	}

	// Load assets if provided
	var assets []*domain.Asset
	if len(input.AssetIDs) > 0 {
		if err := h.db.Where("id IN ?", input.AssetIDs).Find(&assets).Error; err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Failed to load assets"})
		}
	}

	// Compute base score
	baseScore := h.service.ComputeScoreWithConfig(input.Impact, input.Probability, assets, config)

	// Apply trend adjustment if requested
	finalScore := baseScore
	if input.ApplyTrend {
		finalScore = h.service.ApplyTrendAdjustment(baseScore, input.TrendFactor, config)
	}

	// Classify risk level
	riskLevel := h.service.ClassifyRiskLevel(finalScore, config)

	return c.JSON(fiber.Map{
		"base_score":   baseScore,
		"final_score":  finalScore,
		"risk_level":   riskLevel,
		"impact":       input.Impact,
		"probability":  input.Probability,
		"config_id":    config.ID,
		"asset_count":  len(assets),
	})
}

// GetRiskMatrix godoc
// @Summary Récupérer la matrice de risque
// @Description Retourne les seuils de classification pour les niveaux de risque
// @Tags Score Engine
// @Accept json
// @Produce json
// @Param config_id query string false "Configuration ID"
// @Success 200 {object} fiber.Map
// @Router /api/v1/score-engine/matrix [get]
func (h *ScoreEngineHandler) GetRiskMatrix(c *fiber.Ctx) error {
	configID := c.Query("config_id", "default")
	config := h.service.GetConfig(configID)

	matrix := h.service.GetRiskMatrix(config)

	return c.JSON(fiber.Map{
		"matrix":      matrix,
		"config_id":   config.ID,
		"formula":     config.BaseFormula,
		"weighting":   config.WeightingFactors,
		"criticality": config.AssetCriticalityMult,
	})
}

// ClassifyRiskInput représente les données pour classer un risque
type ClassifyRiskInput struct {
	Score    float64 `json:"score" validate:"required,min=0"`
	ConfigID string  `json:"config_id"`
}

// ClassifyRisk godoc
// @Summary Classer un niveau de risque
// @Description Classe le niveau de risque basé sur un score
// @Tags Score Engine
// @Accept json
// @Produce json
// @Param input body ClassifyRiskInput true "Risk classification input"
// @Success 200 {object} fiber.Map
// @Router /api/v1/score-engine/classify [post]
func (h *ScoreEngineHandler) ClassifyRisk(c *fiber.Ctx) error {
	input := new(ClassifyRiskInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if input.Score < 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Score must be non-negative"})
	}

	config := h.service.GetConfig(input.ConfigID)
	level := h.service.ClassifyRiskLevel(input.Score, config)

	return c.JSON(fiber.Map{
		"score":      input.Score,
		"risk_level": level,
		"config_id":  config.ID,
		"matrix":     config.RiskMatrixThresholds,
	})
}

// GetScoringMetrics godoc
// @Summary Récupérer les métriques de scoring
// @Description Retourne les statistiques de la formule de scoring
// @Tags Score Engine
// @Produce json
// @Success 200 {object} fiber.Map
// @Router /api/v1/score-engine/metrics [get]
func (h *ScoreEngineHandler) GetScoringMetrics(c *fiber.Ctx) error {
	config := h.service.DefaultScoringConfig()

	// Count risks by level
	type RiskStats struct {
		Level string
		Count int64
	}

	var stats []RiskStats
	h.db.Model(&domain.Risk{}).
		Select("CASE "+
			"WHEN score >= ? THEN 'critical' "+
			"WHEN score >= ? THEN 'high' "+
			"WHEN score >= ? THEN 'medium' "+
			"ELSE 'low' END as level, "+
			"COUNT(*) as count",
			config.RiskMatrixThresholds["critical"],
			config.RiskMatrixThresholds["high"],
			config.RiskMatrixThresholds["medium"]).
		Group("level").
		Scan(&stats)

	// Calculate average score
	var avgScore float64
	h.db.Model(&domain.Risk{}).Select("AVG(score)").Row().Scan(&avgScore)

	// Calculate distribution
	var maxScore float64
	h.db.Model(&domain.Risk{}).Select("MAX(score)").Row().Scan(&maxScore)

	return c.JSON(fiber.Map{
		"avg_score":   avgScore,
		"max_score":   maxScore,
		"risk_stats":  stats,
		"formula":     config.BaseFormula,
		"thresholds":  config.RiskMatrixThresholds,
	})
}

// RegisterScoreEngineRoutes enregistre les routes du Score Engine
func RegisterScoreEngineRoutes(app *fiber.App, db *gorm.DB, service *services.ScoreEngineService) {
	handler := NewScoreEngineHandler(db, service)

	// Configuration management
	app.Get("/api/v1/score-engine/configs", handler.GetScoringConfigs)
	app.Get("/api/v1/score-engine/configs/:id", handler.GetScoringConfig)
	app.Post("/api/v1/score-engine/configs", handler.CreateScoringConfig)
	app.Put("/api/v1/score-engine/configs/:id", handler.UpdateScoringConfig)

	// Score computation
	app.Post("/api/v1/score-engine/compute", handler.ComputeRiskScore)
	app.Get("/api/v1/score-engine/matrix", handler.GetRiskMatrix)
	app.Post("/api/v1/score-engine/classify", handler.ClassifyRisk)
	app.Get("/api/v1/score-engine/metrics", handler.GetScoringMetrics)
}
