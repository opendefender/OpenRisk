// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// genericServerErrorMessage is what a client sees for any unhandled failure in
// production. It carries no detail on purpose.
const genericServerErrorMessage = "An internal error occurred. Please contact support with the reference below if the problem persists."

// IsProductionEnv reports whether the process is running in production, which is
// what decides if internal error detail may reach clients.
//
// Anything that is not explicitly a development or test environment is treated
// as production. Failing closed matters here: a deployment that forgets to set
// APP_ENV must get the safe behaviour, not the leaky one.
func IsProductionEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "development", "dev", "test", "testing", "local":
		return false
	default:
		return true
	}
}

// ErrorHandler builds the Fiber error handler.
//
// Audit finding F-02: the previous handler returned err.Error() verbatim to the
// client, in production as well as development. With GORM in the stack that
// string routinely carries the SQL statement, table and column names, and
// fragments of row values - handing an attacker a free map of the schema.
//
// In production the client now receives a fixed message plus a correlation ID;
// the full error is written to the server log against that same ID, so support
// can still trace an incident from a user-reported reference without the detail
// ever crossing the wire. Outside production the detail is returned inline,
// because losing it would make local debugging materially worse.
//
// fiber.Error is treated differently on purpose: those are the framework's own
// deliberate, client-facing statuses (404 on an unknown route, 405, 413...).
// Their messages are chosen for clients and contain no internals, so they pass
// through unchanged and keep their status code.
func ErrorHandler(production bool) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		if fiberErr, ok := err.(*fiber.Error); ok {
			return c.Status(fiberErr.Code).JSON(fiber.Map{
				"error": true,
				"msg":   fiberErr.Message,
			})
		}

		if !production {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": true,
				"msg":   err.Error(),
			})
		}

		reference := uuid.NewString()
		// Server-side only. This is the sole place the raw error survives.
		log.Printf("[error][ref=%s] %s %s: %v", reference, c.Method(), c.Path(), err)

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":     true,
			"msg":       genericServerErrorMessage,
			"reference": reference,
		})
	}
}
