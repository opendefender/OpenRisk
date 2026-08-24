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
// A malformed value is an error, NEVER a silent fall back to the default window.
// Answering ?period=6m with the all-time numbers, under a control still showing
// "6 months", is the exact class of quiet substitution this wave exists to
// remove: the user cannot tell that the filter was dropped, and the numbers look
// perfectly reasonable.
//
// IT RETURNS AN ERROR, and the caller renders it. The first version of this
// helper returned the *response* instead — `c.Status(400).JSON(...)` — and that
// was wrong in a way worth recording, because it produced exactly the failure
// this wave is about. Fiber's JSON() returns nil on success, so the caller's
// `if errResp != nil` never fired: the handler set a 400, then went on to compute
// the full aggregate against a ZERO window and wrote it over the error body. The
// response was a 400 carrying a complete, plausible dashboard for a period the
// server had just rejected. Found by driving a live server, not by any unit test,
// because both halves were correct in isolation and only the seam was not.
func parsePeriod(c *fiber.Ctx) (timeframe.Window, error) {
	return timeframe.Parse(c.Query("period"), c.Query("from"), c.Query("to"), time.Now().UTC())
}

// writePeriodError renders a rejected period, and nothing else.
//
// No partial payload, no counters computed "anyway": a request whose window did
// not parse has no numbers to report, and reporting some would invite a client to
// render them.
func writePeriodError(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error":   "invalid_period",
		"message": err.Error(),
	})
}
