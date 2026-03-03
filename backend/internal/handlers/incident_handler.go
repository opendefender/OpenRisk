package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/models"
	"github.com/opendefender/openrisk/internal/services"
)

// IncidentHandler handles incident endpoints
type IncidentHandler struct {
	incidentService *services.IncidentService
}

// NewIncidentHandler creates a new incident handler
func NewIncidentHandler(incidentService *services.IncidentService) *IncidentHandler {
	return &IncidentHandler{
		incidentService: incidentService,
	}
}

// CreateIncident creates a new incident
// POST /incidents
func (h *IncidentHandler) CreateIncident(c *fiber.Ctx) error {
	var req models.IncidentCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	incident, err := h.incidentService.CreateIncident(tenantID, req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create incident: %v", err),
		})
	}

	return c.Status(http.StatusCreated).JSON(incident)
}

// GetIncident retrieves an incident by ID
// GET /incidents/:incidentId
func (h *IncidentHandler) GetIncident(c *fiber.Ctx) error {
	incidentID, err := strconv.ParseUint(c.Params("incidentId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	incident, err := h.incidentService.GetIncident(tenantID, uint(incidentID))
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Incident not found",
		})
	}

	return c.JSON(incident)
}

// ListIncidents lists incidents with filtering
// GET /incidents?status=open&severity=critical&limit=20&offset=0
func (h *IncidentHandler) ListIncidents(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	query := models.IncidentQuery{
		Status:   c.Query("status"),
		Severity: c.Query("severity"),
		Type:     c.Query("type"),
		Limit:    c.QueryInt("limit", 20),
		Offset:   c.QueryInt("offset", 0),
	}

	if riskIDStr := c.Query("risk_id"); riskIDStr != "" {
		if riskID, err := strconv.ParseUint(riskIDStr, 10, 32); err == nil {
			rid := uint(riskID)
			query.RiskID = &rid
		}
	}

	incidents, total, err := h.incidentService.ListIncidents(tenantID, query)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to list incidents: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"incidents": incidents,
		"total":     total,
		"limit":     query.Limit,
		"offset":    query.Offset,
	})
}

// UpdateIncident updates an incident
// PUT /incidents/:incidentId
func (h *IncidentHandler) UpdateIncident(c *fiber.Ctx) error {
	incidentID, err := strconv.ParseUint(c.Params("incidentId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	var req models.IncidentUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tenantID := c.Locals("tenant_id").(string)
	userID := c.Locals("user_id").(string)

	incident, err := h.incidentService.UpdateIncident(tenantID, uint(incidentID), req, userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to update incident: %v", err),
		})
	}

	return c.JSON(incident)
}

// DeleteIncident deletes an incident
// DELETE /incidents/:incidentId
func (h *IncidentHandler) DeleteIncident(c *fiber.Ctx) error {
	incidentID, err := strconv.ParseUint(c.Params("incidentId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	if err := h.incidentService.DeleteIncident(tenantID, uint(incidentID)); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to delete incident: %v", err),
		})
	}

	return c.Status(http.StatusNoContent).Send(nil)
}

// GetIncidentTimeline retrieves incident timeline
// GET /incidents/:incidentId/timeline
func (h *IncidentHandler) GetIncidentTimeline(c *fiber.Ctx) error {
	incidentID, err := strconv.ParseUint(c.Params("incidentId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	timeline, err := h.incidentService.GetTimeline(uint(incidentID))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get timeline: %v", err),
		})
	}

	return c.JSON(timeline)
}

// LinkRisk links incident to a risk
// POST /incidents/:incidentId/link-risk/:riskId
func (h *IncidentHandler) LinkRisk(c *fiber.Ctx) error {
	incidentID, err := strconv.ParseUint(c.Params("incidentId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	riskID, err := strconv.ParseUint(c.Params("riskId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	if err := h.incidentService.LinkRisk(uint(incidentID), uint(riskID)); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to link risk: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Risk linked successfully",
	})
}

// CreateIncidentAction creates a mitigation action
// POST /incidents/:incidentId/actions
func (h *IncidentHandler) CreateIncidentAction(c *fiber.Ctx) error {
	incidentID, err := strconv.ParseUint(c.Params("incidentId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	var req struct {
		Title       string    `json:"title" binding:"required"`
		Description string    `json:"description"`
		AssignedTo  string    `json:"assigned_to" binding:"required"`
		DueDate     time.Time `json:"due_date" binding:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	action, err := h.incidentService.CreateIncidentAction(
		uint(incidentID), req.Title, req.Description, req.DueDate, req.AssignedTo)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create action: %v", err),
		})
	}

	return c.Status(http.StatusCreated).JSON(action)
}

// GetIncidentActions retrieves incident actions
// GET /incidents/:incidentId/actions
func (h *IncidentHandler) GetIncidentActions(c *fiber.Ctx) error {
	incidentID, err := strconv.ParseUint(c.Params("incidentId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	actions, err := h.incidentService.GetIncidentActions(uint(incidentID))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get actions: %v", err),
		})
	}

	return c.JSON(actions)
}

// UpdateIncidentAction updates action status
// PUT /incidents/:incidentId/actions/:actionId
func (h *IncidentHandler) UpdateIncidentAction(c *fiber.Ctx) error {
	actionID, err := strconv.ParseUint(c.Params("actionId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid action ID",
		})
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.incidentService.UpdateIncidentAction(uint(actionID), req.Status); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to update action: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Action updated successfully",
	})
}

// GetIncidentStats returns incident statistics
// GET /incidents/stats
func (h *IncidentHandler) GetIncidentStats(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	stats := h.incidentService.GetIncidentStats(tenantID)

	return c.JSON(stats)
}

// GetIncidentsForRisk retrieves incidents for a specific risk
// GET /risks/:riskId/incidents
func (h *IncidentHandler) GetIncidentsForRisk(c *fiber.Ctx) error {
	riskID, err := strconv.ParseUint(c.Params("riskId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	incidents, err := h.incidentService.GetIncidentsForRisk(tenantID, uint(riskID))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get incidents: %v", err),
		})
	}

	return c.JSON(incidents)
}

// GetIncidents retrieves all incidents with pagination
func (h *IncidentHandler) GetIncidents(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	query := models.IncidentQuery{
		Status:   c.Query("status"),
		Severity: c.Query("severity"),
		Type:     c.Query("type"),
		Limit:    c.QueryInt("limit", 20),
		Offset:   c.QueryInt("offset", 0),
	}

	incidents, total, err := h.incidentService.ListIncidents(tenantID, query)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to list incidents: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"incidents": incidents,
		"total":     total,
	})
}
