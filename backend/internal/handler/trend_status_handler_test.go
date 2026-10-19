// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

// The two reachable risk-trend endpoints must answer "insufficient_data" on a
// tenant without history, never a series of 0.0 averages (#756).

func trendDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE risks (
		id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, status TEXT, score REAL DEFAULT 0,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	return db
}

func trendApp(db *gorm.DB, tenant uuid.UUID, signedIn bool) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if signedIn {
			c.Locals("userID", uuid.NewString())
			middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		}
		return c.Next()
	})
	app.Get("/analytics/risks/trends", NewAnalyticsHandler(service.NewAnalyticsService(db)).GetRiskTrends)
	app.Get("/dashboard/risk-trends", NewEnhancedDashboardHandler(service.NewDashboardDataService(db, nil)).GetRiskTrends)
	return app
}

type trendBody struct {
	Status    string            `json:"status"`
	MinPoints int               `json:"min_points"`
	Trends    []json.RawMessage `json:"trends"`
}

func getTrend(t *testing.T, app *fiber.App, path string) (int, trendBody, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body trendBody
	if resp.StatusCode == fiber.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &body))
	}
	return resp.StatusCode, body, string(raw)
}

var trendPaths = []string{"/analytics/risks/trends?days=30", "/dashboard/risk-trends"}

func TestRiskTrendEndpoints_EmptyTenant_InsufficientData(t *testing.T) {
	app := trendApp(trendDB(t), uuid.New(), true)
	for _, path := range trendPaths {
		code, body, raw := getTrend(t, app, path)
		require.Equal(t, fiber.StatusOK, code, path)
		require.Equal(t, service.TrendStatusInsufficientData, body.Status, path)
		require.Equal(t, service.MinTrendPoints, body.MinPoints, path)
		require.Empty(t, body.Trends, "%s must not draw a series without history", path)
		require.NotContains(t, raw, `"avg_score":0`, path)
		require.NotContains(t, raw, `"score":0`, path)
	}
}

func TestRiskTrendEndpoints_History_Success(t *testing.T) {
	db := trendDB(t)
	tenant := uuid.New()
	for i, score := range []float64{6, 9, 3} {
		at := time.Now().AddDate(0, 0, -(3 - i))
		require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, status, score, created_at, updated_at)
			VALUES (?,?,?,?,?,?)`, uuid.NewString(), tenant.String(), "open", score, at, at).Error)
	}
	app := trendApp(db, tenant, true)
	for _, path := range trendPaths {
		code, body, _ := getTrend(t, app, path)
		require.Equal(t, fiber.StatusOK, code, path)
		require.Equal(t, service.TrendStatusOK, body.Status, path)
		require.NotEmpty(t, body.Trends, path)
	}
}

func TestRiskTrendEndpoints_Unauthorized(t *testing.T) {
	app := trendApp(trendDB(t), uuid.New(), false)
	for _, path := range trendPaths {
		code, _, _ := getTrend(t, app, path)
		require.Equal(t, fiber.StatusUnauthorized, code, path)
	}
}
