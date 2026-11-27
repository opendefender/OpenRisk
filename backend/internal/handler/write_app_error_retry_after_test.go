// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	invapp "github.com/opendefender/openrisk/internal/application/inventory"
	"github.com/opendefender/openrisk/internal/domain"
)

func retryAfterOf(t *testing.T, h fiber.Handler) (int, string) {
	t.Helper()
	app := fiber.New()
	app.Get("/", h)
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Retry-After")
}

// #1318: a cooldown error says the cooldown, not one second.
func TestWriteAppError_CooldownsAnnounceTheirWait(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"inventory scan":  {invapp.ErrScanCooldown, "600"},
		"inventory push":  {invapp.ErrPushCooldown, "60"},
		"no delay given":  {domain.NewRateLimitError("slow down", 0), "1"},
		"partial seconds": {domain.NewRateLimitError("slow down", 1500*time.Millisecond), "2"},
	} {
		status, got := retryAfterOf(t, func(c *fiber.Ctx) error { return writeAppError(c, tc.err) })
		assert.Equal(t, 429, status, name)
		assert.Equal(t, tc.want, got, name)
	}
}

// A Retry-After the handler set itself is kept.
func TestWriteAppError_KeepsAHandlerRetryAfter(t *testing.T) {
	_, got := retryAfterOf(t, func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderRetryAfter, "42")
		return writeAppError(c, domain.NewRateLimitError("slow down", 0))
	})
	assert.Equal(t, "42", got)
}

// Not a 429: no header.
func TestWriteAppError_NoRetryAfterOutsideA429(t *testing.T) {
	status, got := retryAfterOf(t, func(c *fiber.Ctx) error {
		return writeAppError(c, domain.NewNotFoundError("thing", "x"))
	})
	assert.Equal(t, 404, status)
	assert.Empty(t, got)
}
