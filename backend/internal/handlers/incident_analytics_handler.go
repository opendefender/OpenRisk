// filepath: backend/internal/handlers/incident_analytics_handler.go
package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/services"
)

// IncidentAnalyticsHandler handles incident analytics and reporting
type IncidentAnalyticsHandler struct {
	incidentService *services.IncidentService
	trendService    *services.TrendAnalysisService
}

// NewIncidentAnalyticsHandler creates a new incident analytics handler
func NewIncidentAnalyticsHandler(
	incidentService *services.IncidentService,
	trendService *services.TrendAnalysisService,
) *IncidentAnalyticsHandler {
	return &IncidentAnalyticsHandler{
		incidentService: incidentService,
		trendService:    trendService,
	}
}

// GetIncidentMetrics retrieves comprehensive incident metrics
// GET /api/v1/incidents/analytics/metrics
func (h *IncidentAnalyticsHandler) GetIncidentMetrics(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	metrics := h.incidentService.GetIncidentMetrics(tenantID)

	return c.JSON(fiber.Map{
		"data":      metrics,
		"timestamp": fiber.Now(),
	})
}

// GetIncidentTrends retrieves incident trends over time
// GET /api/v1/incidents/analytics/trends?days=30
func (h *IncidentAnalyticsHandler) GetIncidentTrends(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	trendData, err := h.incidentService.GetIncidentTrendData(tenantID, days)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get trend data: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"data":  trendData,
		"days":  days,
		"count": len(trendData),
	})
}

// GetIncidentStats retrieves incident statistics
// GET /api/v1/incidents/analytics/stats
func (h *IncidentAnalyticsHandler) GetIncidentStats(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	stats := h.incidentService.GetIncidentStats(tenantID)

	return c.JSON(fiber.Map{
		"data": stats,
	})
}

// BulkUpdateIncidents updates multiple incidents
// POST /api/v1/incidents/bulk-update
func (h *IncidentAnalyticsHandler) BulkUpdateIncidents(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	var req struct {
		IncidentIDs []uint `json:"incident_ids" binding:"required"`
		Status      string `json:"status" binding:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.incidentService.BulkUpdateIncidentStatus(tenantID, req.IncidentIDs, req.Status); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to update incidents: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Successfully updated %d incidents", len(req.IncidentIDs)),
		"count":   len(req.IncidentIDs),
	})
}

// ExportIncidentMetrics exports incident metrics to JSON
// GET /api/v1/incidents/analytics/export
func (h *IncidentAnalyticsHandler) ExportIncidentMetrics(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	metrics := h.incidentService.GetIncidentMetrics(tenantID)
	trendData, err := h.incidentService.GetIncidentTrendData(tenantID, 30)
	if err != nil {
		trendData = []map[string]interface{}{}
	}

	exportData := fiber.Map{
		"export_type": "incident_analytics",
		"tenant_id":   tenantID,
		"exported_at": fiber.Now(),
		"metrics":     metrics,
		"trends":      trendData,
	}

	// Set headers for file download
	c.Set("Content-Disposition", "attachment; filename=incident-analytics.json")
	c.Set("Content-Type", "application/json")

	return c.JSON(exportData)
}

// RegisterIncidentAnalyticsRoutes registers incident analytics routes
func RegisterIncidentAnalyticsRoutes(router fiber.Router, handler *IncidentAnalyticsHandler) {
	analytics := router.Group("/incidents/analytics")

	analytics.Get("/metrics", handler.GetIncidentMetrics)
	analytics.Get("/trends", handler.GetIncidentTrends)
	analytics.Get("/stats", handler.GetIncidentStats)
	analytics.Get("/export", handler.ExportIncidentMetrics)

	incidents := router.Group("/incidents")
	incidents.Post("/bulk-update", handler.BulkUpdateIncidents)
}
