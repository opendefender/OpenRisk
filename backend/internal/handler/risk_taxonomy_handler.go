// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/risk"
)

// RiskTaxonomyHandler serves the two halves of the classification model that are
// NOT free text: the tenant's controlled category vocabulary, and a risk's
// references to real compliance controls.
//
// Tags need no handler — they are a plain array on the risk and always were.
// That asymmetry is the point: only these two have rules.
type RiskTaxonomyHandler struct {
	categories *risk.CategoryUseCase
	mappings   *risk.ControlMappingUseCase
}

func NewRiskTaxonomyHandler(categories *risk.CategoryUseCase, mappings *risk.ControlMappingUseCase) *RiskTaxonomyHandler {
	return &RiskTaxonomyHandler{categories: categories, mappings: mappings}
}

// =============================================================================
// Categories
// =============================================================================

// ListCategories GET /risk-categories — the tenant's controlled vocabulary.
func (h *RiskTaxonomyHandler) ListCategories(c *fiber.Ctx) error {
	rows, err := h.categories.List(
		c.UserContext(), tenantID(c),
		c.Query("include_inactive") == "true",
		c.Query("with_counts") == "true",
	)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"items": rows, "total": len(rows)})
}

type categoryInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	SortOrder   int    `json:"sort_order"`
	Active      *bool  `json:"active"`
}

func (in categoryInput) toUseCase() risk.CategoryInput {
	return risk.CategoryInput{
		Name: in.Name, Description: in.Description,
		Color: in.Color, SortOrder: in.SortOrder, Active: in.Active,
	}
}

// CreateCategory POST /risk-categories — admin-gated by the route.
func (h *RiskTaxonomyHandler) CreateCategory(c *fiber.Ctx) error {
	input := new(categoryInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	created, err := h.categories.Create(c.UserContext(), tenantID(c), input.toUseCase())
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(created)
}

// UpdateCategory PATCH /risk-categories/:id
func (h *RiskTaxonomyHandler) UpdateCategory(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid category id"})
	}
	input := new(categoryInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	updated, err := h.categories.Update(c.UserContext(), tenantID(c), id, input.toUseCase())
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(updated)
}

// DeleteCategory DELETE /risk-categories/:id — detaches the risks that carried
// it rather than leaving them pointing at nothing.
func (h *RiskTaxonomyHandler) DeleteCategory(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid category id"})
	}
	if err := h.categories.Delete(c.UserContext(), tenantID(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// =============================================================================
// Control mappings
// =============================================================================

// ListRiskMappings GET /risks/:id/control-mappings
func (h *RiskTaxonomyHandler) ListRiskMappings(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid risk id"})
	}
	rows, err := h.mappings.ListForRisk(c.UserContext(), tenantID(c), riskID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"items": rows, "total": len(rows)})
}

type createRiskMappingInput struct {
	FrameworkID string  `json:"framework_id"`
	ControlID   *string `json:"control_id"`
	Note        string  `json:"note"`
}

// CreateRiskMapping POST /risks/:id/control-mappings
func (h *RiskTaxonomyHandler) CreateRiskMapping(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid risk id"})
	}
	input := new(createRiskMappingInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}

	in := risk.CreateMappingInput{RiskID: riskID, Note: input.Note, CreatedBy: userID(c)}
	if input.FrameworkID != "" {
		fw, err := uuid.Parse(input.FrameworkID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
		}
		in.FrameworkID = fw
	}
	if input.ControlID != nil && *input.ControlID != "" {
		ctrl, err := uuid.Parse(*input.ControlID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
		}
		in.ControlID = &ctrl
	}

	created, err := h.mappings.Create(c.UserContext(), tenantID(c), in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(created)
}

// DeleteRiskMapping DELETE /risks/:id/control-mappings/:mappingId
func (h *RiskTaxonomyHandler) DeleteRiskMapping(c *fiber.Ctx) error {
	mappingID, err := uuid.Parse(c.Params("mappingId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid mapping id"})
	}
	if err := h.mappings.Delete(c.UserContext(), tenantID(c), mappingID); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// ListUnmappedRisks GET /risks/unmapped — the catch-up screen for risks nobody
// mapped to a control. Mapping stays optional at creation on purpose; this is
// where the backlog becomes visible instead of invisible.
func (h *RiskTaxonomyHandler) ListUnmappedRisks(c *fiber.Ctx) error {
	rows, err := h.mappings.ListUnmapped(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"items": rows, "total": len(rows)})
}
