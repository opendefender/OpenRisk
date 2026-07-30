// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

// CustomFieldHandler handles custom field endpoints
type CustomFieldHandler struct {
	service *service.CustomFieldService
}

// NewCustomFieldHandler creates a new custom field handler
func NewCustomFieldHandler() *CustomFieldHandler {
	return &CustomFieldHandler{
		service: service.NewCustomFieldService(),
	}
}

// CreateCustomField handles POST /custom-fields
// Creates a new custom field for the specified scope (risk or asset)
func (h *CustomFieldHandler) CreateCustomField(c *fiber.Ctx) error {
	userClaims := middleware.GetUserClaims(c)
	if userClaims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse request
	req := &domain.CreateCustomFieldRequest{}
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Name == "" || req.FieldType == "" || req.Scope == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing required fields: name, field_type, scope",
		})
	}

	// Create field (scoped to the caller's tenant)
	tenantID := safeGetUUID(c, "tenant_id")
	field, err := h.service.CreateCustomField(tenantID, userClaims.Sub, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(field)
}

// GetCustomField handles GET /custom-fields/:id
// Retrieves a specific custom field
func (h *CustomFieldHandler) GetCustomField(c *fiber.Ctx) error {
	fieldID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid field ID",
		})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	field, err := h.service.GetCustomField(tenantID, fieldID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Field not found",
		})
	}

	return c.JSON(field)
}

// ListCustomFields handles GET /custom-fields
// Lists all custom fields, optionally filtered by scope
func (h *CustomFieldHandler) ListCustomFields(c *fiber.Ctx) error {
	scopeParam := c.Query("scope")
	var scope *domain.CustomFieldScope

	if scopeParam != "" {
		s := domain.CustomFieldScope(scopeParam)
		switch s {
		case domain.CustomFieldScopeRisk, domain.CustomFieldScopeAsset:
			scope = &s
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid scope: must be 'risk' or 'asset'",
			})
		}
	}

	tenantID := safeGetUUID(c, "tenant_id")
	fields, err := h.service.ListCustomFields(tenantID, scope)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fields)
}

// UpdateCustomField handles PATCH /custom-fields/:id
// Updates a custom field
func (h *CustomFieldHandler) UpdateCustomField(c *fiber.Ctx) error {
	fieldID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid field ID",
		})
	}

	// Parse request
	req := &domain.UpdateCustomFieldRequest{}
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update field (tenant-scoped)
	tenantID := safeGetUUID(c, "tenant_id")
	field, err := h.service.UpdateCustomField(tenantID, fieldID, req)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Field not found",
		})
	}

	return c.JSON(field)
}

// DeleteCustomField handles DELETE /custom-fields/:id
// Deletes a custom field
func (h *CustomFieldHandler) DeleteCustomField(c *fiber.Ctx) error {
	fieldID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid field ID",
		})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	if err := h.service.DeleteCustomField(tenantID, fieldID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Field not found",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListCustomFieldsByScope handles GET /custom-fields/scope/:scope
// Lists fields for a specific scope
func (h *CustomFieldHandler) ListCustomFieldsByScope(c *fiber.Ctx) error {
	scopeParam := c.Params("scope")
	scope := domain.CustomFieldScope(scopeParam)

	switch scope {
	case domain.CustomFieldScopeRisk, domain.CustomFieldScopeAsset:
		// Valid scope
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid scope: must be 'risk' or 'asset'",
		})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	fields, err := h.service.GetCustomFieldsByScope(tenantID, scope)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fields)
}

// ApplyTemplate handles POST /custom-fields/templates/:id/apply
// Applies a custom field template
func (h *CustomFieldHandler) ApplyTemplate(c *fiber.Ctx) error {
	userClaims := middleware.GetUserClaims(c)
	if userClaims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	templateID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid template ID",
		})
	}

	// Apply template (tenant-scoped)
	tenantID := safeGetUUID(c, "tenant_id")
	fields, err := h.service.ApplyTemplate(tenantID, templateID, userClaims.Sub)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"fields": fields,
		"count":  len(fields),
	})
}
