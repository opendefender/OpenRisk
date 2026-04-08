package handler

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

// AuditLogHandler handles audit log endpoints
type AuditLogHandler struct {
	auditService *service.AuditService
}

// NewAuditLogHandler creates a new audit log handler
func NewAuditLogHandler() *AuditLogHandler {
	return &AuditLogHandler{
		auditService: service.NewAuditService(),
	}
}

type AuditLogDTO struct {
	ID           string  `json:"id"`
	UserID       *string `json:"user_id,omitempty"`
	Action       string  `json:"action"`
	Resource     string  `json:"resource,omitempty"`
	ResourceID   *string `json:"resource_id,omitempty"`
	Result       string  `json:"result"`
	ErrorMessage string  `json:"error_message,omitempty"`
	IPAddress    *string `json:"ip_address,omitempty"`
	UserAgent    string  `json:"user_agent,omitempty"`
	Timestamp    string  `json:"timestamp"`
}

// GetAuditLogs retrieves all audit logs (admin only)
// Query parameters: page (default 1), limit (default 20), action, result, user_id
func (h *AuditLogHandler) GetAuditLogs(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	if !claims.HasPermission("*") && claims.RoleName != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can view audit logs"})
	}

	// Parse pagination
	page := 1
	if p := c.Query("page"); p != "" {
		if parsedPage, err := strconv.Atoi(p); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	offset := (page - 1) * limit

	// Get audit logs by date range (last 30 days by default)
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -30)

	logs, err := h.auditService.GetAuditLogsByDateRange(startTime, endTime, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve audit logs"})
	}

	// Convert to DTO
	response := make([]AuditLogDTO, 0, len(logs))
	for _, log := range logs {
		dto := AuditLogDTO{
			ID:           log.ID.String(),
			Action:       log.Action.String(),
			Resource:     log.Resource.String(),
			Result:       log.Result.String(),
			ErrorMessage: log.ErrorMessage,
			UserAgent:    log.UserAgent,
			Timestamp:    log.Timestamp.Format("2006-01-02T15:04:05Z"),
		}

		if log.UserID != nil {
			userIDStr := log.UserID.String()
			dto.UserID = &userIDStr
		}

		if log.ResourceID != nil {
			resourceIDStr := log.ResourceID.String()
			dto.ResourceID = &resourceIDStr
		}

		if log.IPAddress != nil {
			ipStr := log.IPAddress.String()
			dto.IPAddress = &ipStr
		}

		response = append(response, dto)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":  response,
		"page":  page,
		"limit": limit,
		"count": len(response),
	})
}

// GetUserAuditLogs retrieves audit logs for a specific user (admin only)
func (h *AuditLogHandler) GetUserAuditLogs(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	if !claims.HasPermission("*") && claims.RoleName != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can view audit logs"})
	}

	userID := c.Params("user_id")

	// Parse pagination
	page := 1
	if p := c.Query("page"); p != "" {
		if parsedPage, err := strconv.Atoi(p); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	offset := (page - 1) * limit

	logs, err := h.auditService.GetAuditLogsByUser(parseUUID(userID), limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve audit logs"})
	}

	// Convert to DTO
	response := make([]AuditLogDTO, 0, len(logs))
	for _, log := range logs {
		dto := AuditLogDTO{
			ID:           log.ID.String(),
			Action:       log.Action.String(),
			Resource:     log.Resource.String(),
			Result:       log.Result.String(),
			ErrorMessage: log.ErrorMessage,
			UserAgent:    log.UserAgent,
			Timestamp:    log.Timestamp.Format("2006-01-02T15:04:05Z"),
		}

		if log.UserID != nil {
			userIDStr := log.UserID.String()
			dto.UserID = &userIDStr
		}

		if log.ResourceID != nil {
			resourceIDStr := log.ResourceID.String()
			dto.ResourceID = &resourceIDStr
		}

		if log.IPAddress != nil {
			ipStr := log.IPAddress.String()
			dto.IPAddress = &ipStr
		}

		response = append(response, dto)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":  response,
		"page":  page,
		"limit": limit,
		"count": len(response),
	})
}

// GetAuditLogsByAction retrieves audit logs for a specific action (admin only)
func (h *AuditLogHandler) GetAuditLogsByAction(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	if !claims.HasPermission("*") && claims.RoleName != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can view audit logs"})
	}

	action := c.Params("action")

	// Parse pagination
	page := 1
	if p := c.Query("page"); p != "" {
		if parsedPage, err := strconv.Atoi(p); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	offset := (page - 1) * limit

	logs, err := h.auditService.GetAuditLogsByAction(domain.AuditLogAction(action), limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve audit logs"})
	}

	// Convert to DTO
	response := make([]AuditLogDTO, 0, len(logs))
	for _, log := range logs {
		dto := AuditLogDTO{
			ID:           log.ID.String(),
			Action:       log.Action.String(),
			Resource:     log.Resource.String(),
			Result:       log.Result.String(),
			ErrorMessage: log.ErrorMessage,
			UserAgent:    log.UserAgent,
			Timestamp:    log.Timestamp.Format("2006-01-02T15:04:05Z"),
		}

		if log.UserID != nil {
			userIDStr := log.UserID.String()
			dto.UserID = &userIDStr
		}

		if log.ResourceID != nil {
			resourceIDStr := log.ResourceID.String()
			dto.ResourceID = &resourceIDStr
		}

		if log.IPAddress != nil {
			ipStr := log.IPAddress.String()
			dto.IPAddress = &ipStr
		}

		response = append(response, dto)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":  response,
		"page":  page,
		"limit": limit,
		"count": len(response),
	})
}

// Helper function to parse UUID from string
func parseUUID(uuidStr string) uuid.UUID {
	id, _ := uuid.Parse(uuidStr)
	return id
}
