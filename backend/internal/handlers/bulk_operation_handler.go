package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// BulkOperationHandler handles bulk operation endpoints
type BulkOperationHandler struct {
	service *services.BulkOperationService
}

// NewBulkOperationHandler creates a new bulk operation handler
func NewBulkOperationHandler() *BulkOperationHandler {
	return &BulkOperationHandler{
		service: services.NewBulkOperationService(),
	}
}

// CreateBulkOperation handles POST /bulk-operations
// Creates a new bulk operation job
func (h *BulkOperationHandler) CreateBulkOperation(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*domain.UserClaims)
	if userClaims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse request
	req := &domain.CreateBulkOperationRequest{}
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate operation type
	if req.OperationType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing required field: operation_type",
		})
	}

	// Create bulk operation
	op, err := h.service.CreateBulkOperation(userClaims.ID, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(op)
}

// GetBulkOperation handles GET /bulk-operations/:id
// Retrieves a specific bulk operation
func (h *BulkOperationHandler) GetBulkOperation(c *fiber.Ctx) error {
	opID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid operation ID",
		})
	}

	op, err := h.service.GetBulkOperation(opID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Operation not found",
		})
	}

	return c.JSON(op)
}

// ListBulkOperations handles GET /bulk-operations
// Lists bulk operations for the authenticated user
func (h *BulkOperationHandler) ListBulkOperations(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*domain.UserClaims)
	if userClaims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	limit := 20
	if l := c.QueryInt("limit"); l > 0 && l <= 100 {
		limit = l
	}

	offset := 0
	if o := c.QueryInt("offset"); o >= 0 {
		offset = o
	}

	ops, err := h.service.ListBulkOperations(userClaims.ID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"operations": ops,
		"count":      len(ops),
	})
}
