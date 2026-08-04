// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/middleware"
)

// serverError returns a 500 without disclosing the underlying error.
//
// Audit finding F-02 was fixed in the global Fiber error handler, but that only
// catches errors a handler *returns*. Handlers that build a 500 response
// themselves — `c.Status(500).JSON(fiber.Map{"error": ..., "details": err.Error()})`
// — never reach it, so the raw error still went out over the wire. With GORM in
// the stack that string routinely carries the SQL statement, table and column
// names, and fragments of row values.
//
// This restores the same contract as the global handler: the full error is
// logged server-side against a correlation id, and the client gets a stable
// message plus that id. Outside production the detail is returned inline, since
// hiding it locally makes debugging worse for no security gain.
//
// Use it for genuine server faults. Failures that map to a typed domain error
// (not-found, forbidden, conflict, validation) should go through writeAppError
// instead, which preserves their status code.
func serverError(c *fiber.Ctx, publicMessage string, err error) error {
	if !middleware.IsProductionEnv() {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   publicMessage,
			"details": err.Error(),
		})
	}

	reference := uuid.NewString()
	log.Printf("[error][ref=%s] %s %s: %v", reference, c.Method(), c.Path(), err)

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error":     publicMessage,
		"reference": reference,
	})
}
