// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/complianceaudit"
	"github.com/opendefender/openrisk/internal/domain"
)

// ComplianceAuditHandler exposes the compliance audit + remediation-plan use
// cases. Tenant/user come from middleware.GetContext via the shared tenantID()/
// userID() helpers (defined in compliance_handler.go).
type ComplianceAuditHandler struct {
	createAudit *complianceaudit.CreateAuditUseCase
	listAudits  *complianceaudit.ListAuditsUseCase
	getAudit    *complianceaudit.GetAuditUseCase
	updateAudit *complianceaudit.UpdateAuditUseCase
	deleteAudit *complianceaudit.DeleteAuditUseCase

	createRemediation *complianceaudit.CreateRemediationUseCase
	listRemediations  *complianceaudit.ListRemediationsUseCase
	updateRemediation *complianceaudit.UpdateRemediationUseCase
	deleteRemediation *complianceaudit.DeleteRemediationUseCase

	generateRemediations *complianceaudit.GenerateRemediationsFromAuditUseCase
}

func NewComplianceAuditHandler(
	createAudit *complianceaudit.CreateAuditUseCase,
	listAudits *complianceaudit.ListAuditsUseCase,
	getAudit *complianceaudit.GetAuditUseCase,
	updateAudit *complianceaudit.UpdateAuditUseCase,
	deleteAudit *complianceaudit.DeleteAuditUseCase,
	createRemediation *complianceaudit.CreateRemediationUseCase,
	listRemediations *complianceaudit.ListRemediationsUseCase,
	updateRemediation *complianceaudit.UpdateRemediationUseCase,
	deleteRemediation *complianceaudit.DeleteRemediationUseCase,
	generateRemediations *complianceaudit.GenerateRemediationsFromAuditUseCase,
) *ComplianceAuditHandler {
	return &ComplianceAuditHandler{
		createAudit: createAudit, listAudits: listAudits, getAudit: getAudit,
		updateAudit: updateAudit, deleteAudit: deleteAudit,
		createRemediation: createRemediation, listRemediations: listRemediations,
		updateRemediation: updateRemediation, deleteRemediation: deleteRemediation,
		generateRemediations: generateRemediations,
	}
}

// parseOptionalDate accepts RFC3339 or a plain YYYY-MM-DD date; "" → nil.
func parseOptionalDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t, nil
	}
	return nil, domain.NewValidationError("invalid date: " + s)
}

// parseOptionalUUID parses a non-empty uuid string; "" → nil.
func parseOptionalUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, domain.NewValidationError("invalid id: " + s)
	}
	return &id, nil
}

// =============================================================================
// Audits
// =============================================================================

type createAuditBody struct {
	Title          string `json:"title"`
	FrameworkID    string `json:"framework_id"`
	Type           string `json:"type"`
	Auditor        string `json:"auditor"`
	Scope          string `json:"scope"`
	ScheduledStart string `json:"scheduled_start"`
	ScheduledEnd   string `json:"scheduled_end"`
}

func (h *ComplianceAuditHandler) ListAudits(c *fiber.Ctx) error {
	audits, err := h.listAudits.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(audits)
}

func (h *ComplianceAuditHandler) CreateAudit(c *fiber.Ctx) error {
	var body createAuditBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	fwID, err := parseOptionalUUID(body.FrameworkID)
	if err != nil {
		return writeAppError(c, err)
	}
	start, err := parseOptionalDate(body.ScheduledStart)
	if err != nil {
		return writeAppError(c, err)
	}
	end, err := parseOptionalDate(body.ScheduledEnd)
	if err != nil {
		return writeAppError(c, err)
	}
	audit, err := h.createAudit.Execute(c.UserContext(), tenantID(c), userID(c), complianceaudit.CreateAuditInput{
		Title:          body.Title,
		FrameworkID:    fwID,
		Type:           body.Type,
		Auditor:        body.Auditor,
		Scope:          body.Scope,
		ScheduledStart: start,
		ScheduledEnd:   end,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(audit)
}

func (h *ComplianceAuditHandler) GetAudit(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid audit id"})
	}
	audit, err := h.getAudit.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(audit)
}

type updateAuditBody struct {
	Title           *string  `json:"title"`
	FrameworkID     *string  `json:"framework_id"` // "" clears (program-wide)
	Type            *string  `json:"type"`
	Status          *string  `json:"status"`
	Auditor         *string  `json:"auditor"`
	Scope           *string  `json:"scope"`
	Summary         *string  `json:"summary"`
	ComplianceScore *float64 `json:"compliance_score"`
	ScheduledStart  *string  `json:"scheduled_start"`
	ScheduledEnd    *string  `json:"scheduled_end"`
}

func (h *ComplianceAuditHandler) UpdateAudit(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid audit id"})
	}
	var body updateAuditBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}

	in := complianceaudit.UpdateAuditInput{
		Title:           body.Title,
		Type:            body.Type,
		Status:          body.Status,
		Auditor:         body.Auditor,
		Scope:           body.Scope,
		Summary:         body.Summary,
		ComplianceScore: body.ComplianceScore,
	}
	if body.FrameworkID != nil {
		if *body.FrameworkID == "" {
			in.ClearFramework = true
		} else {
			fwID, err := parseOptionalUUID(*body.FrameworkID)
			if err != nil {
				return writeAppError(c, err)
			}
			in.FrameworkID = fwID
		}
	}
	if body.ScheduledStart != nil {
		start, err := parseOptionalDate(*body.ScheduledStart)
		if err != nil {
			return writeAppError(c, err)
		}
		in.ScheduledStart = start
	}
	if body.ScheduledEnd != nil {
		end, err := parseOptionalDate(*body.ScheduledEnd)
		if err != nil {
			return writeAppError(c, err)
		}
		in.ScheduledEnd = end
	}

	audit, err := h.updateAudit.Execute(c.UserContext(), tenantID(c), id, in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(audit)
}

