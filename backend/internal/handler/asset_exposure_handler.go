// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/vulnerability"
)

// AssetExposureHandler serves GET /assets/exposure (#906).
type AssetExposureHandler struct {
	uc *vulnerability.AssetExposureUseCase
}

// NewAssetExposureHandler builds the handler.
func NewAssetExposureHandler(uc *vulnerability.AssetExposureUseCase) *AssetExposureHandler {
	return &AssetExposureHandler{uc: uc}
}

// List returns open vulnerabilities, open KEV and last detection per asset.
func (h *AssetExposureHandler) List(c *fiber.Ctx) error {
	out, err := h.uc.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"items": out})
}

// AssetAnalysisHandler serves GET /assets/:id/analysis (#937).
type AssetAnalysisHandler struct {
	uc *vulnerability.AssetAnalysisUseCase
}

// NewAssetAnalysisHandler builds the handler.
func NewAssetAnalysisHandler(uc *vulnerability.AssetAnalysisUseCase) *AssetAnalysisHandler {
	return &AssetAnalysisHandler{uc: uc}
}

// Get returns the asset's exposure analysis.
func (h *AssetAnalysisHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid UUID"})
	}
	out, err := h.uc.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(out)
}
