// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/entity"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/middleware"
)

// ScoreWorkingHandler serves GET /risks/:id/score-working (#486).
type ScoreWorkingHandler struct {
	uc *risk.GetScoreWorkingUseCase
}

func NewScoreWorkingHandler(uc *risk.GetScoreWorkingUseCase) *ScoreWorkingHandler {
	return &ScoreWorkingHandler{uc: uc}
}

// Get returns the score with its terms and the audit entry behind each one.
// The route is guarded by risks:read; the provenance additionally needs the
// audit-read permission, the same gate as the entity drawer's audit tab.
func (h *ScoreWorkingHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk id"})
	}
	canReadAudit := middleware.NewPermissionChecker(c).HasPermission(entity.AuditPermission)
	w, err := h.uc.Execute(c.UserContext(), tenantID(c), id, canReadAudit)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(w)
}
