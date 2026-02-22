package monitoring

import (
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
)

// MetricsMiddleware provides Fiber middleware for collecting metrics
type MetricsMiddleware struct {
	collector *MetricsCollector
	startTime time.Time
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(collector *MetricsCollector) *MetricsMiddleware {
	return &MetricsMiddleware{
		collector: collector,
		startTime: time.Now(),
	}
}

// Handler returns the Fiber middleware handler
func (mm *MetricsMiddleware) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Mark request as in-flight
		mm.collector.HTTPRequestsInFlight.Inc()
		defer mm.collector.HTTPRequestsInFlight.Dec()

		// Record start time
		start := time.Now()

		// Continue to next handler
		err := c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := c.Response().StatusCode()
		success := statusCode >= 200 && statusCode < 400

		mm.collector.RecordAPIRequest(duration, success)

		return err
	}
}

// StartSystemMetricsCollector starts a goroutine to collect system metrics
func (mm *MetricsMiddleware) StartSystemMetricsCollector(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			// Collect memory metrics
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			mm.collector.UpdateGoMemoryHeap(float64(m.HeapAlloc))
			mm.collector.UpdateGoMemoryAlloc(float64(m.Alloc))
			mm.collector.UpdateGoGoroutines(float64(runtime.NumGoroutine()))

			// Update system uptime
			uptime := time.Since(mm.startTime).Seconds()
			mm.collector.UpdateSystemUptime(uptime)
		}
	}()
}
