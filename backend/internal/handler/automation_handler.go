// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	appauto "github.com/opendefender/openrisk/internal/application/automation"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// AutomationHandler exposes the Security Automation / SOAR API (spec §10):
// rule CRUD + dry-run, execution audit trail, the SLA dashboard, and the
// per-tenant alert-channel configuration.
type AutomationHandler struct {
	rules      *appauto.RuleService
	executions *appauto.ExecutionService
	sla        *appauto.SLAService
	channels   *appauto.ChannelService
	engine     *appauto.Engine
}

// NewAutomationHandler builds the handler.
func NewAutomationHandler(
	rules *appauto.RuleService,
	executions *appauto.ExecutionService,
	sla *appauto.SLAService,
	channels *appauto.ChannelService,
	engine *appauto.Engine,
) *AutomationHandler {
	return &AutomationHandler{rules: rules, executions: executions, sla: sla, channels: channels, engine: engine}
}

func (h *AutomationHandler) tenant(c *fiber.Ctx) uuid.UUID {
	if mw := middleware.GetContext(c); mw != nil {
		return mw.OrganizationID
	}
	return uuid.Nil
}

func (h *AutomationHandler) user(c *fiber.Ctx) uuid.UUID {
	if mw := middleware.GetContext(c); mw != nil {
		return mw.UserID
	}
	return uuid.Nil
}

// ruleBody is the create/update payload. Conditions/Actions/SLA bind directly
// from JSON into their domain shapes.
type ruleBody struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Enabled     *bool                       `json:"enabled"`
	Trigger     string                      `json:"trigger"`
	Conditions  domain.AutomationConditions `json:"conditions"`
	Actions     domain.AutomationActionList `json:"actions"`
	SLA         domain.AutomationSLAConfig  `json:"sla"`
	Priority    int                         `json:"priority"`
}

func (b ruleBody) toInput() appauto.RuleInput {
	return appauto.RuleInput{
		Name:        b.Name,
		Description: b.Description,
		Enabled:     b.Enabled,
		Trigger:     b.Trigger,
		Conditions:  b.Conditions,
		Actions:     b.Actions,
		SLA:         b.SLA,
		Priority:    b.Priority,
	}
}

// ListRules GET /automation/rules
func (h *AutomationHandler) ListRules(c *fiber.Ctx) error {
	items, err := h.rules.List(c.UserContext(), h.tenant(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not list rules", "details": err.Error()})
	}
	return c.JSON(fiber.Map{"items": items})
}

// CreateRule POST /automation/rules
func (h *AutomationHandler) CreateRule(c *fiber.Ctx) error {
	var body ruleBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	rule, err := h.rules.Create(c.UserContext(), h.tenant(c), h.user(c), body.toInput())
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(rule)
}

// GetRule GET /automation/rules/:id
func (h *AutomationHandler) GetRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	rule, err := h.rules.Get(c.UserContext(), h.tenant(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(rule)
}

// UpdateRule PUT /automation/rules/:id
func (h *AutomationHandler) UpdateRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	var body ruleBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	rule, err := h.rules.Update(c.UserContext(), h.tenant(c), id, body.toInput())
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(rule)
}

// DeleteRule DELETE /automation/rules/:id
func (h *AutomationHandler) DeleteRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	if err := h.rules.Delete(c.UserContext(), h.tenant(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// testRuleBody is the optional dry-run trigger context.
type testRuleBody struct {
	Severity     string  `json:"severity"`
	CVEID        string  `json:"cve_id"`
	CVSS         float64 `json:"cvss"`
	KEV          bool    `json:"kev"`
	PriorityTier string  `json:"priority_tier"`
	AssetName    string  `json:"asset_name"`
}

// TestRule POST /automation/rules/:id/test — run a rule immediately against a
// supplied (or sample) trigger context, without waiting for a real event.
func (h *AutomationHandler) TestRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	var body testRuleBody
	_ = c.BodyParser(&body) // body is optional
	if body.Severity == "" {
		body.Severity = "critical"
	}
	if body.CVEID == "" {
		body.CVEID = "CVE-0000-TEST"
	}
	tc := appauto.TriggerContext{
		Ref:          "test:" + body.CVEID,
		Subject:      "Automation rule dry-run",
		Title:        firstNonEmpty(body.CVEID, "Automation rule dry-run"),
		Severity:     body.Severity,
		CVSS:         body.CVSS,
		KEV:          body.KEV,
		PriorityTier: body.PriorityTier,
		CVEID:        body.CVEID,
		AssetName:    body.AssetName,
		TriggeredBy:  h.user(c),
	}
	exec, err := h.engine.RunRuleByID(c.UserContext(), id, h.tenant(c), tc)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(exec)
}

// ListExecutions GET /automation/executions
func (h *AutomationHandler) ListExecutions(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	items, err := h.executions.List(c.UserContext(), h.tenant(c), limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not list executions", "details": err.Error()})
	}
	return c.JSON(fiber.Map{"items": items})
}

// ListRuleExecutions GET /automation/rules/:id/executions
func (h *AutomationHandler) ListRuleExecutions(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	items, err := h.executions.ListByRule(c.UserContext(), h.tenant(c), id, c.QueryInt("limit", 50))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not list executions", "details": err.Error()})
	}
	return c.JSON(fiber.Map{"items": items})
}

// ListSLA GET /automation/sla — live SLA countdowns.
func (h *AutomationHandler) ListSLA(c *fiber.Ctx) error {
	items, err := h.sla.ListOpen(c.UserContext(), h.tenant(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not list SLA trackers", "details": err.Error()})
	}
	return c.JSON(fiber.Map{"items": items})
}

// SLAStats GET /automation/sla/stats
func (h *AutomationHandler) SLAStats(c *fiber.Ctx) error {
	stats, err := h.sla.Stats(c.UserContext(), h.tenant(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not compute SLA stats", "details": err.Error()})
	}
	return c.JSON(stats)
}

// GetChannels GET /automation/channels
func (h *AutomationHandler) GetChannels(c *fiber.Ctx) error {
	cfg, err := h.channels.Get(c.UserContext(), h.tenant(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(cfg)
}

// channelsBody is the save payload. Webhook URLs are write-only.
type channelsBody struct {
	SlackEnabled    bool   `json:"slack_enabled"`
	SlackWebhookURL string `json:"slack_webhook_url"`
	TeamsEnabled    bool   `json:"teams_enabled"`
	TeamsWebhookURL string `json:"teams_webhook_url"`
	EmailEnabled    bool   `json:"email_enabled"`
	DefaultEmail    string `json:"default_email"`
}

// SaveChannels PUT /automation/channels
func (h *AutomationHandler) SaveChannels(c *fiber.Ctx) error {
	var body channelsBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	cfg, err := h.channels.Save(c.UserContext(), h.tenant(c), appauto.ChannelInput{
		SlackEnabled:    body.SlackEnabled,
		SlackWebhookURL: body.SlackWebhookURL,
		TeamsEnabled:    body.TeamsEnabled,
		TeamsWebhookURL: body.TeamsWebhookURL,
		EmailEnabled:    body.EmailEnabled,
		DefaultEmail:    body.DefaultEmail,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(cfg)
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
