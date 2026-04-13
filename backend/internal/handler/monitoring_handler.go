package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/middleware"
)

// MetricsHandler provides HTTP handlers for metrics endpoints
type MetricsHandler struct {
	collector *middleware.MetricsCollector
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(collector *middleware.MetricsCollector) *MetricsHandler {
	return &MetricsHandler{
		collector: collector,
	}
}

// RegisterRoutes registers metrics routes on a Fiber router
func (mh *MetricsHandler) RegisterRoutes(app *fiber.App) {
	// Lightweight metrics endpoint compatible with Fiber context.
	app.Get("/metrics", func(c *fiber.Ctx) error {
		if mh.collector == nil {
			return c.JSON(fiber.Map{"status": "ok"})
		}
		return c.JSON(mh.collector.GetStats())
	})
}

// GetCollector returns the metrics collector
func (mh *MetricsHandler) GetCollector() *middleware.MetricsCollector {
	return mh.collector
}
