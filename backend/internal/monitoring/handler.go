package monitoring

import (
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler provides HTTP handlers for metrics endpoints
type MetricsHandler struct {
	collector *MetricsCollector
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(collector *MetricsCollector) *MetricsHandler {
	return &MetricsHandler{
		collector: collector,
	}
}

// RegisterRoutes registers metrics routes on a Fiber router
func (mh *MetricsHandler) RegisterRoutes(app *fiber.App) {
	// Prometheus metrics endpoint - GET /metrics
	// This endpoint is typically scraped by Prometheus
	http := promhttp.Handler()
	app.Get("/metrics", func(c *fiber.Ctx) error {
		http.ServeHTTP(c.Response().BodyWriter(), c.Request())
		return nil
	})
}

// GetCollector returns the metrics collector
func (mh *MetricsHandler) GetCollector() *MetricsCollector {
	return mh.collector
}
