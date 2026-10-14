// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/threatintel"
	"github.com/opendefender/openrisk/internal/domain"
)

// ThreatHandler serves the threat list of the Threat Intel page (#911): the
// catalogue entries that touch the tenant's assets, with an explained
// relevance, and the tenant's "not relevant" calls.
type ThreatHandler struct {
	list    *threatintel.ListThreatsUseCase
	dismiss *threatintel.DismissThreatUseCase
}

// NewThreatHandler builds the handler.
func NewThreatHandler(l *threatintel.ListThreatsUseCase, d *threatintel.DismissThreatUseCase) *ThreatHandler {
	return &ThreatHandler{list: l, dismiss: d}
}

// List GET /cti/threats
func (h *ThreatHandler) List(c *fiber.Ctx) error {
	tid := tenantID(c)
	if tid == uuid.Nil {
		return writeAppError(c, domain.ErrUnauthorized)
	}
	res, err := h.list.Execute(c.UserContext(), tid)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// Dismiss PUT /cti/threats/:cve/dismissal
func (h *ThreatHandler) Dismiss(c *fiber.Ctx) error {
	tid := tenantID(c)
	if tid == uuid.Nil {
		return writeAppError(c, domain.ErrUnauthorized)
	}
	if err := h.dismiss.Dismiss(c.UserContext(), tid, userID(c), utils.CopyString(c.Params("cve"))); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Restore DELETE /cti/threats/:cve/dismissal
func (h *ThreatHandler) Restore(c *fiber.Ctx) error {
	tid := tenantID(c)
	if tid == uuid.Nil {
		return writeAppError(c, domain.ErrUnauthorized)
	}
	if err := h.dismiss.Restore(c.UserContext(), tid, utils.CopyString(c.Params("cve"))); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
