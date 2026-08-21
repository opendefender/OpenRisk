// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/domain/timeframe"
)

// parsePeriod reads the Command Center's period parameters off a request.
//
// Accepted, and identical on every endpoint that supports a period:
//
//	?period=all|7d|30d|90d            a named window
//	?from=<date>&to=<date>            an explicit half-open range [from, to)
//
// A malformed value is a 400 with the reason, NEVER a silent fall back to the
// default window. Answering ?period=6m with the all-time numbers, under a control
// still showing "6 months", is the exact class of quiet substitution this wave
// exists to remove: the user cannot tell that the filter was dropped, and the
// numbers look perfectly reasonable.
//
// The second return is the response to send; when it is non-nil the caller must
// return it unchanged and must not touch the window.
func parsePeriod(c *fiber.Ctx) (timeframe.Window, error) {
	w, err := timeframe.Parse(c.Query("period"), c.Query("from"), c.Query("to"), time.Now().UTC())
	if err != nil {
		return timeframe.Window{}, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid_period",
			"message": err.Error(),
		})
	}
	return w, nil
}
