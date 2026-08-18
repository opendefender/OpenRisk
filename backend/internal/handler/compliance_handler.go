// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"fmt"
	"github.com/opendefender/openrisk/pkg/monitoring"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/report"
	"github.com/opendefender/openrisk/pkg/validation"
)

// ComplianceHandler encapsulates the compliance use cases.
//
// NEVER call Preload("Controls") on domain.ComplianceFramework from this
// handler (or add such a call anywhere else): that GORM relation has no
// tenant filter — frameworks are global, controls are tenant-scoped — so
// eager-loading it would mix every tenant's controls together. Always go
// through ListControls (tenant-scoped) instead.
type ComplianceHandler struct {
	createFrameworkUC *compliance.CreateFrameworkUseCase
	getFrameworkUC    *compliance.GetFrameworkUseCase
	listFrameworksUC  *compliance.ListFrameworksUseCase
	deleteFrameworkUC *compliance.DeleteFrameworkUseCase
	createControlUC   *compliance.CreateControlUseCase
	getControlUC      *compliance.GetControlUseCase
	listControlsUC    *compliance.ListControlsUseCase
	updateControlUC   *compliance.UpdateControlUseCase
	deleteControlUC   *compliance.DeleteControlUseCase
	getProgressUC     *compliance.GetComplianceProgressUseCase
	listCatalogsUC    *compliance.ListCatalogsUseCase
	importCatalogUC   *compliance.ImportCatalogUseCase
	generateReportUC  *compliance.GenerateComplianceReportUseCase
	getGapAnalysisUC  *compliance.GetGapAnalysisUseCase

	createMappingUC *compliance.CreateControlMappingUseCase
	listMappingsUC  *compliance.ListControlMappingsUseCase
	deleteMappingUC *compliance.DeleteControlMappingUseCase
	// Optional: absent on deployments that do not wire crosswalks, and the
	// endpoint says so rather than pretending the tenant inherits nothing.
	inheritedCoverageUC *compliance.GetInheritedCoverageUseCase
}

func NewComplianceHandler(
	createFramework *compliance.CreateFrameworkUseCase,
	getFramework *compliance.GetFrameworkUseCase,
	listFrameworks *compliance.ListFrameworksUseCase,
	deleteFramework *compliance.DeleteFrameworkUseCase,
	createControl *compliance.CreateControlUseCase,
	getControl *compliance.GetControlUseCase,
	listControls *compliance.ListControlsUseCase,
	updateControl *compliance.UpdateControlUseCase,
	deleteControl *compliance.DeleteControlUseCase,
	getProgress *compliance.GetComplianceProgressUseCase,
	listCatalogs *compliance.ListCatalogsUseCase,
	importCatalog *compliance.ImportCatalogUseCase,
	generateReport *compliance.GenerateComplianceReportUseCase,
	getGapAnalysis *compliance.GetGapAnalysisUseCase,
	createMapping *compliance.CreateControlMappingUseCase,
	listMappings *compliance.ListControlMappingsUseCase,
	deleteMapping *compliance.DeleteControlMappingUseCase,
) *ComplianceHandler {
	return &ComplianceHandler{
		createFrameworkUC: createFramework,
		getFrameworkUC:    getFramework,
		listFrameworksUC:  listFrameworks,
		deleteFrameworkUC: deleteFramework,
		createControlUC:   createControl,
		getControlUC:      getControl,
		listControlsUC:    listControls,
		updateControlUC:   updateControl,
		deleteControlUC:   deleteControl,
		getProgressUC:     getProgress,
		listCatalogsUC:    listCatalogs,
		importCatalogUC:   importCatalog,
		generateReportUC:  generateReport,
		getGapAnalysisUC:  getGapAnalysis,
		createMappingUC:   createMapping,
		listMappingsUC:    listMappings,
		deleteMappingUC:   deleteMapping,
	}
}

func writeAppError(c *fiber.Ctx, err error) error {
	return c.Status(domain.HTTPStatusFromError(err)).JSON(fiber.Map{"error": domain.MessageFromError(err)})
}

func tenantID(c *fiber.Ctx) uuid.UUID {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil {
		return uuid.Nil
	}
	return mwCtx.OrganizationID
}

func userID(c *fiber.Ctx) uuid.UUID {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil {
		return uuid.Nil
	}
	return mwCtx.UserID
}

// =============================================================================
// Frameworks (global)
// =============================================================================

