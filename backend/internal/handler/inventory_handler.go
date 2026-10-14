// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	invapp "github.com/opendefender/openrisk/internal/application/inventory"
	"github.com/opendefender/openrisk/internal/domain"
)

// InventoryHandler serves the user side of the software inventory (#910):
// "Lancer un scan" and an asset's inventory status. The agent side lives on
// ScannerHandler.AgentInventory.
type InventoryHandler struct {
	trigger *invapp.TriggerScanUseCase
	list    *invapp.ListAssetInventoriesUseCase
}

// NewInventoryHandler builds the handler.
func NewInventoryHandler(t *invapp.TriggerScanUseCase, l *invapp.ListAssetInventoriesUseCase) *InventoryHandler {
	return &InventoryHandler{trigger: t, list: l}
}

// Scan POST /vulnerabilities/scan: ask the tenant's online agents for their
// host's packages; each inventory is matched as it arrives.
func (h *InventoryHandler) Scan(c *fiber.Ctx) error {
	res, err := h.trigger.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		if errors.Is(err, domain.ErrConflict) && domain.HTTPStatusFromError(err) == http.StatusTooManyRequests {
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(invapp.ScanCooldown.Seconds())))
		}
		return writeAppError(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(res)
}

// ForAsset GET /assets/:id/inventories
func (h *InventoryHandler) ForAsset(c *fiber.Ctx) error {
	assetID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return writeAppError(c, domain.NewValidationError("invalid asset id"))
	}
	out, err := h.list.Execute(c.UserContext(), tenantID(c), assetID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"inventories": out})
}
