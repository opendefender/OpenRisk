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

	appinc "github.com/opendefender/openrisk/internal/application/incident"
	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Incident operations: declaring one properly, explaining where they come from,
// and the structured review.
// ---------------------------------------------------------------------------

// WithPostMortems attaches the review service. Optional: without it the
// post-mortem endpoints answer 503 rather than pretending.
func (h *IncidentHandler) WithPostMortems(s *appinc.PostMortemService) *IncidentHandler {
	h.postMortems = s
	return h
}

// ListIncidentOrigins GET /incidents/origins
//
// Backs the "Where do incidents come from?" help page. Derived from the code
// paths that actually create incidents, plus this tenant's own counts — a help
// page that lists five sources while four of them have never fired is a page
// that teaches the wrong thing.
func (h *IncidentHandler) ListIncidentOrigins(c *fiber.Ctx) error {
	tenantID := safeGetString(c, "tenant_id")
	counts := h.incidentService.CountByOrigin(tenantID)

	origins := domain.IncidentOrigins()
	items := make([]fiber.Map, 0, len(origins))
	for _, o := range origins {
		items = append(items, fiber.Map{
			"origin": o,
			"count":  counts[o.Key],
		})
	}
	return c.JSON(fiber.Map{"items": items, "total": counts["_total"]})
}

// GetPostMortem GET /incidents/:id/post-mortem
func (h *IncidentHandler) GetPostMortem(c *fiber.Ctx) error {
	if h.postMortems == nil {
		return c.Status(503).JSON(fiber.Map{"error": "post-mortems are not available on this deployment"})
	}
	tenantID, incidentID, err := h.reviewTarget(c)
	if err != nil {
		return err
	}
	view, uerr := h.postMortems.Get(c.UserContext(), tenantID, incidentID)
	if uerr != nil {
		return writeAppError(c, uerr)
	}
	return c.JSON(view)
}

// postMortemBody is the editable review.
type postMortemBody struct {
	Summary             string `json:"summary"`
	RootCause           string `json:"root_cause"`
	ContributingFactors string `json:"contributing_factors"`
	Impact              string `json:"impact"`
	Detection           string `json:"detection"`
	WhatWentWell        string `json:"what_went_well"`
	LessonsLearned      string `json:"lessons_learned"`

	Timeline          []domain.PostMortemTimelineEntry `json:"timeline"`
	CorrectiveActions []domain.CorrectiveAction        `json:"corrective_actions"`
}

// SavePostMortem PUT /incidents/:id/post-mortem
func (h *IncidentHandler) SavePostMortem(c *fiber.Ctx) error {
	if h.postMortems == nil {
		return c.Status(503).JSON(fiber.Map{"error": "post-mortems are not available on this deployment"})
	}
	tenantID, incidentID, err := h.reviewTarget(c)
	if err != nil {
		return err
	}
	var body postMortemBody
	if perr := c.BodyParser(&body); perr != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": perr.Error()})
	}
	view, uerr := h.postMortems.Save(c.UserContext(), tenantID, incidentID, userID(c), appinc.PostMortemInput{
		Summary:             body.Summary,
		RootCause:           body.RootCause,
		ContributingFactors: body.ContributingFactors,
		Impact:              body.Impact,
		Detection:           body.Detection,
		WhatWentWell:        body.WhatWentWell,
		LessonsLearned:      body.LessonsLearned,
		Timeline:            body.Timeline,
		CorrectiveActions:   body.CorrectiveActions,
	})
	if uerr != nil {
		return writeAppError(c, uerr)
	}
	return c.JSON(view)
}

// PublishPostMortem POST /incidents/:id/post-mortem/publish
//
// Freezes the review and turns its corrective actions into tracked mitigation
// plans. The response reports what was converted AND what was not, with the
// reason — a publish that silently drops half the decisions would be worse than
// one that refuses.
func (h *IncidentHandler) PublishPostMortem(c *fiber.Ctx) error {
	if h.postMortems == nil {
		return c.Status(503).JSON(fiber.Map{"error": "post-mortems are not available on this deployment"})
	}
	tenantID, incidentID, err := h.reviewTarget(c)
	if err != nil {
		return err
	}
	res, uerr := h.postMortems.Publish(c.UserContext(), tenantID, incidentID, userID(c))
	if uerr != nil {
		return writeAppError(c, uerr)
	}
	return c.JSON(res)
}

// reviewTarget resolves the tenant and incident of a review request.
func (h *IncidentHandler) reviewTarget(c *fiber.Ctx) (uuid.UUID, uint, error) {
	raw := strings.TrimSpace(safeGetString(c, "tenant_id"))
	tenantID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, 0, c.Status(401).JSON(fiber.Map{"error": "no tenant in context"})
	}
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return uuid.Nil, 0, c.Status(400).JSON(fiber.Map{"error": "invalid incident id"})
	}
	return tenantID, uint(id), nil
}
