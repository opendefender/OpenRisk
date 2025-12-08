package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/services"
)

// AnalyticsHandler handles analytics endpoints
type AnalyticsHandler struct {
	analyticsService *services.AnalyticsService
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(analyticsService *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
	}
}

// GetRiskMetrics retrieves aggregated risk metrics
// GET /api/v1/analytics/risks/metrics
func (h *AnalyticsHandler) GetRiskMetrics(c *fiber.Ctx) error {
	// Check permission
	userID := c.Locals("userID").(string)
	if userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	metrics, err := h.analyticsService.GetRiskMetrics(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve risk metrics",
		})
	}

	return c.Status(http.StatusOK).JSON(metrics)
}

// GetRiskTrends retrieves risk trends over time
// GET /api/v1/analytics/risks/trends?days=30
func (h *AnalyticsHandler) GetRiskTrends(c *fiber.Ctx) error {
	// Check permission
	userID := c.Locals("userID").(string)
	if userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Parse days parameter
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 && parsedDays <= 365 {
			days = parsedDays
		}
	}

	trends, err := h.analyticsService.GetRiskTrends(c.Context(), days)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve risk trends",
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"days":   days,
		"trends": trends,
	})
}

// GetMitigationMetrics retrieves mitigation analytics
// GET /api/v1/analytics/mitigations/metrics
func (h *AnalyticsHandler) GetMitigationMetrics(c *fiber.Ctx) error {
	// Check permission
	userID := c.Locals("userID").(string)
	if userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	metrics, err := h.analyticsService.GetMitigationMetrics(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve mitigation metrics",
		})
	}

	return c.Status(http.StatusOK).JSON(metrics)
}

// GetFrameworkAnalytics retrieves compliance analytics by framework
// GET /api/v1/analytics/frameworks
func (h *AnalyticsHandler) GetFrameworkAnalytics(c *fiber.Ctx) error {
	// Check permission
	userID := c.Locals("userID").(string)
	if userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	analytics, err := h.analyticsService.GetFrameworkAnalytics(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve framework analytics",
		})
	}

	return c.Status(http.StatusOK).JSON(analytics)
}

// GetDashboardSnapshot retrieves a complete dashboard snapshot
// GET /api/v1/analytics/dashboard
func (h *AnalyticsHandler) GetDashboardSnapshot(c *fiber.Ctx) error {
	// Check permission
	userID := c.Locals("userID").(string)
	if userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	snapshot, err := h.analyticsService.GetDashboardSnapshot(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve dashboard snapshot",
		})
	}

	return c.Status(http.StatusOK).JSON(snapshot)
}

// GetExportData exports analytics data in various formats
// GET /api/v1/analytics/export?format=json|csv|pdf
func (h *AnalyticsHandler) GetExportData(c *fiber.Ctx) error {
	// Check permission
	userID := c.Locals("userID").(string)
	if userID == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	format := c.Query("format", "json")

	snapshot, err := h.analyticsService.GetDashboardSnapshot(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve data for export",
		})
	}

	switch format {
	case "csv":
		return h.exportAsCSV(c, snapshot)
	case "json":
		c.Set("Content-Disposition", "attachment; filename=analytics-"+time.Now().Format("2006-01-02")+".json")
		return c.JSON(snapshot)
	default:
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "unsupported format",
		})
	}
}

// exportAsCSV exports analytics data as CSV
func (h *AnalyticsHandler) exportAsCSV(c *fiber.Ctx, snapshot *services.DashboardSnapshot) error {
	filename := "analytics-" + time.Now().Format("2006-01-02") + ".csv"
	c.Set("Content-Disposition", "attachment; filename="+filename)
	c.Set("Content-Type", "text/csv")

	csv := "OpenRisk Analytics Export\n"
	csv += "Timestamp," + snapshot.Timestamp.Format(time.RFC3339) + "\n\n"

	// Risk Metrics Section
	csv += "Risk Metrics\n"
	csv += "Metric,Value\n"
	csv += "Total Risks," + strconv.FormatInt(snapshot.RiskMetrics.TotalRisks, 10) + "\n"
	csv += "Active Risks," + strconv.FormatInt(snapshot.RiskMetrics.ActiveRisks, 10) + "\n"
	csv += "Mitigated Risks," + strconv.FormatInt(snapshot.RiskMetrics.MitigatedRisks, 10) + "\n"
	csv += "Average Score," + strconv.FormatFloat(snapshot.RiskMetrics.AverageScore, 'f', 2, 64) + "\n"
	csv += "High Risks," + strconv.FormatInt(snapshot.RiskMetrics.HighRisks, 10) + "\n"
	csv += "Medium Risks," + strconv.FormatInt(snapshot.RiskMetrics.MediumRisks, 10) + "\n"
	csv += "Low Risks," + strconv.FormatInt(snapshot.RiskMetrics.LowRisks, 10) + "\n\n"

	// Mitigation Metrics Section
	csv += "Mitigation Metrics\n"
	csv += "Metric,Value\n"
	csv += "Total Mitigations," + strconv.FormatInt(snapshot.MitigationMetrics.TotalMitigations, 10) + "\n"
	csv += "Completed," + strconv.FormatInt(snapshot.MitigationMetrics.CompletedMitigations, 10) + "\n"
	csv += "Pending," + strconv.FormatInt(snapshot.MitigationMetrics.PendingMitigations, 10) + "\n"
	csv += "Completion Rate," + strconv.FormatFloat(snapshot.MitigationMetrics.CompletionRate, 'f', 2, 64) + "%\n\n"

	// Framework Analytics Section
	csv += "Framework Compliance\n"
	csv += "Framework,Associated Risks,Average Score\n"
	for _, fw := range snapshot.FrameworkAnalytics {
		csv += fw.Framework + "," +
			strconv.FormatInt(fw.AssociatedRisks, 10) + "," +
			strconv.FormatFloat(fw.AverageRiskScore, 'f', 2, 64) + "\n"
	}

	return c.SendString(csv)
}

// RegisterAnalyticsRoutes registers all analytics routes
func RegisterAnalyticsRoutes(app *fiber.App, handler *AnalyticsHandler) {
	// Create protected group
	protected := app.Group("/api/v1/analytics")
	protected.Use(middleware.Protected())

	// Analytics endpoints
	protected.Get("/risks/metrics", handler.GetRiskMetrics)
	protected.Get("/risks/trends", handler.GetRiskTrends)
	protected.Get("/mitigations/metrics", handler.GetMitigationMetrics)
	protected.Get("/frameworks", handler.GetFrameworkAnalytics)
	protected.Get("/dashboard", handler.GetDashboardSnapshot)
	protected.Get("/export", handler.GetExportData)
}
