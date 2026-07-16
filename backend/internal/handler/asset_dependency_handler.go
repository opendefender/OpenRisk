// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/validation"
)

// AssetDependencyHandler exposes the asset dependency graph — the persistence
// behind the dependency cartography ("cartographie des dépendances entre actifs").
type AssetDependencyHandler struct {
	listUC   *assetuc.ListAssetDependenciesUseCase
	createUC *assetuc.CreateAssetDependencyUseCase
	deleteUC *assetuc.DeleteAssetDependencyUseCase
}

func NewAssetDependencyHandler(
	list *assetuc.ListAssetDependenciesUseCase,
	create *assetuc.CreateAssetDependencyUseCase,
	del *assetuc.DeleteAssetDependencyUseCase,
) *AssetDependencyHandler {
	return &AssetDependencyHandler{listUC: list, createUC: create, deleteUC: del}
}

// ListAssetDependencies returns the tenant's full dependency graph.
func (h *AssetDependencyHandler) ListAssetDependencies(c *fiber.Ctx) error {
	deps, err := h.listUC.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(deps)
}

type createAssetDependencyInput struct {
	SourceAssetID string `json:"source_asset_id" validate:"required,uuid"`
	TargetAssetID string `json:"target_asset_id" validate:"required,uuid"`
	Type          string `json:"type" validate:"omitempty,oneof=depends_on runs_on connects_to hosted_by stores_data_in authenticates_via backs_up_to managed_by"`
	Description   string `json:"description"`
}

// CreateAssetDependency adds a directed edge Source → Target.
func (h *AssetDependencyHandler) CreateAssetDependency(c *fiber.Ctx) error {
	input := new(createAssetDependencyInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	source, err := uuid.Parse(input.SourceAssetID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid source_asset_id"})
	}
	target, err := uuid.Parse(input.TargetAssetID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid target_asset_id"})
	}

	dep, err := h.createUC.Execute(c.UserContext(), tenantID(c), assetuc.CreateAssetDependencyInput{
		SourceAssetID: source,
		TargetAssetID: target,
		Type:          domain.DependencyType(input.Type),
		Description:   input.Description,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(dep)
}

// DeleteAssetDependency removes an edge from the graph.
func (h *AssetDependencyHandler) DeleteAssetDependency(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid dependency id"})
	}
	if err := h.deleteUC.Execute(c.UserContext(), tenantID(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}
