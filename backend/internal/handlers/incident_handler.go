package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// IncidentHandler manages incident endpoints
type IncidentHandler struct {
	db *gorm.DB
}

// NewIncidentHandler creates a new incident handler
func NewIncidentHandler(db *gorm.DB) *IncidentHandler {
	return &IncidentHandler{db: db}
}

// GetIncidents retrieves all incidents with pagination
func (h *IncidentHandler) GetIncidents(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	severity := c.Query("severity")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	type IncidentResponse struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Severity    string `json:"severity"`
		Status      string `json:"status"`
		Date        string `json:"date"`
		Assignee    string `json:"assignee"`
		Description string `json:"description"`
	}

	var incidents []IncidentResponse
	query := h.db

	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	result := query.
		Offset(offset).
		Limit(limit).
		Find(&incidents)

	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": result.Error.Error()})
	}

	var total int64
	countQuery := h.db
	if severity != "" {
		countQuery = countQuery.Where("severity = ?", severity)
	}
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	countQuery.Model(&IncidentResponse{}).Count(&total)

	return c.JSON(fiber.Map{
		"incidents": incidents,
		"total":     total,
		"page":      page,
		"limit":     limit,
	})
}

// GetIncident retrieves a single incident by ID
func (h *IncidentHandler) GetIncident(c *fiber.Ctx) error {
	id := c.Params("id")

	type IncidentDetail struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Severity    string `json:"severity"`
		Status      string `json:"status"`
		Date        string `json:"date"`
		Assignee    string `json:"assignee"`
		Description string `json:"description"`
		Source      string `json:"source"`
		ExternalID  string `json:"external_id"`
	}

	var incident IncidentDetail
	result := h.db.Where("id = ?", id).First(&incident)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "Incident not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": result.Error.Error()})
	}

	return c.JSON(incident)
}
