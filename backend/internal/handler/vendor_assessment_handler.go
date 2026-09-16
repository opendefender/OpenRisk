// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/tprm"
	"github.com/opendefender/openrisk/internal/domain"
)

// VendorAssessmentHandler exposes questionnaire templates and vendor assessments
// to tenant users (#670, ADR 0004 D3). Mounted behind vendors:read /
// vendors:manage and the vendor_risk entitlement in cmd/server/main.go.
type VendorAssessmentHandler struct {
	createTemplate  *tprm.CreateQuestionnaireTemplateUseCase
	listTemplates   *tprm.ListQuestionnaireTemplatesUseCase
	getTemplate     *tprm.GetQuestionnaireTemplateUseCase
	updateTemplate  *tprm.UpdateQuestionnaireTemplateUseCase
	archiveTemplate *tprm.ArchiveQuestionnaireTemplateUseCase
	send            *tprm.SendVendorAssessmentUseCase
	listAssessments *tprm.ListVendorAssessmentsUseCase
	getAssessment   *tprm.GetVendorAssessmentUseCase
	revoke          *tprm.RevokeVendorAssessmentUseCase
	resend          *tprm.ResendVendorAssessmentUseCase
}

// NewVendorAssessmentHandler builds every use case from one set of dependencies,
// so they cannot be wired inconsistently.
func NewVendorAssessmentHandler(deps tprm.AssessmentDeps) *VendorAssessmentHandler {
	return &VendorAssessmentHandler{
		createTemplate:  tprm.NewCreateQuestionnaireTemplateUseCase(deps),
		listTemplates:   tprm.NewListQuestionnaireTemplatesUseCase(deps),
		getTemplate:     tprm.NewGetQuestionnaireTemplateUseCase(deps),
		updateTemplate:  tprm.NewUpdateQuestionnaireTemplateUseCase(deps),
		archiveTemplate: tprm.NewArchiveQuestionnaireTemplateUseCase(deps),
		send:            tprm.NewSendVendorAssessmentUseCase(deps),
		listAssessments: tprm.NewListVendorAssessmentsUseCase(deps),
		getAssessment:   tprm.NewGetVendorAssessmentUseCase(deps),
		revoke:          tprm.NewRevokeVendorAssessmentUseCase(deps),
		resend:          tprm.NewResendVendorAssessmentUseCase(deps),
	}
}

func (h *VendorAssessmentHandler) ListTemplates(c *fiber.Ctx) error {
	list, err := h.listTemplates.Execute(c.UserContext(), tenantID(c), c.Query("include_archived") == "true")
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(list)
}

func (h *VendorAssessmentHandler) CreateTemplate(c *fiber.Ctx) error {
	var in tprm.TemplateInput
	if err := c.BodyParser(&in); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	t, err := h.createTemplate.Execute(c.UserContext(), tenantID(c), userID(c), in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(t)
}

func (h *VendorAssessmentHandler) GetTemplate(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id", "questionnaire template")
	if err != nil {
		return writeAppError(c, err)
	}
	t, err := h.getTemplate.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(t)
}

func (h *VendorAssessmentHandler) UpdateTemplate(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id", "questionnaire template")
	if err != nil {
		return writeAppError(c, err)
	}
	var in tprm.TemplateInput
	if err := c.BodyParser(&in); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	t, err := h.updateTemplate.Execute(c.UserContext(), tenantID(c), userID(c), id, in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(t)
}

func (h *VendorAssessmentHandler) ArchiveTemplate(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id", "questionnaire template")
	if err != nil {
		return writeAppError(c, err)
	}
	t, err := h.archiveTemplate.Execute(c.UserContext(), tenantID(c), userID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(t)
}

func (h *VendorAssessmentHandler) ListAssessments(c *fiber.Ctx) error {
	vendorID, err := pathUUID(c, "id", "vendor")
	if err != nil {
		return writeAppError(c, err)
	}
	list, err := h.listAssessments.Execute(c.UserContext(), tenantID(c), vendorID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(list)
}

type sendAssessmentBody struct {
	TemplateID      string `json:"template_id"`
	DueAt           string `json:"due_at"`
	ContactEmail    string `json:"contact_email"`
	ContactLanguage string `json:"contact_language"`
}

func (h *VendorAssessmentHandler) SendAssessment(c *fiber.Ctx) error {
	vendorID, err := pathUUID(c, "id", "vendor")
	if err != nil {
		return writeAppError(c, err)
	}
	var body sendAssessmentBody
	if err := c.BodyParser(&body); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	templateID, err := uuid.Parse(strings.TrimSpace(body.TemplateID))
	if err != nil {
		return writeAppError(c, domain.NewValidationError("template_id must be a uuid"))
	}
	dueAt, err := time.Parse(time.RFC3339, strings.TrimSpace(body.DueAt))
	if err != nil {
		return writeAppError(c, domain.NewValidationError("due_at must be an RFC 3339 date-time"))
	}

	out, err := h.send.Execute(c.UserContext(), tenantID(c), userID(c), vendorID, tprm.SendAssessmentInput{
		TemplateID:      templateID,
		DueAt:           dueAt,
		ContactEmail:    body.ContactEmail,
		ContactLanguage: body.ContactLanguage,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	// The link may be in this body (mail unavailable or failed): never cache it.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *VendorAssessmentHandler) GetAssessment(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id", "vendor assessment")
	if err != nil {
		return writeAppError(c, err)
	}
	a, err := h.getAssessment.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(a)
}

func (h *VendorAssessmentHandler) RevokeAssessment(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id", "vendor assessment")
	if err != nil {
		return writeAppError(c, err)
	}
	a, err := h.revoke.Execute(c.UserContext(), tenantID(c), userID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(a)
}

func (h *VendorAssessmentHandler) ResendAssessment(c *fiber.Ctx) error {
	id, err := pathUUID(c, "id", "vendor assessment")
	if err != nil {
		return writeAppError(c, err)
	}
	out, err := h.resend.Execute(c.UserContext(), tenantID(c), userID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}
