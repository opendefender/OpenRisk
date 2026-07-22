// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// VulnIntegrationHandler exposes the connector + ticketing configuration
// use cases. Credentials are write-only: they are never returned to the client.
type VulnIntegrationHandler struct {
	save         *vulnapp.SaveIntegrationUseCase
	list         *vulnapp.ListIntegrationsUseCase
	get          *vulnapp.GetIntegrationUseCase
	del          *vulnapp.DeleteIntegrationUseCase
	pull         *vulnapp.TriggerLivePullUseCase
	saveTicket   *vulnapp.SaveTicketingUseCase
	getTicket    *vulnapp.GetTicketingUseCase
	deleteTicket *vulnapp.DeleteTicketingUseCase
	createTicket *vulnapp.CreateTicketUseCase
}

func NewVulnIntegrationHandler(
	save *vulnapp.SaveIntegrationUseCase,
	list *vulnapp.ListIntegrationsUseCase,
	get *vulnapp.GetIntegrationUseCase,
	del *vulnapp.DeleteIntegrationUseCase,
	pull *vulnapp.TriggerLivePullUseCase,
	saveTicket *vulnapp.SaveTicketingUseCase,
	getTicket *vulnapp.GetTicketingUseCase,
	deleteTicket *vulnapp.DeleteTicketingUseCase,
	createTicket *vulnapp.CreateTicketUseCase,
) *VulnIntegrationHandler {
	return &VulnIntegrationHandler{
		save: save, list: list, get: get, del: del, pull: pull,
		saveTicket: saveTicket, getTicket: getTicket, deleteTicket: deleteTicket,
		createTicket: createTicket,
	}
}

func (h *VulnIntegrationHandler) tenant(c *fiber.Ctx) uuid.UUID {
	if mw := middleware.GetContext(c); mw != nil {
		return mw.OrganizationID
	}
	return uuid.Nil
}

// saveIntegrationBody is the POST/PUT /vulnerabilities/integrations body.
type saveIntegrationBody struct {
	Source                 string            `json:"source"`
	Name                   string            `json:"name"`
	Enabled                *bool             `json:"enabled"`
	BaseURL                string            `json:"base_url"`
	Credentials            map[string]string `json:"credentials"`
	ClearCredentials       bool              `json:"clear_credentials"`
	LivePullEnabled        bool              `json:"live_pull_enabled"`
	ScheduleMinutes        int               `json:"schedule_minutes"`
	WebhookEnabled         bool              `json:"webhook_enabled"`
	RegenerateWebhookToken bool              `json:"regenerate_webhook_token"`
	AutoCreateRisk         bool              `json:"auto_create_risk"`
	AutoCreateTicket       bool              `json:"auto_create_ticket"`
}

// SaveIntegration POST /vulnerabilities/integrations — create or update a
// connector config for a source (credentials encrypted at rest).
func (h *VulnIntegrationHandler) SaveIntegration(c *fiber.Ctx) error {
	var body saveIntegrationBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	res, err := h.save.Execute(c.UserContext(), h.tenant(c), vulnapp.SaveIntegrationInput{
		Source:                 domain.VulnSource(body.Source),
		Name:                   body.Name,
		Enabled:                enabled,
		BaseURL:                body.BaseURL,
		Credentials:            body.Credentials,
		ClearCredentials:       body.ClearCredentials,
		LivePullEnabled:        body.LivePullEnabled,
		ScheduleMinutes:        body.ScheduleMinutes,
		WebhookEnabled:         body.WebhookEnabled,
		RegenerateWebhookToken: body.RegenerateWebhookToken,
		AutoCreateRisk:         body.AutoCreateRisk,
		AutoCreateTicket:       body.AutoCreateTicket,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(res)
}

// ListIntegrations GET /vulnerabilities/integrations
func (h *VulnIntegrationHandler) ListIntegrations(c *fiber.Ctx) error {
	items, err := h.list.Execute(c.UserContext(), h.tenant(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not list integrations", "details": err.Error()})
	}
	return c.JSON(fiber.Map{"items": items})
}

// GetIntegration GET /vulnerabilities/integrations/:id
func (h *VulnIntegrationHandler) GetIntegration(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	res, err := h.get.Execute(c.UserContext(), h.tenant(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// DeleteIntegration DELETE /vulnerabilities/integrations/:id
func (h *VulnIntegrationHandler) DeleteIntegration(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	if err := h.del.Execute(c.UserContext(), h.tenant(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// TriggerPull POST /vulnerabilities/integrations/:id/pull — run a live API pull now.
func (h *VulnIntegrationHandler) TriggerPull(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	res, err := h.pull.Execute(c.UserContext(), h.tenant(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// ---- Ticketing config -----------------------------------------------------

type saveTicketingBody struct {
	Provider         string            `json:"provider"`
	Enabled          bool              `json:"enabled"`
	BaseURL          string            `json:"base_url"`
	ProjectOrTable   string            `json:"project_or_table"`
	DefaultIssueType string            `json:"default_issue_type"`
	Credentials      map[string]string `json:"credentials"`
	ClearCredentials bool              `json:"clear_credentials"`
}

// GetTicketing GET /vulnerabilities/ticketing
func (h *VulnIntegrationHandler) GetTicketing(c *fiber.Ctx) error {
	res, err := h.getTicket.Execute(c.UserContext(), h.tenant(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// SaveTicketing PUT /vulnerabilities/ticketing
func (h *VulnIntegrationHandler) SaveTicketing(c *fiber.Ctx) error {
	var body saveTicketingBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input", "details": err.Error()})
	}
	res, err := h.saveTicket.Execute(c.UserContext(), h.tenant(c), vulnapp.SaveTicketingInput{
		Provider:         domain.VulnTicketProvider(body.Provider),
		Enabled:          body.Enabled,
		BaseURL:          body.BaseURL,
		ProjectOrTable:   body.ProjectOrTable,
		DefaultIssueType: body.DefaultIssueType,
		Credentials:      body.Credentials,
		ClearCredentials: body.ClearCredentials,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// DeleteTicketing DELETE /vulnerabilities/ticketing
func (h *VulnIntegrationHandler) DeleteTicketing(c *fiber.Ctx) error {
	if err := h.deleteTicket.Execute(c.UserContext(), h.tenant(c)); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// CreateTicket POST /vulnerabilities/:id/ticket — open an ITSM ticket for a vuln.
func (h *VulnIntegrationHandler) CreateTicket(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid uuid"})
	}
	v, err := h.createTicket.Execute(c.UserContext(), h.tenant(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(v)
}
