// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/crq"
	"github.com/opendefender/openrisk/pkg/events"
	"github.com/opendefender/openrisk/pkg/validation"
)

// RiskHandler encapsulates the risk use cases.
type RiskHandler struct {
	createRiskUseCase *risk.CreateRiskUseCase
	getRiskUseCase    *risk.GetRiskUseCase
	listRisksUseCase  *risk.ListRisksUseCase
	updateRiskUseCase *risk.UpdateRiskUseCase
	deleteRiskUseCase *risk.DeleteRiskUseCase
	markReviewedUC    *risk.MarkRiskReviewedUseCase
	redisClient       *redis.Client
	crq               *crq.Quantifier // Cyber Risk Quantification (XAF + USD)
}

func NewRiskHandler(
	createRisk *risk.CreateRiskUseCase,
	getRisk *risk.GetRiskUseCase,
	listRisks *risk.ListRisksUseCase,
	updateRisk *risk.UpdateRiskUseCase,
	deleteRisk *risk.DeleteRiskUseCase,
	markReviewed *risk.MarkRiskReviewedUseCase,
	redisClient *redis.Client,
	quantifier *crq.Quantifier,
) *RiskHandler {
	return &RiskHandler{
		createRiskUseCase: createRisk,
		getRiskUseCase:    getRisk,
		listRisksUseCase:  listRisks,
		updateRiskUseCase: updateRisk,
		deleteRiskUseCase: deleteRisk,
		markReviewedUC:    markReviewed,
		redisClient:       redisClient,
		crq:               quantifier,
	}
}

// MarkReviewed POST /risks/:id/review — records a review now and reschedules the
// next one from the risk's cadence.
func (h *RiskHandler) MarkReviewed(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid UUID"})
	}
	mwCtx := middleware.GetContext(c)
	orgID := uuid.Nil
	if mwCtx != nil {
		orgID = mwCtx.OrganizationID
	}
	r, err := h.markReviewedUC.Execute(c.UserContext(), orgID, riskID)
	if err != nil {
		return writeAppError(c, err)
	}
	h.quantify(r)
	return c.JSON(r)
}

// quantify fills a risk's computed CRQ fields (ALE in XAF + USD, basis) from its
// SLE/ARO (or the reference model). Safe on nil.
func (h *RiskHandler) quantify(r *domain.Risk) {
	if r == nil || h.crq == nil {
		return
	}
	q := h.crq.Quantify(r.SLEXAF, r.ARO, string(r.Criticality))
	r.ALEXAF = q.ALE.XAF
	r.ALEUSD = q.ALE.USD
	r.ALEBasis = string(q.Basis)
}

// CreateRiskInput : DTO pour séparer la logique API de la logique DB
type CreateRiskInput struct {
	Title       string   `json:"title" validate:"required"`
	Description string   `json:"description"`
	Impact      float64  `json:"impact" validate:"required,min=0,max=10"`     // ERD numeric(5,1) — bounds [0,10]
	Probability float64  `json:"probability" validate:"required,min=0,max=1"` // ERD numeric(5,3) — bounds [0,1]
	Tags        []string `json:"tags"`
	AssetIDs    []string `json:"asset_ids"` // Liste des UUIDs des assets concernés
	Frameworks  []string `json:"frameworks"`
	// CRQ monetary inputs (XAF). Optional.
	SLEXAF *float64 `json:"sle_xaf" validate:"omitempty,min=0"`
	ARO    *float64 `json:"aro" validate:"omitempty,min=0"`
}

