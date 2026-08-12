// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// VulnCorrelationHandler serves the vulnerability↔asset correlation surface:
// the findings nobody could attribute, the candidates for each, and the manual
// resolution that pins a human's decision.
type VulnCorrelationHandler struct {
	listUnassigned *vulnapp.ListUnassignedUseCase
	candidates     *vulnapp.GetCandidatesUseCase
	resolve        *vulnapp.ResolveAssetUseCase
}

func NewVulnCorrelationHandler(
	listUnassigned *vulnapp.ListUnassignedUseCase,
	candidates *vulnapp.GetCandidatesUseCase,
	resolve *vulnapp.ResolveAssetUseCase,
) *VulnCorrelationHandler {
	return &VulnCorrelationHandler{
		listUnassigned: listUnassigned, candidates: candidates, resolve: resolve,
	}
}

func (h *VulnCorrelationHandler) tenant(c *fiber.Ctx) uuid.UUID {
	if mw := middleware.GetContext(c); mw != nil {
		return mw.OrganizationID
	}
	return uuid.Nil
}

// ListUnassigned GET /vulnerabilities/unassigned
// Findings the correlator would not attribute: nothing matched, or several
// assets matched equally well.
func (h *VulnCorrelationHandler) ListUnassigned(c *fiber.Ctx) error {
	q := domain.NewVulnerabilityQuery()
	if v := c.Query("q"); v != "" {
		q.Search = v
	}
	if v := c.Query("severity"); v != "" {
		q.Severities = strings.Split(v, ",")
	}
	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			q.Page = p
		}
	}
	if v := c.Query("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			q.Limit = l
		}
	}

	res, err := h.listUnassigned.Execute(c.UserContext(), h.tenant(c), q)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{
		"items": res.Data, "total": res.Total, "page": res.Page, "limit": res.Limit,
	})
}

// GetCandidates GET /vulnerabilities/:id/match-candidates
func (h *VulnCorrelationHandler) GetCandidates(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid vulnerability id"})
	}
	out, err := h.candidates.Execute(c.UserContext(), h.tenant(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"candidates": out})
}

type resolveAssetBody struct {
	// AssetID may be null to explicitly detach the finding ("this belongs to
	// nothing we own") — also a decision worth recording.
	AssetID *string `json:"asset_id"`
}

// ResolveAsset PUT /vulnerabilities/:id/asset — pin a human's attribution.
func (h *VulnCorrelationHandler) ResolveAsset(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid vulnerability id"})
	}
	var body resolveAssetBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}

	var assetID *uuid.UUID
	if body.AssetID != nil && strings.TrimSpace(*body.AssetID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*body.AssetID))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid asset_id"})
		}
		assetID = &parsed
	}

	v, err := h.resolve.Execute(c.UserContext(), h.tenant(c), id, assetID, userID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}
