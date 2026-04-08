package handler

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/validation"
)

// RiskHandler encapsulates the risk use cases.
type RiskHandler struct {
	createRiskUseCase *risk.CreateRiskUseCase
	getRiskUseCase    *risk.GetRiskUseCase
	listRisksUseCase  *risk.ListRisksUseCase
	updateRiskUseCase *risk.UpdateRiskUseCase
	deleteRiskUseCase *risk.DeleteRiskUseCase
}

func NewRiskHandler(
	createRisk *risk.CreateRiskUseCase,
	getRisk *risk.GetRiskUseCase,
	listRisks *risk.ListRisksUseCase,
	updateRisk *risk.UpdateRiskUseCase,
	deleteRisk *risk.DeleteRiskUseCase,
) *RiskHandler {
	return &RiskHandler{
		createRiskUseCase: createRisk,
		getRiskUseCase:    getRisk,
		listRisksUseCase:  listRisks,
		updateRiskUseCase: updateRisk,
		deleteRiskUseCase: deleteRisk,
	}
}

// CreateRiskInput : DTO pour séparer la logique API de la logique DB
type CreateRiskInput struct {
	Title       string   `json:"title" validate:"required"`
	Description string   `json:"description"`
	Impact      int      `json:"impact" validate:"required,min=1,max=5"`
	Probability int      `json:"probability" validate:"required,min=1,max=5"`
	Tags        []string `json:"tags"`
	AssetIDs    []string `json:"asset_ids"` // Liste des UUIDs des assets concernés
	Frameworks  []string `json:"frameworks"`
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
	Frameworks  []string `json:"frameworks" validate:"omitempty,dive,required"`
}

// CreateRisk godoc
func (h *RiskHandler) CreateRisk(c *fiber.Ctx) error {
	input := new(CreateRiskInput)

	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   "Invalid input format",
			"details": err.Error(),
		})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	stdCtx := c.UserContext()

	mwCtx := middleware.GetContext(c)
	orgID := uuid.Nil
	if mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	ucInput := risk.CreateRiskInput{
		Title:       input.Title,
		Description: input.Description,
		Impact:      input.Impact,
		Probability: input.Probability,
		Tags:        input.Tags,
		Frameworks:  input.Frameworks,
	}

	domainRisk, err := h.createRiskUseCase.Execute(stdCtx, orgID, ucInput)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Link Assets (fallback until AssetRepo introduced)
	if len(input.AssetIDs) > 0 {
		var assets []*domain.Asset
		query := database.DB
		if mwCtx != nil {
			query = query.Where("organization_id = ?", mwCtx.OrganizationID)
		}
		if err := query.Where("id IN ?", input.AssetIDs).Find(&assets).Error; err == nil {
			domainRisk.Assets = assets
			// Recompute score
			domainRisk.Score = service.ComputeRiskScore(domainRisk.Impact, domainRisk.Probability, assets)
			// Save relationships and updated score
			database.DB.Save(domainRisk)
            database.DB.Model(&domainRisk).Association("Assets").Replace(assets)
		}
	}

	var out domain.Risk
	if err := database.DB.Preload("Mitigations").Preload("Mitigations.SubActions").Preload("Assets").First(&out, "id = ?", domainRisk.ID).Error; err != nil {
		return c.Status(201).JSON(domainRisk)
	}

	return c.Status(201).JSON(out)
}

