// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package handler

import (
	"context"

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

	// dryRuns keeps in-flight traces so a test can be cancelled and re-read.
	dryRuns *appauto.DryRunRegistry
	// channelTester performs real single-channel deliveries. Optional.
	channelTester *appauto.ChannelTester
	// userEmails resolves actor emails for the run history. Optional.
	userEmails UserEmailLookup
}

// UserEmailLookup resolves user emails for display in the run history.
type UserEmailLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// NewAutomationHandler builds the handler.
func NewAutomationHandler(
	rules *appauto.RuleService,
	executions *appauto.ExecutionService,
	sla *appauto.SLAService,
	channels *appauto.ChannelService,
	engine *appauto.Engine,
) *AutomationHandler {
	return &AutomationHandler{
		rules: rules, executions: executions, sla: sla, channels: channels, engine: engine,
		dryRuns: appauto.NewDryRunRegistry(),
	}
}

// WithChannelTester attaches the real single-channel delivery tester.
func (h *AutomationHandler) WithChannelTester(t *appauto.ChannelTester) *AutomationHandler {
	h.channelTester = t
	return h
}

// WithUserEmails attaches actor-email resolution for the run history.
func (h *AutomationHandler) WithUserEmails(l UserEmailLookup) *AutomationHandler {
	h.userEmails = l
	return h
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
		return serverError(c, "could not list rules", err)
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

// runNowBody is the optional trigger context for a REAL manual run.
type runNowBody struct {
	Severity     string  `json:"severity"`
	CVEID        string  `json:"cve_id"`
	CVSS         float64 `json:"cvss"`
	KEV          bool    `json:"kev"`
	PriorityTier string  `json:"priority_tier"`
	AssetName    string  `json:"asset_name"`
	// Confirm must be true. Running a rule opens risks, files tickets and pages
	// people; the previous version of this endpoint did all of that behind a
	// button labelled "Test", which is how production got surprised. To test
	// without side effects, use POST /automation/rules/:id/dry-run.
	Confirm bool `json:"confirm"`
}

// RunRuleNow POST /automation/rules/:id/run — execute a rule for real, now.
func (h *AutomationHandler) RunRuleNow(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	var body runNowBody
	_ = c.BodyParser(&body)
	if !body.Confirm {
		return c.Status(400).JSON(fiber.Map{
			"error":  "confirmation required",
			"detail": "running a rule has real side effects (risks, tickets, alerts). Send confirm:true, or use /dry-run to trace it without changing anything.",
		})
	}
	if body.Severity == "" {
		body.Severity = "critical"
	}
	tc := appauto.TriggerContext{
		Ref:          "manual:" + firstNonEmpty(body.CVEID, id.String()),
		Subject:      "Manual run of an automation rule",
		Title:        firstNonEmpty(body.CVEID, "Manual run"),
		Severity:     body.Severity,
		CVSS:         body.CVSS,
		KEV:          body.KEV,
		PriorityTier: body.PriorityTier,
		CVEID:        body.CVEID,
		AssetName:    body.AssetName,
		TriggeredBy:  h.user(c),
	}
	exec, err := h.engine.RunRuleByID(c.UserContext(), id, h.tenant(c), tc, domain.ExecutionModeManual)
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
		return serverError(c, "could not list executions", err)
	}
	return c.JSON(fiber.Map{"items": h.decorate(c, items)})
}

// ListRuleExecutions GET /automation/rules/:id/executions
func (h *AutomationHandler) ListRuleExecutions(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	items, err := h.executions.ListByRule(c.UserContext(), h.tenant(c), id, c.QueryInt("limit", 50))
	if err != nil {
		return serverError(c, "could not list executions", err)
	}
	return c.JSON(fiber.Map{"items": h.decorate(c, items)})
}

// ListSLA GET /automation/sla — live SLA countdowns.
func (h *AutomationHandler) ListSLA(c *fiber.Ctx) error {
	items, err := h.sla.ListOpen(c.UserContext(), h.tenant(c))
	if err != nil {
		return serverError(c, "could not list SLA trackers", err)
	}
	return c.JSON(fiber.Map{"items": items})
}

// SLAStats GET /automation/sla/stats
func (h *AutomationHandler) SLAStats(c *fiber.Ctx) error {
	stats, err := h.sla.Stats(c.UserContext(), h.tenant(c))
	if err != nil {
		return serverError(c, "could not compute SLA stats", err)
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

	WebhookEnabled bool   `json:"webhook_enabled"`
	WebhookURL     string `json:"webhook_url"`
	WebhookSecret  string `json:"webhook_secret"`

	SMSEnabled     bool   `json:"sms_enabled"`
	SMSGatewayURL  string `json:"sms_gateway_url"`
	SMSAPIKey      string `json:"sms_api_key"`
	SMSSender      string `json:"sms_sender"`
	SMSRecipients  string `json:"sms_recipients"`
	SMSToField     string `json:"sms_to_field"`
	SMSTextField   string `json:"sms_text_field"`
	SMSSenderField string `json:"sms_sender_field"`
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
		WebhookEnabled:  body.WebhookEnabled,
		WebhookURL:      body.WebhookURL,
		WebhookSecret:   body.WebhookSecret,
		SMSEnabled:      body.SMSEnabled,
		SMSGatewayURL:   body.SMSGatewayURL,
		SMSAPIKey:       body.SMSAPIKey,
		SMSSender:       body.SMSSender,
		SMSRecipients:   body.SMSRecipients,
		SMSToField:      body.SMSToField,
		SMSTextField:    body.SMSTextField,
		SMSSenderField:  body.SMSSenderField,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(cfg)
}

// decorate adds the per-run step tally and resolves actor emails, so the history
// answers "who ran this, and what came of it" without a second request.
func (h *AutomationHandler) decorate(c *fiber.Ctx, items []domain.AutomationExecution) []appauto.ExecutionHistory {
	out := make([]appauto.ExecutionHistory, 0, len(items))
	for _, e := range items {
		out = append(out, appauto.Summarise(e))
	}
	if h.userEmails == nil {
		return out
	}
	idset := map[uuid.UUID]struct{}{}
	for _, e := range out {
		if e.ActorID != nil && *e.ActorID != uuid.Nil {
			idset[*e.ActorID] = struct{}{}
		}
	}
	if len(idset) == 0 {
		return out
	}
	ids := make([]uuid.UUID, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	emails, err := h.userEmails.EmailsByIDs(c.UserContext(), ids)
	if err != nil {
		return out // degrade to bare ids rather than failing the list
	}
	for i := range out {
		if out[i].ActorID != nil {
			out[i].ActorEmail = emails[*out[i].ActorID]
		}
	}
	return out
}

// channelsBody gains the generic webhook and SMS gateway fields; every secret is
// write-only (an empty value preserves what is stored).

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
