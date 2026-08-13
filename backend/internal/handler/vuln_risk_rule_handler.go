// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	riskapp "github.com/opendefender/openrisk/internal/application/risk"
	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/middleware"
)

// VulnRiskRuleHandler serves the vulnerability→risk rule and the review queue
// of the DRAFT risks it proposes.
type VulnRiskRuleHandler struct {
	rules  *vulnapp.RiskRuleService
	drafts *riskapp.DraftReviewUseCase
}

func NewVulnRiskRuleHandler(rules *vulnapp.RiskRuleService, drafts *riskapp.DraftReviewUseCase) *VulnRiskRuleHandler {
	return &VulnRiskRuleHandler{rules: rules, drafts: drafts}
}

func (h *VulnRiskRuleHandler) tenant(c *fiber.Ctx) uuid.UUID {
	if mw := middleware.GetContext(c); mw != nil {
		return mw.OrganizationID
	}
	return uuid.Nil
}

// GetRule GET /attack-surface/risk-rule
func (h *VulnRiskRuleHandler) GetRule(c *fiber.Ctx) error {
	rule, err := h.rules.Get(c.UserContext(), h.tenant(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(rule)
}

// UpdateRule PUT /attack-surface/risk-rule (admin)
func (h *VulnRiskRuleHandler) UpdateRule(c *fiber.Ctx) error {
	var in vulnapp.UpdateRuleInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	rule, err := h.rules.Update(c.UserContext(), h.tenant(c), userID(c), in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(rule)
}

// PreviewRule POST /attack-surface/risk-rule/preview
//
// Runs the rule over the current register WITHOUT creating anything. The body
// is optional: absent, it previews the saved rule; present, the proposed one.
func (h *VulnRiskRuleHandler) PreviewRule(c *fiber.Ctx) error {
	var in *vulnapp.UpdateRuleInput
	// An EMPTY object means "preview the saved rule", not "preview a rule with
	// every field false". Testing len(body) alone got this wrong: `{}` is two
	// bytes, so it parsed into a zero-value input and the preview reported the
	// rule as disabled seconds after it had been enabled.
	if fields := map[string]json.RawMessage{}; len(c.Body()) > 0 &&
		json.Unmarshal(c.Body(), &fields) == nil && len(fields) > 0 {
		parsed := new(vulnapp.UpdateRuleInput)
		if err := c.BodyParser(parsed); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
		}
		in = parsed
	}
	preview, err := h.rules.Preview(c.UserContext(), h.tenant(c), in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(preview)
}

// ListDrafts GET /attack-surface/draft-risks
func (h *VulnRiskRuleHandler) ListDrafts(c *fiber.Ctx) error {
	page, limit := 1, 50
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			page = n
		}
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	res, err := h.drafts.List(c.UserContext(), h.tenant(c), page, limit)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{
		"items": res.Data, "total": res.Total, "page": res.Page, "limit": res.Limit,
	})
}

type bulkReviewBody struct {
	RiskIDs  []string `json:"risk_ids"`
	Decision string   `json:"decision"` // accept | dismiss
}

// BulkReview POST /attack-surface/draft-risks/review
func (h *VulnRiskRuleHandler) BulkReview(c *fiber.Ctx) error {
	var body bulkReviewBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	ids := make([]uuid.UUID, 0, len(body.RiskIDs))
	for _, raw := range body.RiskIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid risk id: " + raw})
		}
		ids = append(ids, id)
	}

	res, err := h.drafts.BulkReview(c.UserContext(), h.tenant(c), ids, riskapp.BulkDecision(body.Decision))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}
