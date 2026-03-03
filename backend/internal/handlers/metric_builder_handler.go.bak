package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/models"
	"github.com/opendefender/openrisk/internal/services"
)

// MetricBuilderHandler handles custom metric endpoints
type MetricBuilderHandler struct {
	metricService *services.MetricBuilderService
}

// NewMetricBuilderHandler creates a new metric builder handler
func NewMetricBuilderHandler(metricService *services.MetricBuilderService) *MetricBuilderHandler {
	return &MetricBuilderHandler{
		metricService: metricService,
	}
}

// CreateCustomMetric creates a new custom metric
// POST /metrics/custom
func (h *MetricBuilderHandler) CreateCustomMetric(c *fiber.Ctx) error {
	var def models.MetricDefinition
	if err := c.BodyParser(&def); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tenantID := c.Locals("tenant_id").(string)
	userID := c.Locals("user_id").(string)

	metric, err := h.metricService.CreateCustomMetric(tenantID, def, userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create metric: %v", err),
		})
	}

	return c.Status(http.StatusCreated).JSON(metric)
}

// GetCustomMetric retrieves a custom metric by ID
// GET /metrics/custom/:metricId
func (h *MetricBuilderHandler) GetCustomMetric(c *fiber.Ctx) error {
	metricID, err := strconv.ParseUint(c.Params("metricId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid metric ID",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	metric, err := h.metricService.GetCustomMetric(tenantID, uint(metricID))
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Metric not found",
		})
	}

	return c.JSON(metric)
}

// ListCustomMetrics lists all custom metrics for tenant
// GET /metrics/custom
func (h *MetricBuilderHandler) ListCustomMetrics(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	metrics, err := h.metricService.ListCustomMetrics(tenantID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to list metrics: %v", err),
		})
	}

	return c.JSON(metrics)
}

// UpdateCustomMetric updates a custom metric
// PUT /metrics/custom/:metricId
func (h *MetricBuilderHandler) UpdateCustomMetric(c *fiber.Ctx) error {
	metricID, err := strconv.ParseUint(c.Params("metricId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid metric ID",
		})
	}

	var def models.MetricDefinition
	if err := c.BodyParser(&def); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	metric, err := h.metricService.UpdateCustomMetric(tenantID, uint(metricID), def)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to update metric: %v", err),
		})
	}

	return c.JSON(metric)
}

// DeleteCustomMetric deletes a custom metric
// DELETE /metrics/custom/:metricId
func (h *MetricBuilderHandler) DeleteCustomMetric(c *fiber.Ctx) error {
	metricID, err := strconv.ParseUint(c.Params("metricId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid metric ID",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	if err := h.metricService.DeleteCustomMetric(tenantID, uint(metricID)); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to delete metric: %v", err),
		})
	}

	return c.Status(http.StatusNoContent).Send(nil)
}

// GetCalculatedMetric gets current metric value with trend
// GET /metrics/custom/:metricId/calculated
func (h *MetricBuilderHandler) GetCalculatedMetric(c *fiber.Ctx) error {
	metricID, err := strconv.ParseUint(c.Params("metricId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid metric ID",
		})
	}

	tenantID := c.Locals("tenant_id").(string)

	calculated, err := h.metricService.GetCalculatedMetric(tenantID, uint(metricID))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to calculate metric: %v", err),
		})
	}

	return c.JSON(calculated)
}

// GetMetricHistory retrieves historical values
// GET /metrics/custom/:metricId/history?days=30
func (h *MetricBuilderHandler) GetMetricHistory(c *fiber.Ctx) error {
	metricID, err := strconv.ParseUint(c.Params("metricId"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid metric ID",
		})
	}

	days := c.QueryInt("days", 30)
	tenantID := c.Locals("tenant_id").(string)

	history, err := h.metricService.GetMetricHistory(tenantID, uint(metricID), days)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get history: %v", err),
		})
	}

	return c.JSON(history)
}

// CompareMetrics compares multiple metrics
// POST /metrics/compare
func (h *MetricBuilderHandler) CompareMetrics(c *fiber.Ctx) error {
	var req struct {
		MetricIDs []uint `json:"metric_ids" binding:"required"`
		Days      int    `json:"days"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Days == 0 {
		req.Days = 30
	}

	tenantID := c.Locals("tenant_id").(string)

	comparison, err := h.metricService.CompareMetrics(tenantID, req.MetricIDs, req.Days)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to compare metrics: %v", err),
		})
	}

	return c.JSON(comparison)
}

// ExportMetricsSnapshot exports current metric snapshot
// GET /metrics/export/snapshot
func (h *MetricBuilderHandler) ExportMetricsSnapshot(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)

	snapshot, err := h.metricService.ExportMetricsSnapshot(tenantID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to export snapshot: %v", err),
		})
	}

	c.Set("Content-Disposition", "attachment; filename=metrics-snapshot.json")
	c.Set("Content-Type", "application/json")
	return c.JSON(snapshot)
}
