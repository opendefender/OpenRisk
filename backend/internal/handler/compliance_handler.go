// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
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
	createFrameworkUC  *compliance.CreateFrameworkUseCase
	getFrameworkUC     *compliance.GetFrameworkUseCase
	listFrameworksUC   *compliance.ListFrameworksUseCase
	createControlUC    *compliance.CreateControlUseCase
	getControlUC       *compliance.GetControlUseCase
	listControlsUC     *compliance.ListControlsUseCase
	updateControlUC    *compliance.UpdateControlUseCase
	deleteControlUC    *compliance.DeleteControlUseCase
	createEvidenceUC   *compliance.CreateEvidenceUseCase
	listEvidencesUC    *compliance.ListEvidencesUseCase
	deleteEvidenceUC   *compliance.DeleteEvidenceUseCase
	downloadEvidenceUC *compliance.DownloadEvidenceUseCase
	getProgressUC      *compliance.GetComplianceProgressUseCase
	listCatalogsUC     *compliance.ListCatalogsUseCase
	importCatalogUC    *compliance.ImportCatalogUseCase
}

func NewComplianceHandler(
	createFramework *compliance.CreateFrameworkUseCase,
	getFramework *compliance.GetFrameworkUseCase,
	listFrameworks *compliance.ListFrameworksUseCase,
	createControl *compliance.CreateControlUseCase,
	getControl *compliance.GetControlUseCase,
	listControls *compliance.ListControlsUseCase,
	updateControl *compliance.UpdateControlUseCase,
	deleteControl *compliance.DeleteControlUseCase,
	createEvidence *compliance.CreateEvidenceUseCase,
	listEvidences *compliance.ListEvidencesUseCase,
	deleteEvidence *compliance.DeleteEvidenceUseCase,
	downloadEvidence *compliance.DownloadEvidenceUseCase,
	getProgress *compliance.GetComplianceProgressUseCase,
	listCatalogs *compliance.ListCatalogsUseCase,
	importCatalog *compliance.ImportCatalogUseCase,
) *ComplianceHandler {
	return &ComplianceHandler{
		createFrameworkUC:  createFramework,
		getFrameworkUC:     getFramework,
		listFrameworksUC:   listFrameworks,
		createControlUC:    createControl,
		getControlUC:       getControl,
		listControlsUC:     listControls,
		updateControlUC:    updateControl,
		deleteControlUC:    deleteControl,
		createEvidenceUC:   createEvidence,
		listEvidencesUC:    listEvidences,
		deleteEvidenceUC:   deleteEvidence,
		downloadEvidenceUC: downloadEvidence,
		getProgressUC:      getProgress,
		listCatalogsUC:     listCatalogs,
		importCatalogUC:    importCatalog,
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

	fw, err := h.createFrameworkUC.Execute(c.UserContext(), compliance.CreateFrameworkInput{
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
	fw, err := h.getFrameworkUC.Execute(c.UserContext(), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fw)
}

// ListFrameworks godoc
func (h *ComplianceHandler) ListFrameworks(c *fiber.Ctx) error {
	frameworks, err := h.listFrameworksUC.Execute(c.UserContext())
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(frameworks)
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

// =============================================================================
// Evidences (tenant-scoped)
// =============================================================================

// CreateEvidence godoc — multipart/form-data: file (required), description (optional).
func (h *ComplianceHandler) CreateEvidence(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file is required"})
	}
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "failed to read uploaded file"})
	}
	defer file.Close()

	evidence, err := h.createEvidenceUC.Execute(c.UserContext(), tenantID(c), compliance.CreateEvidenceInput{
		ControlID:   controlID,
		Filename:    fileHeader.Filename,
		Description: c.FormValue("description"),
		Content:     file,
		UploadedBy:  userID(c),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(evidence)
}

// ListEvidences godoc
func (h *ComplianceHandler) ListEvidences(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}
	evidences, err := h.listEvidencesUC.Execute(c.UserContext(), tenantID(c), controlID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(evidences)
}

// DownloadEvidence godoc — the only path to evidence file content; no
// public/static route exists for these files (see storage.Storage doc).
func (h *ComplianceHandler) DownloadEvidence(c *fiber.Ctx) error {
	evidenceID, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}

	evidence, content, err := h.downloadEvidenceUC.Execute(c.UserContext(), tenantID(c), evidenceID)
	if err != nil {
		return writeAppError(c, err)
	}
	// No defer content.Close() here: SendStream hands the reader to
	// fasthttp, which reads (and closes, since it implements io.Closer)
	// the stream lazily *after* this handler returns while serializing
	// the response — closing it here would race the actual write.

	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, evidence.Filename))
	c.Set(fiber.HeaderContentType, "application/octet-stream")
	return c.SendStream(content)
}

// DeleteEvidence godoc
func (h *ComplianceHandler) DeleteEvidence(c *fiber.Ctx) error {
	evidenceID, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}
	if err := h.deleteEvidenceUC.Execute(c.UserContext(), tenantID(c), evidenceID); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}