// UpdateRiskInput : DTO pour la mise à jour partielle
type UpdateRiskInput struct {
	Title       string   `json:"title" validate:"omitempty"`
	Description string   `json:"description" validate:"omitempty"`
	Impact      float64  `json:"impact" validate:"omitempty,min=0,max=10"`
	Probability float64  `json:"probability" validate:"omitempty,min=0,max=1"`
	Status      string   `json:"status" validate:"omitempty"`
	Tags        []string `json:"tags" validate:"omitempty,dive,required"`
	AssetIDs    []string `json:"asset_ids" validate:"omitempty,dive,uuid4"`
	Frameworks  []string `json:"frameworks" validate:"omitempty,dive,required"`
	// CRQ monetary inputs (XAF). Pointers → nil means "leave unchanged".
	SLEXAF *float64 `json:"sle_xaf" validate:"omitempty,min=0"`
	ARO    *float64 `json:"aro" validate:"omitempty,min=0"`
	// Review cadence in days (0 disables).
	ReviewIntervalDays *int `json:"review_interval_days" validate:"omitempty,min=0"`
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
		SLEXAF:      input.SLEXAF,
		ARO:         input.ARO,
	}

	domainRisk, err := h.createRiskUseCase.Execute(stdCtx, orgID, ucInput)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Link Assets (fallback until AssetRepo introduced)
	var linkedAssets []*domain.Asset
	if len(input.AssetIDs) > 0 {
		query := database.DB
		if mwCtx != nil {
			query = query.Where("organization_id = ?", mwCtx.OrganizationID)
		}
		if err := query.Where("id IN ?", input.AssetIDs).Find(&linkedAssets).Error; err == nil {
			domainRisk.Assets = linkedAssets
			// Save relationships (no direct score compute — publish Redis event instead)
			if err := database.DB.Model(&domainRisk).Association("Assets").Replace(linkedAssets); err != nil {
				log.Printf("Warning: failed to update asset associations for risk %s: %v", domainRisk.ID, err)
			}
		}
	}

	// RULE #12: Score Engine is NEVER called directly from handler.
	// Always publish Redis event → ScoreWorker listens and recalculates async,
	// using the real criticality of whichever assets were just linked instead
	// of a hardcoded placeholder.
	userID := uuid.Nil
	if mwCtx != nil {
		userID = mwCtx.UserID
	}
	if h.redisClient != nil {
		event := events.RiskUpdatedEvent{
			RiskID:           domainRisk.ID.String(),
			TenantID:         orgID.String(),
			Probability:      float64(domainRisk.Probability),
			Impact:           float64(domainRisk.Impact),
			AssetCriticality: averageAssetCriticalityFactor(linkedAssets),
			TriggeredBy:      userID.String(),
		}
		_ = h.redisClient.Publish(c.Context(), events.RiskUpdated, event)
	}

	var out domain.Risk
	if err := database.DB.Preload("Mitigations").Preload("Mitigations.SubActions").Preload("Assets").First(&out, "id = ?", domainRisk.ID).Error; err != nil {
		h.quantify(domainRisk)
		return c.Status(201).JSON(domainRisk)
	}

	h.quantify(&out)
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
		query.Status = []string{status}
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

	for i := range result.Data {
		h.quantify(&result.Data[i])
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

	h.quantify(domainRisk)
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
		Title:              &input.Title,
		Description:        &input.Description,
		Impact:             &input.Impact,
		Probability:        &input.Probability,
		Tags:               input.Tags,
		Frameworks:         input.Frameworks,
		SLEXAF:             input.SLEXAF,
		ARO:                input.ARO,
		ReviewIntervalDays: input.ReviewIntervalDays,
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
		var linkedAssets []*domain.Asset
		query := database.DB
		if mwCtx != nil {
			query = query.Where("organization_id = ?", mwCtx.OrganizationID)
		}
		if err := query.Where("id IN ?", input.AssetIDs).Find(&linkedAssets).Error; err == nil {
			domainRisk.Assets = linkedAssets
			// No direct score compute here (RULE #12) — save the association,
			// then publish a Redis event below so the ScoreWorker recalculates
			// via the real Score Engine, same as CreateRisk.
			if err := database.DB.Model(&domainRisk).Association("Assets").Replace(linkedAssets); err != nil {
				log.Printf("Warning: failed to update asset associations for risk %s: %v", domainRisk.ID, err)
			}
		}
	}

	var out domain.Risk
	hasOut := database.DB.Preload("Mitigations").Preload("Mitigations.SubActions").Preload("Assets").First(&out, "id = ?", riskID).Error == nil

	// RULE #12: Score Engine is NEVER called directly from handler.
	// Always publish Redis event → ScoreWorker listens and recalculates async.
	// Uses the risk's currently linked assets — freshly replaced above if this
	// update touched asset_ids, or its pre-existing ones otherwise — so an
	// Impact/Probability-only edit still gets a criticality-adjusted score.
	assetsForScoring := domainRisk.Assets
	if hasOut {
		assetsForScoring = out.Assets
	}
	if h.redisClient != nil {
		userID := uuid.Nil
		if mwCtx != nil {
			userID = mwCtx.UserID
		}
		event := events.RiskUpdatedEvent{
			RiskID:           domainRisk.ID.String(),
			TenantID:         orgID.String(),
			Probability:      float64(domainRisk.Probability),
			Impact:           float64(domainRisk.Impact),
			AssetCriticality: averageAssetCriticalityFactor(assetsForScoring),
			TriggeredBy:      userID.String(),
		}
		_ = h.redisClient.Publish(c.Context(), events.RiskUpdated, event)
	}

	if !hasOut {
		h.quantify(domainRisk)
		return c.JSON(domainRisk)
	}
	h.quantify(&out)
	return c.JSON(out)
}

// averageAssetCriticalityFactor averages domain.AssetCriticality.ScoreFactor()
// across a risk's linked assets, for the Redis event consumed by ScoreWorker.
// Defaults to 1.0 (neutral) when a risk has no linked assets yet.
func averageAssetCriticalityFactor(assets []*domain.Asset) float64 {
	if len(assets) == 0 {
		return 1.0
	}
	var sum float64
	for _, a := range assets {
		sum += a.Criticality.ScoreFactor()
	}
	return sum / float64(len(assets))
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