// GetRisks godoc
func (h *RiskHandler) GetRisks(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	orgID := uuid.Nil
	if mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	query := domain.NewRiskQuery()

	if q := c.Query("q"); q != "" {
		query.Search = q
	}
	if status := c.Query("status"); status != "" {
		query.Status = status
	}
	if minScoreStr := c.Query("min_score"); minScoreStr != "" {
		if v, err := strconv.ParseFloat(minScoreStr, 64); err == nil {
			query.MinScore = &v
		}
	}
	if maxScoreStr := c.Query("max_score"); maxScoreStr != "" {
		if v, err := strconv.ParseFloat(maxScoreStr, 64); err == nil {
			query.MaxScore = &v
		}
	}
	if tag := c.Query("tag"); tag != "" {
		query.Tags = []string{tag}
	}

	// Page & Limit
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			query.Page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			query.Limit = l
		}
	}

	// Sorting
	sortBy := c.Query("sort_by")
	sortDir := strings.ToLower(c.Query("sort_dir"))

	if sortBy != "" {
		switch strings.ToLower(sortBy) {
		case "score", "title", "created_at", "updated_at", "impact", "probability", "status", "source":
			query.SortBy = sortBy
		case "newest":
			query.SortBy = "created_at"
			sortDir = "desc"
		case "oldest":
			query.SortBy = "created_at"
			sortDir = "asc"
		case "updated":
			query.SortBy = "updated_at"
		}
	}
	if sortDir == "asc" || sortDir == "desc" {
		query.SortOrder = sortDir
	}

	result, err := h.listRisksUseCase.Execute(c.UserContext(), orgID, query)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not fetch risks", "details": err.Error()})
	}

	return c.JSON(fiber.Map{"items": result.Data, "total": result.Total})
}

// GetRisk godoc
func (h *RiskHandler) GetRisk(c *fiber.Ctx) error {
	id := c.Params("id")
	riskID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}

	mwCtx := middleware.GetContext(c)
	orgID := uuid.Nil
	if mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	domainRisk, err := h.getRiskUseCase.Execute(c.UserContext(), orgID, riskID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Risk not found", "details": err.Error()})
	}

	return c.JSON(domainRisk)
}

// UpdateRisk godoc
func (h *RiskHandler) UpdateRisk(c *fiber.Ctx) error {
	id := c.Params("id")
	riskID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}

	input := new(UpdateRiskInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	mwCtx := middleware.GetContext(c)
	orgID := uuid.Nil
	if mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	ucInput := risk.UpdateRiskInput{
		Title:       &input.Title,
		Description: &input.Description,
		Impact:      &input.Impact,
		Probability: &input.Probability,
		Tags:        input.Tags,
		Frameworks:  input.Frameworks,
	}

	if input.Title == "" {
		ucInput.Title = nil
	}
	if input.Description == "" {
		ucInput.Description = nil
	}
	if input.Impact == 0 {
		ucInput.Impact = nil
	}
	if input.Probability == 0 {
		ucInput.Probability = nil
	}
	if input.Status != "" {
		s := domain.RiskStatus(input.Status)
		ucInput.Status = &s
	}

	domainRisk, err := h.updateRiskUseCase.Execute(c.UserContext(), orgID, riskID, ucInput)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Could not update risk", "details": err.Error()})
	}

	if len(input.AssetIDs) > 0 {
		var assets []*domain.Asset
		query := database.DB
		if mwCtx != nil {
			query = query.Where("organization_id = ?", mwCtx.OrganizationID)
		}
		if err := query.Where("id IN ?", input.AssetIDs).Find(&assets).Error; err == nil {
			domainRisk.Assets = assets
			domainRisk.Score = service.ComputeRiskScore(domainRisk.Impact, domainRisk.Probability, assets)
			
			database.DB.Save(domainRisk)
			database.DB.Model(&domainRisk).Association("Assets").Replace(assets)
		}
	}

	var out domain.Risk
	if err := database.DB.Preload("Mitigations").Preload("Mitigations.SubActions").Preload("Assets").First(&out, "id = ?", riskID).Error; err != nil {
		return c.JSON(domainRisk)
	}

	return c.JSON(out)
}

// DeleteRisk godoc
func (h *RiskHandler) DeleteRisk(c *fiber.Ctx) error {
	id := c.Params("id")
	riskID, err := uuid.Parse(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}

	mwCtx := middleware.GetContext(c)
	orgID := uuid.Nil
	if mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}

	err = h.deleteRiskUseCase.Execute(c.UserContext(), orgID, riskID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not delete risk", "details": err.Error()})
	}

	return c.SendStatus(204)
}
