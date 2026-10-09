// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"

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
