// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/timeframe"
	"github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/pkg/events"
	"github.com/opendefender/openrisk/pkg/validation"
)

// AssetHandler encapsulates the asset use cases (ROADMAP.md M3).
type AssetHandler struct {
	createAssetUC        *assetuc.CreateAssetUseCase
	getAssetUC           *assetuc.GetAssetUseCase
	listAssetsUC         *assetuc.ListAssetsUseCase
	updateAssetUC        *assetuc.UpdateAssetUseCase
	deleteAssetUC        *assetuc.DeleteAssetUseCase
	listAssetSnapshotsUC *assetuc.ListAssetSnapshotsUseCase
	assetStatisticsUC    *assetuc.AssetStatisticsUseCase
	redisClient          *redis.Client
}

func NewAssetHandler(
	createAsset *assetuc.CreateAssetUseCase,
	getAsset *assetuc.GetAssetUseCase,
	listAssets *assetuc.ListAssetsUseCase,
	updateAsset *assetuc.UpdateAssetUseCase,
	deleteAsset *assetuc.DeleteAssetUseCase,
	listAssetSnapshots *assetuc.ListAssetSnapshotsUseCase,
	assetStatistics *assetuc.AssetStatisticsUseCase,
	redisClient *redis.Client,
) *AssetHandler {
	return &AssetHandler{
		createAssetUC:        createAsset,
		getAssetUC:           getAsset,
		listAssetsUC:         listAssets,
		updateAssetUC:        updateAsset,
		deleteAssetUC:        deleteAsset,
		listAssetSnapshotsUC: listAssetSnapshots,
		assetStatisticsUC:    assetStatistics,
		redisClient:          redisClient,
	}
}

type createAssetInput struct {
	Name        string `json:"name" validate:"required"`
	Type        string `json:"type"`
	Criticality string `json:"criticality" validate:"omitempty,oneof=LOW MEDIUM HIGH CRITICAL"`
	Owner       string `json:"owner"`
	// Category is NOT validated with `oneof` here: the authoritative list lives
	// in domain.AssetCategories, and duplicating it in a struct tag is how the
	// two drift. The use case parses it and returns a named validation error.
	Category   string         `json:"category"`
	Attributes map[string]any `json:"attributes"`
}

// CreateAsset godoc
func (h *AssetHandler) CreateAsset(c *fiber.Ctx) error {
	input := new(createAssetInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	assetEntity, err := h.createAssetUC.Execute(c.UserContext(), tenantID(c), assetuc.CreateAssetInput{
		Name:        input.Name,
		Type:        input.Type,
		Criticality: domain.AssetCriticality(input.Criticality),
		Owner:       input.Owner,
		Category:    domain.AssetCategory(input.Category),
		Attributes:  input.Attributes,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(assetEntity)
}

// ListAssets godoc
// Supports typed-attribute search: ?category=server&attr.environment=production
func (h *AssetHandler) ListAssets(c *fiber.Ctx) error {
	filter := assetuc.ListFilter{Category: domain.AssetCategory(c.Query("category"))}
	// Any query parameter prefixed `attr.` is an attribute search term. Using a
	// prefix rather than a fixed parameter list is what keeps search working
	// when a tenant adds an attribute to a schema — no server change needed.
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if !strings.HasPrefix(key, "attr.") || len(v) == 0 {
			return
		}
		filter.Attributes = append(filter.Attributes, domain.AttributeSearchTerm{
			Key: strings.TrimPrefix(key, "attr."), Value: string(v),
		})
	})

	assets, err := h.listAssetsUC.Search(c.UserContext(), tenantID(c), filter)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(assets)
}

// assetStatisticsResponse is the wire shape of GET /assets/statistics.
//
// The period is echoed back, and `period_applies_to` names the fields the window
// actually narrowed. That list is not decoration: a dashboard that shows numbers
// without saying which period produced them cannot be reconciled against
// anything, and a period control that appears to filter every tile while filtering
// one is worse than no control at all.
type assetStatisticsResponse struct {
	Period timeframe.Resolved `json:"period"`
	// PeriodAppliesTo names the period-scoped fields. Everything else is a
	// point-in-time stock, counted in full.
	PeriodAppliesTo []string `json:"period_applies_to"`
	*domain.AssetStatistics
	GeneratedAt string `json:"generated_at"`
}

// GetAssetStatistics godoc
//
// GET /assets/statistics?period=…  — the inventory's shape, counted in SQL.
//
// This exists because the estate dashboard used to answer the same question by
// downloading the entire inventory (with its risk associations preloaded, since
// that is what GET /assets does for the topology graph) and reducing it in the
// browser. The counts were right; obtaining them cost the whole inventory on
// every dashboard paint, and grew with the tenant.
func (h *AssetHandler) GetAssetStatistics(c *fiber.Ctx) error {
	window, err := parsePeriod(c)
	if err != nil {
		return writePeriodError(c, err)
	}
	stats, err := h.assetStatisticsUC.Execute(c.UserContext(), tenantID(c), window)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(assetStatisticsResponse{
		Period:          window.Resolved(),
		PeriodAppliesTo: []string{"added_in_period"},
		AssetStatistics: stats,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
	})
}

// GetAsset godoc
func (h *AssetHandler) GetAsset(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid asset id"})
	}
	assetEntity, err := h.getAssetUC.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(assetEntity)
}

type updateAssetInput struct {
	Name        *string `json:"name" validate:"omitempty"`
	Type        *string `json:"type" validate:"omitempty"`
	Criticality *string `json:"criticality" validate:"omitempty,oneof=LOW MEDIUM HIGH CRITICAL"`
	Owner       *string `json:"owner" validate:"omitempty"`
	Category    *string `json:"category" validate:"omitempty"`
	// Attributes replaces the whole bag when present; absent leaves it alone.
	Attributes map[string]any `json:"attributes"`
}

// UpdateAsset godoc
func (h *AssetHandler) UpdateAsset(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid asset id"})
	}
	input := new(updateAssetInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	ucInput := assetuc.UpdateAssetInput{
		Name: input.Name, Type: input.Type, Owner: input.Owner,
		Attributes: input.Attributes,
	}
	if input.Criticality != nil {
		crit := domain.AssetCriticality(*input.Criticality)
		ucInput.Criticality = &crit
	}
	if input.Category != nil {
		cat := domain.AssetCategory(*input.Category)
		ucInput.Category = &cat
	}

	result, err := h.updateAssetUC.Execute(c.UserContext(), tenantID(c), id, userID(c), ucInput)
	if err != nil {
		return writeAppError(c, err)
	}

	// RULE #12 (same convention as risks): the Score Engine is never called
	// directly from a handler. Publishing this event lets ScoreWorker
	// recalculate every risk linked to this asset via the real Engine.
	if result.CriticalityChanged && h.redisClient != nil {
		event := events.AssetCriticalityChangedEvent{
			AssetID:        result.Asset.ID.String(),
			TenantID:       tenantID(c).String(),
			OldCriticality: string(result.OldCriticality),
			NewCriticality: string(result.NewCriticality),
			ChangedBy:      userID(c).String(),
			ChangedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		_ = h.redisClient.Publish(c.Context(), events.AssetCriticalityChanged, event)
	}

	return c.JSON(result.Asset)
}

// DeleteAsset godoc
func (h *AssetHandler) DeleteAsset(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid asset id"})
	}
	if err := h.deleteAssetUC.Execute(c.UserContext(), tenantID(c), id, userID(c)); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// GetAssetHistory godoc
func (h *AssetHandler) GetAssetHistory(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid asset id"})
	}
	history, err := h.listAssetSnapshotsUC.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(history)
}
