// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
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
	redisClient          *redis.Client
}

func NewAssetHandler(
	createAsset *assetuc.CreateAssetUseCase,
	getAsset *assetuc.GetAssetUseCase,
	listAssets *assetuc.ListAssetsUseCase,
	updateAsset *assetuc.UpdateAssetUseCase,
	deleteAsset *assetuc.DeleteAssetUseCase,
	listAssetSnapshots *assetuc.ListAssetSnapshotsUseCase,
	redisClient *redis.Client,
) *AssetHandler {
	return &AssetHandler{
		createAssetUC:        createAsset,
		getAssetUC:           getAsset,
		listAssetsUC:         listAssets,
		updateAssetUC:        updateAsset,
		deleteAssetUC:        deleteAsset,
		listAssetSnapshotsUC: listAssetSnapshots,
		redisClient:          redisClient,
	}
}

type createAssetInput struct {
	Name        string `json:"name" validate:"required"`
	Type        string `json:"type"`
	Criticality string `json:"criticality" validate:"omitempty,oneof=LOW MEDIUM HIGH CRITICAL"`
	Owner       string `json:"owner"`
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
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(assetEntity)
}

// ListAssets godoc
func (h *AssetHandler) ListAssets(c *fiber.Ctx) error {
	assets, err := h.listAssetsUC.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(assets)
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

	ucInput := assetuc.UpdateAssetInput{Name: input.Name, Type: input.Type, Owner: input.Owner}
	if input.Criticality != nil {
		crit := domain.AssetCriticality(*input.Criticality)
		ucInput.Criticality = &crit
	}

	result, err := h.updateAssetUC.Execute(c.UserContext(), tenantID(c), id, ucInput)
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
	if err := h.deleteAssetUC.Execute(c.UserContext(), tenantID(c), id); err != nil {
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
