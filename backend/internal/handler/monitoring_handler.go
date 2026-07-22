// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
