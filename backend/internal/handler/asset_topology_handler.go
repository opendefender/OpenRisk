// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
)

// AssetTopologyHandler serves the attack-surface topology: the dependency graph
// as the view needs it, and the compromise chain of a single asset.
type AssetTopologyHandler struct {
	topologyUC *assetuc.GetTopologyUseCase
	chainUC    *assetuc.GetCompromiseChainUseCase
}

func NewAssetTopologyHandler(
	topology *assetuc.GetTopologyUseCase,
	chain *assetuc.GetCompromiseChainUseCase,
) *AssetTopologyHandler {
	return &AssetTopologyHandler{topologyUC: topology, chainUC: chain}
}

// GetTopology returns the tenant's asset graph.
// GET /attack-surface/topology?limit=2000
func (h *AssetTopologyHandler) GetTopology(c *fiber.Ctx) error {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	topo, err := h.topologyUC.Execute(c.UserContext(), tenantID(c), limit)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(topo)
}

// GetCompromiseChain returns everything reachable from, and impacted by, an
// asset.
// GET /attack-surface/topology/:id/compromise-chain
func (h *AssetTopologyHandler) GetCompromiseChain(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid asset id"})
	}
	chain, err := h.chainUC.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(chain)
}

// GetEdgeTypes returns the topology edge vocabulary so the legend is served by
// the same source that folds stored types onto it.
// GET /attack-surface/topology/edge-types
func (h *AssetTopologyHandler) GetEdgeTypes(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"edge_types": domain.TopologyEdgeTypes})
}
