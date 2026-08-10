// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// The score endpoints — the only way a score reaches a screen.
//
// The band ALWAYS travels with the value it describes, in the same response,
// computed from it by internal/domain/scoring.BandFor. No client is given the
// ingredients to derive its own label, which is what made "the number says 63 and
// the badge says low" possible in the first place.
package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/score"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/scoring"
)

// ScoreHandler serves the canonical score.
type ScoreHandler struct {
	uc *score.UseCase
}

// NewScoreHandler wires the handler.
func NewScoreHandler(uc *score.UseCase) *ScoreHandler {
	return &ScoreHandler{uc: uc}
}

// GetScore GET /score?scope=tenant|risk|asset&id=…
//
// One endpoint for every surface: the dashboard hero, the sidebar footer, the
// dedicated page, the register row and the asset drawer all call this. They
// therefore cannot disagree — there is no second source to disagree with.
func (h *ScoreHandler) GetScore(c *fiber.Ctx) error {
	tenantID := tenantID(c)
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	rawScope := c.Query("scope", string(scoring.ScopeTenant))
	scope, ok := scoring.ParseScope(rawScope)
	if !ok {
		return writeAppError(c, domain.NewValidationError(
			"scope must be one of tenant, risk, asset (got: "+rawScope+")"))
	}

	var id uuid.UUID
	if raw := c.Query("id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return writeAppError(c, domain.NewValidationError("id must be a UUID"))
		}
		id = parsed
	}

	result, err := h.uc.Execute(c.UserContext(), tenantID, scope, id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(result)
}

// PreviewScore POST /score/preview
//
// The live figure a form shows while a slider moves. It runs the SAME model as
// the persisted score — a preview computed by a different formula would be a lie
// told at exactly the moment the user is deciding.
//
// Persists nothing; safe to call on every debounced keystroke.
func (h *ScoreHandler) PreviewScore(c *fiber.Ctx) error {
	if tenantID(c) == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var in score.PreviewInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	result, err := h.uc.Preview(in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(result)
}

// GetScoreModel GET /score/model
//
// The model's own description: scale, band boundaries, factor weights per scope
// and the formula version. It exists so the explainer can state the assumptions
// without hard-coding them — the client must not carry a second copy of the
// thresholds, which is exactly how the four incompatible band mappings appeared.
func (h *ScoreHandler) GetScoreModel(c *fiber.Ctx) error {
	if tenantID(c) == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	return c.JSON(scoring.Describe())
}
