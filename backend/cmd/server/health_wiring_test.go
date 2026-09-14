// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #647: the health endpoint answered 200 with "db": "CONNECTED" whatever the
// database was doing, so no probe built on it could ever report unhealthy.
//
// The endpoint is public and takes no identifier, so the charter's NotFound and
// Unauthorized cases have no meaning here; the failure case that matters is the
// database being unreachable.

func getHealth(t *testing.T, ping healthPinger) (int, map[string]any) {
	t.Helper()
	app := fiber.New()
	app.Get("/health", healthHandler(ping, func() bool { return false }))

	resp, err := app.Test(httptest.NewRequest("GET", "/health", nil))
	require.NoError(t, err)
	defer resp.Body.Close()

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body
}

func TestHealthHandler_Success(t *testing.T) {
	code, body := getHealth(t, func(context.Context) error { return nil })

	assert.Equal(t, 200, code)
	assert.Equal(t, "UP", body["status"])
	assert.Equal(t, "CONNECTED", body["db"])
	assert.Equal(t, false, body["demo_mode"])
}

func TestHealthHandler_DatabaseDown(t *testing.T) {
	code, body := getHealth(t, func(context.Context) error {
		return errors.New("dial tcp db:5432: connect: connection refused")
	})

	assert.Equal(t, 503, code, "a probe on this endpoint must be able to fail")
	assert.Equal(t, "DOWN", body["status"])
	assert.Equal(t, "DISCONNECTED", body["db"])
}

func TestHealthHandler_PingIsBounded(t *testing.T) {
	var deadlineSet bool
	getHealth(t, func(ctx context.Context) error {
		_, deadlineSet = ctx.Deadline()
		return nil
	})

	assert.True(t, deadlineSet, "a hung database must not hang the probe")
}
