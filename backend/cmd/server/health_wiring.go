// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// GET /api/v1/health is what every container probe, installer and CI wait-loop
// reads. It used to answer 200 with a hard-coded `"db": "CONNECTED"`, so a
// backend whose database was gone still reported healthy — a probe that could
// not fail (#647). It now pings the database and answers 503 when it cannot.

// healthPingTimeout stays well under the compose probe's 5s timeout, so a hung
// database reports 503 instead of the probe itself timing out.
const healthPingTimeout = 2 * time.Second

// healthPinger checks the database is reachable. A function rather than a
// *gorm.DB so the handler can be driven in a test without a database.
type healthPinger func(ctx context.Context) error

func gormHealthPinger(db *gorm.DB) healthPinger {
	return func(ctx context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}
}

func healthHandler(ping healthPinger, demoMode func() bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), healthPingTimeout)
		defer cancel()

		status, dbState, code := "UP", "CONNECTED", fiber.StatusOK
		if err := ping(ctx); err != nil {
			status, dbState, code = "DOWN", "DISCONNECTED", fiber.StatusServiceUnavailable
		}

		return c.Status(code).JSON(fiber.Map{
			"status":  status,
			"version": Version,
			"commit":  Commit,
			"db":      dbState,
			// Drives the permanent "demonstration data" banner. Served from the
			// backend rather than a frontend build flag so the two cannot disagree
			// about whether the data on screen is real.
			"demo_mode": demoMode(),
		})
	}
}
