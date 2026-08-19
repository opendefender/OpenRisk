// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"

	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
)

// EntitlementHandler serves the resolved plan/features/limits/usage snapshot the
// frontend uses to grey and explain paid features. It is authoritative: the same
// service enforces on write.
type EntitlementHandler struct {
	svc *entapp.Service
}

func NewEntitlementHandler(svc *entapp.Service) *EntitlementHandler {
	return &EntitlementHandler{svc: svc}
}

// GetEntitlements handles GET /api/v1/entitlements.
func (h *EntitlementHandler) GetEntitlements(c *fiber.Ctx) error {
	snap, err := h.svc.Resolve(c.UserContext(), tenantID(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not resolve entitlements"})
	}
	return c.JSON(snap)
}
