// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #783: the caps a use case applies — active rules (rule create, update,
// enable), scans per month (scan trigger), seats (reactivation, resend) —
// reach the client through writeAppError. Every criterion defines "refused" as
// 402 with code, limit_key, limit and used, the body RequireCapacity sends.
func TestWriteAppError_LimitReachedBody_Unauthorized(t *testing.T) {
	for _, key := range []string{"automation_rules", "scans_per_month", "users"} {
		app := fiber.New()
		app.Get("/x", func(c *fiber.Ctx) error {
			return writeAppError(c, domain.NewLimitReachedError(key, 2, 2))
		})
		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
		require.NoError(t, err)
		body := map[string]any{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		resp.Body.Close()

		assert.Equal(t, fiber.StatusPaymentRequired, resp.StatusCode, key)
		assert.Equal(t, "limit_reached", body["code"], key)
		assert.Equal(t, key, body["limit_key"])
		assert.EqualValues(t, 2, body["limit"], key)
		assert.EqualValues(t, 2, body["used"], key)
		assert.NotEmpty(t, body["message"], key)
	}
}

// TestWriteAppError_LimitReachedBody_Success: a wrapped cap error keeps its body.
func TestWriteAppError_LimitReachedBody_Success(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return writeAppError(c, errors.Join(errors.New("enable rule"), domain.NewLimitReachedError("automation_rules", 20, 20)))
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body := map[string]any{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, fiber.StatusPaymentRequired, resp.StatusCode)
	assert.EqualValues(t, 20, body["limit"])
}

// TestWriteAppError_LimitReachedBody_NotFound: any other typed error keeps its
// own status — a missing rule is a 404, not a cap.
func TestWriteAppError_LimitReachedBody_NotFound(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return writeAppError(c, domain.NewNotFoundError("automation rule", uuid.New()))
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}