func (h *ComplianceAuditHandler) DeleteAudit(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid audit id"})
	}
	if err := h.deleteAudit.Execute(c.UserContext(), tenantID(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// GenerateRemediations POST /compliance/audits/:id/generate-remediations —
// opens a remediation plan for every open gap under the audit's framework, in one
// click. Idempotent: gaps that already have an active plan are skipped.
func (h *ComplianceAuditHandler) GenerateRemediations(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid audit id"})
	}
	res, err := h.generateRemediations.Execute(c.UserContext(), tenantID(c), id, userID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(res)
}

// =============================================================================
// Remediation plans
// =============================================================================

type createRemediationBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ControlID   string `json:"control_id"`
	AuditID     string `json:"audit_id"`
	Priority    string `json:"priority"`
	AssignedTo  string `json:"assigned_to"`
	DueDate     string `json:"due_date"`
}

func (h *ComplianceAuditHandler) ListRemediations(c *fiber.Ctx) error {
	filter := domain.RemediationFilter{}
	if v := c.Query("control_id"); v != "" {
		id, err := parseOptionalUUID(v)
		if err != nil {
			return writeAppError(c, err)
		}
		filter.ControlID = id
	}
	if v := c.Query("framework_id"); v != "" {
		id, err := parseOptionalUUID(v)
		if err != nil {
			return writeAppError(c, err)
		}
		filter.FrameworkID = id
	}
	if v := c.Query("audit_id"); v != "" {
		id, err := parseOptionalUUID(v)
		if err != nil {
			return writeAppError(c, err)
		}
		filter.AuditID = id
	}
	if v := c.Query("status"); v != "" {
		st, err := domain.ParseRemediationStatus(v)
		if err != nil {
			return writeAppError(c, err)
		}
		filter.Status = st
	}
	plans, err := h.listRemediations.Execute(c.UserContext(), tenantID(c), filter)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(plans)
}

func (h *ComplianceAuditHandler) CreateRemediation(c *fiber.Ctx) error {
	var body createRemediationBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	ctrlID, err := parseOptionalUUID(body.ControlID)
	if err != nil {
		return writeAppError(c, err)
	}
	auditID, err := parseOptionalUUID(body.AuditID)
	if err != nil {
		return writeAppError(c, err)
	}
	assignee, err := parseOptionalUUID(body.AssignedTo)
	if err != nil {
		return writeAppError(c, err)
	}
	due, err := parseOptionalDate(body.DueDate)
	if err != nil {
		return writeAppError(c, err)
	}
	plan, err := h.createRemediation.Execute(c.UserContext(), tenantID(c), userID(c), complianceaudit.CreateRemediationInput{
		Title:       body.Title,
		Description: body.Description,
		ControlID:   ctrlID,
		AuditID:     auditID,
		Priority:    body.Priority,
		AssignedTo:  assignee,
		DueDate:     due,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(plan)
}

type updateRemediationBody struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *string `json:"priority"`
	Status      *string `json:"status"`
	AssignedTo  *string `json:"assigned_to"` // "" clears
	DueDate     *string `json:"due_date"`    // "" clears
}

func (h *ComplianceAuditHandler) UpdateRemediation(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid remediation id"})
	}
	var body updateRemediationBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	in := complianceaudit.UpdateRemediationInput{
		Title:       body.Title,
		Description: body.Description,
		Priority:    body.Priority,
		Status:      body.Status,
	}
	if body.AssignedTo != nil {
		if *body.AssignedTo == "" {
			in.ClearAssignee = true
		} else {
			a, err := parseOptionalUUID(*body.AssignedTo)
			if err != nil {
				return writeAppError(c, err)
			}
			in.AssignedTo = a
		}
	}
	if body.DueDate != nil {
		if *body.DueDate == "" {
			in.ClearDueDate = true
		} else {
			d, err := parseOptionalDate(*body.DueDate)
			if err != nil {
				return writeAppError(c, err)
			}
			in.DueDate = d
		}
	}
	plan, err := h.updateRemediation.Execute(c.UserContext(), tenantID(c), id, in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(plan)
}

func (h *ComplianceAuditHandler) DeleteRemediation(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid remediation id"})
	}
	if err := h.deleteRemediation.Execute(c.UserContext(), tenantID(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}
