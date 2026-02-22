package handlers

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/services"
)

// EnhancedDashboardHandler handles advanced dashboard analytics endpoints
type EnhancedDashboardHandler struct {
	dashboardDataService *services.DashboardDataService
}

// NewEnhancedDashboardHandler creates a new enhanced dashboard handler
func NewEnhancedDashboardHandler(dashboardDataService *services.DashboardDataService) *EnhancedDashboardHandler {
	return &EnhancedDashboardHandler{
		dashboardDataService: dashboardDataService,
	}
}

// GetDashboardMetrics retrieves KPI metrics for the dashboard
// GET /api/v1/dashboard/metrics
func (h *EnhancedDashboardHandler) GetDashboardMetrics(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	metrics, err := h.dashboardDataService.GetDashboardMetrics(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve dashboard metrics",
		})
	}

	return c.Status(http.StatusOK).JSON(metrics)
}

// GetRiskTrends retrieves 7-day risk trend data
// GET /api/v1/dashboard/risk-trends
func (h *EnhancedDashboardHandler) GetRiskTrends(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	trends, err := h.dashboardDataService.GetRiskTrends(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve risk trends",
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"trends": trends,
	})
}

// GetSeverityDistribution retrieves risk distribution by severity level
// GET /api/v1/dashboard/severity-distribution
func (h *EnhancedDashboardHandler) GetSeverityDistribution(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	distribution, err := h.dashboardDataService.GetSeverityDistribution(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve severity distribution",
		})
	}

	return c.Status(http.StatusOK).JSON(distribution)
}

// GetMitigationStatus retrieves mitigation count by status
// GET /api/v1/dashboard/mitigation-status
func (h *EnhancedDashboardHandler) GetMitigationStatus(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	status, err := h.dashboardDataService.GetMitigationStatus(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve mitigation status",
		})
	}

	return c.Status(http.StatusOK).JSON(status)
}

// GetTopRisks retrieves the top N risks by score
// GET /api/v1/dashboard/top-risks?limit=5
func (h *EnhancedDashboardHandler) GetTopRisks(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	limit := 5
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	risks, err := h.dashboardDataService.GetTopRisks(c.Context(), limit)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve top risks",
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"top_risks": risks,
		"count":     len(risks),
	})
}

// GetMitigationProgress retrieves mitigation progress tracking data
// GET /api/v1/dashboard/mitigation-progress?limit=10
func (h *EnhancedDashboardHandler) GetMitigationProgress(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	progress, err := h.dashboardDataService.GetMitigationProgress(c.Context(), limit)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve mitigation progress",
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"mitigations": progress,
		"count":       len(progress),
	})
}

// GetCompleteDashboard retrieves all dashboard data in a single request
// GET /api/v1/dashboard/complete
func (h *EnhancedDashboardHandler) GetCompleteDashboard(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	analytics, err := h.dashboardDataService.GetCompleteDashboardData(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve complete dashboard data",
		})
	}

	return c.Status(http.StatusOK).JSON(analytics)
}
