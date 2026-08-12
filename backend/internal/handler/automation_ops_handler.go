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
)

// ---------------------------------------------------------------------------
// Automation operations: dry run, lifecycle, replay, templates, channel tests.
//
// Split from automation_handler.go (CRUD) because these are the verbs that make
// the module trustworthy, and they deserve to be read together.
// ---------------------------------------------------------------------------

// dryRunBody is the optional dry-run request. Everything is optional: with an
// empty body the engine picks the tenant's most relevant live record.
type dryRunBody struct {
	SubjectType string `json:"subject_type"` // vulnerability | risk | incident
	SubjectID   string `json:"subject_id"`
	Overrides   struct {
		Severity     string   `json:"severity"`
		CVSS         float64  `json:"cvss"`
		KEV          *bool    `json:"kev"`
		PriorityTier string   `json:"priority_tier"`
		AssetTags    []string `json:"asset_tags"`
	} `json:"overrides"`
}

// DryRunRule POST /automation/rules/:id/dry-run
//
// Traces what the rule WOULD do against real tenant data, without touching a
// single action port. The response carries the step-by-step verdict, the payload
// as it stood at each step, and the exact failure point when there is one.
func (h *AutomationHandler) DryRunRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	var body dryRunBody
	_ = c.BodyParser(&body) // body is optional

	tenant := h.tenant(c)
	// Register the run first so it has an id the client can cancel while it is
	// still going.
	runID, runCtx := h.dryRuns.Start(c.UserContext(), tenant)

	report, err := h.engine.DryRun(runCtx, id, tenant, appauto.DryRunRequest{
		SubjectType: body.SubjectType,
		SubjectID:   body.SubjectID,
		Overrides: appauto.SubjectOverrides{
			Severity:     body.Overrides.Severity,
			CVSS:         body.Overrides.CVSS,
			KEV:          body.Overrides.KEV,
			PriorityTier: body.Overrides.PriorityTier,
			AssetTags:    body.Overrides.AssetTags,
		},
		ActorID: h.user(c),
	})
	if err != nil {
		h.dryRuns.Finish(runID, nil)
		return writeAppError(c, err)
	}
	h.dryRuns.Finish(runID, report)
	return c.JSON(report)
}

// GetDryRun GET /automation/dry-runs/:id — re-read a trace by id.
func (h *AutomationHandler) GetDryRun(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	rep, ok := h.dryRuns.Get(id, h.tenant(c))
	if !ok {
		return c.Status(404).JSON(fiber.Map{"error": "no such dry run"})
	}
	return c.JSON(rep)
}

// CancelDryRun POST /automation/dry-runs/:id/cancel — stop an in-flight test.
func (h *AutomationHandler) CancelDryRun(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	if !h.dryRuns.Cancel(id, h.tenant(c)) {
		// Nothing to cancel is not an error the user can act on, but they should
		// know why the button did nothing.
		return c.Status(409).JSON(fiber.Map{
			"cancelled": false,
			"detail":    "this test has already finished or is unknown to this tenant",
		})
	}
	return c.JSON(fiber.Map{"cancelled": true})
}

// suspendBody carries the mandatory suspension reason.
type suspendBody struct {
	Reason string `json:"reason"`
}

// EnableRule POST /automation/rules/:id/enable
func (h *AutomationHandler) EnableRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	rule, err := h.rules.Enable(c.UserContext(), h.tenant(c), id, h.user(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(rule)
}

// SuspendRule POST /automation/rules/:id/suspend
func (h *AutomationHandler) SuspendRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	var body suspendBody
	_ = c.BodyParser(&body)
	rule, err := h.rules.Suspend(c.UserContext(), h.tenant(c), id, h.user(c), body.Reason)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(rule)
}

// ReplayExecution POST /automation/executions/:id/replay — re-run a past
// execution against the input recorded with it.
func (h *AutomationHandler) ReplayExecution(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	exec, err := h.engine.Replay(c.UserContext(), id, h.tenant(c), h.user(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(exec)
}

// AutomationState GET /automation/state — the live indicator the UI polls.
func (h *AutomationHandler) AutomationState(c *fiber.Ctx) error {
	state, err := h.rules.State(c.UserContext(), h.tenant(c), c.Query("locale", "fr"))
	if err != nil {
		return serverError(c, "could not read automation state", err)
	}
	return c.JSON(state)
}

// ListTemplates GET /automation/templates — the ready-made playbooks, each
// rendered as the sentence it will become.
func (h *AutomationHandler) ListTemplates(c *fiber.Ctx) error {
	locale := c.Query("locale", "fr")
	templates := domain.AutomationTemplates()
	items := make([]fiber.Map, 0, len(templates))
	for _, t := range templates {
		preview := &domain.AutomationRule{
			Trigger:    t.Trigger,
			Conditions: t.Conditions,
			Actions:    t.Actions,
		}
		items = append(items, fiber.Map{
			"template": t,
			"sentence": preview.Describe(locale),
		})
	}
	return c.JSON(fiber.Map{"items": items})
}

// fromTemplateBody optionally renames the adopted rule.
type fromTemplateBody struct {
	Name string `json:"name"`
}

// CreateRuleFromTemplate POST /automation/templates/:key/adopt
func (h *AutomationHandler) CreateRuleFromTemplate(c *fiber.Ctx) error {
	var body fromTemplateBody
	_ = c.BodyParser(&body)
	rule, err := h.rules.CreateFromTemplate(c.UserContext(), h.tenant(c), h.user(c), c.Params("key"), body.Name)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(rule)
}

// channelTestBody names the channel to test.
type channelTestBody struct {
	Channel string `json:"channel"`
}

// TestChannel POST /automation/channels/test — deliver one real test message on
// one channel and report exactly what happened.
func (h *AutomationHandler) TestChannel(c *fiber.Ctx) error {
	if h.channelTester == nil {
		return c.Status(503).JSON(fiber.Map{"error": "channel testing is not available on this deployment"})
	}
	var body channelTestBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	res, err := h.channelTester.Test(c.UserContext(), h.tenant(c), h.user(c), body.Channel)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// ListChannelCatalogue GET /automation/channels/catalogue — the channels this
// deployment supports, and which of them this tenant has configured.
func (h *AutomationHandler) ListChannelCatalogue(c *fiber.Ctx) error {
	configured, err := h.channels.ConfiguredChannels(c.UserContext(), h.tenant(c))
	if err != nil {
		return serverError(c, "could not read channel configuration", err)
	}
	set := map[string]bool{}
	for _, ch := range configured {
		set[ch] = true
	}
	items := make([]fiber.Map, 0)
	for _, ch := range domain.AllAutomationChannels() {
		items = append(items, fiber.Map{"channel": ch, "configured": set[ch]})
	}
	return c.JSON(fiber.Map{"items": items})
}
