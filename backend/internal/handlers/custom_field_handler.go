package handlers

import (
	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// CustomFieldHandler handles custom field endpoints
type CustomFieldHandler struct {
	service services.CustomFieldService
}

// NewCustomFieldHandler creates a new custom field handler
func NewCustomFieldHandler() CustomFieldHandler {
	return &CustomFieldHandler{
		service: services.NewCustomFieldService(),
	}
}

// CreateCustomField handles POST /custom-fields
// Creates a new custom field for the specified scope (risk or asset)
func (h CustomFieldHandler) CreateCustomField(c fiber.Ctx) error {
	userClaims := c.Locals("user").(domain.UserClaims)
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

	// Create field
	field, err := h.service.CreateCustomField(userClaims.ID, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(field)
}

// GetCustomField handles GET /custom-fields/:id
// Retrieves a specific custom field
func (h CustomFieldHandler) GetCustomField(c fiber.Ctx) error {
	fieldID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid field ID",
		})
	}

	field, err := h.service.GetCustomField(fieldID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Field not found",
		})
	}

	return c.JSON(field)
}

// ListCustomFields handles GET /custom-fields
// Lists all custom fields, optionally filtered by scope
func (h CustomFieldHandler) ListCustomFields(c fiber.Ctx) error {
	scopeParam := c.Query("scope")
	var scope domain.CustomFieldScope

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

	fields, err := h.service.ListCustomFields(scope)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fields)
}

// UpdateCustomField handles PATCH /custom-fields/:id
// Updates a custom field
func (h CustomFieldHandler) UpdateCustomField(c fiber.Ctx) error {
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

	// Update field
	field, err := h.service.UpdateCustomField(fieldID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(field)
}

// DeleteCustomField handles DELETE /custom-fields/:id
// Deletes a custom field
func (h CustomFieldHandler) DeleteCustomField(c fiber.Ctx) error {
	fieldID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid field ID",
		})
	}

	if err := h.service.DeleteCustomField(fieldID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListCustomFieldsByScope handles GET /custom-fields/scope/:scope
// Lists fields for a specific scope
func (h CustomFieldHandler) ListCustomFieldsByScope(c fiber.Ctx) error {
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

	fields, err := h.service.GetCustomFieldsByScope(scope)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fields)
}

// ApplyTemplate handles POST /custom-fields/templates/:id/apply
// Applies a custom field template
func (h CustomFieldHandler) ApplyTemplate(c fiber.Ctx) error {
	userClaims := c.Locals("user").(domain.UserClaims)
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

	// Apply template
	fields, err := h.service.ApplyTemplate(templateID, userClaims.ID)
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