type createFrameworkInput struct {
	Name        string `json:"name" validate:"required"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// CreateFramework godoc
func (h *ComplianceHandler) CreateFramework(c *fiber.Ctx) error {
	input := new(createFrameworkInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	fw, err := h.createFrameworkUC.Execute(c.UserContext(), tenantID(c), compliance.CreateFrameworkInput{
		Name: input.Name, Version: input.Version, Description: input.Description,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(fw)
}

// GetFramework godoc
func (h *ComplianceHandler) GetFramework(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}
	fw, err := h.getFrameworkUC.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fw)
}

// ListFrameworks godoc
func (h *ComplianceHandler) ListFrameworks(c *fiber.Ctx) error {
	// ?imported=true narrows to frameworks that actually carry controls — what
	// the risk form's compliance selector needs. Anything else is not mappable.
	if c.Query("imported") == "true" {
		frameworks, err := h.listFrameworksUC.ExecuteImported(c.UserContext(), tenantID(c))
		if err != nil {
			return writeAppError(c, err)
		}
		return c.JSON(frameworks)
	}
	frameworks, err := h.listFrameworksUC.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(frameworks)
}

// DeleteFramework godoc — removes a framework and the caller's controls under it.
// Admin/root-only (route-gated). Returns 204 on success, 404 if unknown.
func (h *ComplianceHandler) DeleteFramework(c *fiber.Ctx) error {
	frameworkID, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}
	if err := h.deleteFrameworkUC.Execute(c.UserContext(), tenantID(c), frameworkID); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// GetProgress godoc
func (h *ComplianceHandler) GetProgress(c *fiber.Ctx) error {
	frameworkID, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}
	progress, err := h.getProgressUC.Execute(c.UserContext(), tenantID(c), frameworkID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(progress)
}

// GetGapAnalysis godoc — the "analyse d'écarts" endpoint. Returns every
// unsatisfied control (not implemented / in progress) across the tenant's
// frameworks, with per-framework and overall roll-ups. An optional
// ?framework_id=<uuid> query param scopes the analysis to a single framework.
func (h *ComplianceHandler) GetGapAnalysis(c *fiber.Ctx) error {
	frameworkID := uuid.Nil
	if raw := c.Query("framework_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
		}
		frameworkID = id
	}
	analysis, err := h.getGapAnalysisUC.Execute(c.UserContext(), tenantID(c), frameworkID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(analysis)
}

// =============================================================================
// Cross-framework control mappings ("cross-mapping entre référentiels")
// =============================================================================

// ListControlMappings GET /compliance/control-mappings[?control_id=] — the
// tenant's crosswalks, optionally scoped to one control.
func (h *ComplianceHandler) ListControlMappings(c *fiber.Ctx) error {
	var controlID *uuid.UUID
	if raw := c.Query("control_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
		}
		controlID = &id
	}
	mappings, err := h.listMappingsUC.Execute(c.UserContext(), tenantID(c), controlID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(mappings)
}

type createCrosswalkInput struct {
	SourceControlID string `json:"source_control_id" validate:"required"`
	TargetControlID string `json:"target_control_id" validate:"required"`
	Coverage        string `json:"coverage"`
	Rationale       string `json:"rationale"`
}

// GetInheritedCoverage GET /compliance/frameworks/:frameworkId/inherited-coverage
// — how much of this framework the tenant's existing proof already answers.
func (h *ComplianceHandler) GetInheritedCoverage(c *fiber.Ctx) error {
	frameworkID, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}
	if h.inheritedCoverageUC == nil {
		return c.Status(501).JSON(fiber.Map{"error": "inherited coverage is not configured"})
	}
	cov, err := h.inheritedCoverageUC.Execute(c.UserContext(), tenantID(c), frameworkID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(cov)
}

// WithInheritedCoverage attaches the head-start use case. A builder rather than
// an eighteenth positional argument to the constructor.
func (h *ComplianceHandler) WithInheritedCoverage(uc *compliance.GetInheritedCoverageUseCase) *ComplianceHandler {
	h.inheritedCoverageUC = uc
	return h
}

// CreateControlMapping POST /compliance/control-mappings — link two controls
// (normally across frameworks).
func (h *ComplianceHandler) CreateControlMapping(c *fiber.Ctx) error {
	input := new(createCrosswalkInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	src, err := uuid.Parse(input.SourceControlID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid source control id"})
	}
	tgt, err := uuid.Parse(input.TargetControlID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid target control id"})
	}
	m, err := h.createMappingUC.Execute(c.UserContext(), tenantID(c), userID(c), compliance.CreateControlCrosswalkInput{
		SourceControlID: src,
		TargetControlID: tgt,
		Coverage:        input.Coverage,
		Rationale:       input.Rationale,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(m)
}

// DeleteControlMapping DELETE /compliance/control-mappings/:mappingId
func (h *ComplianceHandler) DeleteControlMapping(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("mappingId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid mapping id"})
	}
	if err := h.deleteMappingUC.Execute(c.UserContext(), tenantID(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// GenerateReport godoc — streams an official compliance report (PDF) for one
// framework in a single click. Data is strictly tenant-scoped; the framework is
// global but only the requesting tenant's controls/evidence appear. The locale
// query param (fr|en) selects the fixed-label language, defaulting to French.
func (h *ComplianceHandler) GenerateReport(c *fiber.Ctx) error {
	frameworkID, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}

	locale := report.LocaleFR
	if c.Query("locale") == "en" {
		locale = report.LocaleEN
	}

	data, err := h.generateReportUC.Execute(c.UserContext(), tenantID(c), frameworkID, userID(c), locale)
	if err != nil {
		return writeAppError(c, err)
	}

	pdf, err := report.RenderCompliancePDF(*data)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to render report"})
	}

	monitoring.RecordReportGenerated("compliance")
	filename := reportFilename(data.FrameworkName, data.FrameworkVersion)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Send(pdf)
}

// reportFilename builds a safe, descriptive PDF filename from the framework
// identity, e.g. "compliance-report-iso-iec-27001-2022.pdf".
func reportFilename(name, version string) string {
	slug := func(s string) string {
		var b strings.Builder
		prevDash := false
		for _, r := range strings.ToLower(s) {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				b.WriteRune(r)
				prevDash = false
			default:
				if !prevDash && b.Len() > 0 {
					b.WriteByte('-')
					prevDash = true
				}
			}
		}
		return strings.Trim(b.String(), "-")
	}
	base := "compliance-report"
	if s := slug(name); s != "" {
		base += "-" + s
	}
	if s := slug(version); s != "" {
		base += "-" + s
	}
	return base + ".pdf"
}

// ListCatalogs godoc
// Lists every registered regulatory catalog (global, not tenant-scoped) — available ones
// (e.g. ISO 27001:2022) can be imported via ImportCatalog; unavailable ones are shown so the
// UI can list them as "coming soon" instead of hiding them (see ROADMAP.md M2).
func (h *ComplianceHandler) ListCatalogs(c *fiber.Ctx) error {
	return c.JSON(h.listCatalogsUC.Execute(c.UserContext()))
}

type importCatalogInput struct {
	CatalogKey string `json:"catalog_key" validate:"required"`
}

// ImportCatalog godoc
// Bulk-creates this tenant's controls under the given framework from a regulatory catalog
// (e.g. ISO 27001:2022's 93 Annex A controls), instead of requiring CreateControl calls one
// at a time. Idempotent — safe to call again (e.g. after a catalog is extended).
func (h *ComplianceHandler) ImportCatalog(c *fiber.Ctx) error {
	frameworkID, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}
	input := new(importCatalogInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	result, err := h.importCatalogUC.Execute(c.UserContext(), tenantID(c), compliance.ImportCatalogInput{
		FrameworkID: frameworkID, CatalogKey: input.CatalogKey,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(result)
}

// =============================================================================
// Controls (tenant-scoped)
// =============================================================================

type createControlInput struct {
	ReferenceCode string `json:"reference_code"`
	Name          string `json:"name" validate:"required"`
	Description   string `json:"description"`
}

// CreateControl godoc
func (h *ComplianceHandler) CreateControl(c *fiber.Ctx) error {
	frameworkID, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}

	input := new(createControlInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	control, err := h.createControlUC.Execute(c.UserContext(), tenantID(c), compliance.CreateControlInput{
		FrameworkID: frameworkID, ReferenceCode: input.ReferenceCode, Name: input.Name, Description: input.Description,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(control)
}

// ListControls godoc
func (h *ComplianceHandler) ListControls(c *fiber.Ctx) error {
	frameworkID, err := uuid.Parse(c.Params("frameworkId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
	}
	controls, err := h.listControlsUC.Execute(c.UserContext(), tenantID(c), frameworkID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(controls)
}

// GetControl godoc
func (h *ComplianceHandler) GetControl(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}
	control, err := h.getControlUC.Execute(c.UserContext(), tenantID(c), controlID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(control)
}

type updateControlInput struct {
	ReferenceCode *string `json:"reference_code"`
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Status        *string `json:"status"`
}

// UpdateControl godoc
func (h *ComplianceHandler) UpdateControl(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}

	input := new(updateControlInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}

	ucInput := compliance.UpdateControlInput{
		ReferenceCode: input.ReferenceCode,
		Name:          input.Name,
		Description:   input.Description,
	}
	if input.Status != nil {
		s := domain.ControlStatus(*input.Status)
		ucInput.Status = &s
	}

	control, err := h.updateControlUC.Execute(c.UserContext(), tenantID(c), controlID, ucInput)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(control)
}

// DeleteControl godoc
func (h *ComplianceHandler) DeleteControl(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}
	if err := h.deleteControlUC.Execute(c.UserContext(), tenantID(c), controlID); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// Evidence lives in its own module now (application/evidence, EvidenceHandler).
// The per-control routes kept their URLs but are served from there, because
// evidence is reusable across controls and two writers into one register would
// mean two truths. See internal/handler/evidence_handler.go.
