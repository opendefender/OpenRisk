// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The regression this file exists for.
//
// parsePeriod originally returned the *response* — `c.Status(400).JSON(...)` —
// and Fiber's JSON() returns nil on success, so the caller's `if errResp != nil`
// never fired. The handler set a 400 and then computed the whole aggregate
// against a ZERO window and wrote it over the error body. The result was a 400
// carrying a complete, plausible dashboard for a period the server had just
// rejected: a status code saying no, and a payload a client would happily render.
//
// Neither unit test could see it. timeframe.Parse rejected the input correctly
// and ComputeDashboardStats aggregated correctly; only the seam between them was
// wrong, and only driving a live server showed it. So the seam is tested here,
// through a real fiber app, on the response BODY and not just its status.
func TestParsePeriod_RejectionCarriesNoPayload(t *testing.T) {
	app := fiber.New()
	app.Get("/stats", func(c *fiber.Ctx) error {
		window, err := parsePeriod(c)
		if err != nil {
			return writePeriodError(c, err)
		}
		// Stand in for the aggregate. If the guard above ever stops short-
		// circuiting, this leaks into the response and the test fails.
		return c.JSON(fiber.Map{"total_risks": 8, "preset": string(window.Preset)})
	})

	bad := []struct{ name, qs string }{
		{"unknown preset", "?period=6m"},
		{"prose", "?period=yesterday"},
		{"custom is not a value", "?period=custom"},
		{"unparseable from", "?from=not-a-date&to=2026-09-01"},
		{"non-ISO to", "?from=2026-08-01&to=31/12/2026"},
		{"inverted range", "?from=2026-09-01&to=2026-08-01"},
		{"empty range", "?from=2026-08-01&to=2026-08-01"},
		{"to without from", "?to=2026-09-01"},
		{"excessive range", "?from=2020-01-01&to=2026-01-01"},
		{"preset and bounds together", "?period=30d&from=2026-08-01&to=2026-09-01"},
	}

	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/stats"+tc.qs, nil))
			require.NoError(t, err)
			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))

			// The rejection says what was wrong...
			assert.Equal(t, "invalid_period", body["error"])
			assert.NotEmpty(t, body["message"], "a rejected period must say why")
			// ...and carries NO numbers. This is the assertion that fails if the
			// short-circuit regresses.
			assert.NotContains(t, body, "total_risks",
				"a rejected period must not return a payload a client could render")
			assert.NotContains(t, string(raw), "total_risks")
		})
	}
}

func TestParsePeriod_AcceptsWhatTheProductOffers(t *testing.T) {
	app := fiber.New()
	app.Get("/stats", func(c *fiber.Ctx) error {
		window, err := parsePeriod(c)
		if err != nil {
			return writePeriodError(c, err)
		}
		return c.JSON(fiber.Map{"preset": string(window.Preset)})
	})

	good := map[string]string{
		"":                               "all", // no parameters at all
		"?period=all":                    "all",
		"?period=7d":                     "7d",
		"?period=30d":                    "30d",
		"?period=90d":                    "90d",
		"?from=2026-08-01&to=2026-09-01": "custom",
		"?from=2026-08-01T00:00:00Z&to=2026-09-01T00:00:00Z": "custom",
		// Exactly at the cap, which must be allowed rather than off-by-one'd out.
		"?from=2025-08-24&to=2026-08-25": "custom",
	}
	for qs, wantPreset := range good {
		t.Run(qs, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/stats"+qs, nil))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			var body map[string]string
			raw, _ := io.ReadAll(resp.Body)
			require.NoError(t, json.Unmarshal(raw, &body))
			assert.Equal(t, wantPreset, body["preset"])
		})
	}
}
