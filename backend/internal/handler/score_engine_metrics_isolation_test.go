// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// scoringMetricsApp serves GetScoringMetrics with tenantID in the session
// locals, the way the Protected middleware leaves it.
func scoringMetricsApp(t *testing.T, db *gorm.DB, tenantID *uuid.UUID) *fiber.App {
	t.Helper()
	h := NewScoreEngineHandler(db, service.NewScoreEngineService(db))
	app := fiber.New()
	app.Get("/metrics", func(c *fiber.Ctx) error {
		if tenantID != nil {
			c.Locals("tenant_id", *tenantID)
		}
		return h.GetScoringMetrics(c)
	})
	return app
}

func scoringMetricsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE risks (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, score REAL, deleted_at DATETIME);`).Error)
	return db
}

// TestGetScoringMetrics_Success: the figures describe the caller's own risks.
func TestGetScoringMetrics_Success(t *testing.T) {
	db := scoringMetricsDB(t)
	a, b := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, score) VALUES (?,?,?)`, uuid.NewString(), a.String(), 2.0).Error)
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, score) VALUES (?,?,?)`, uuid.NewString(), b.String(), 9.0).Error)

	resp, err := scoringMetricsApp(t, db, &a).Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var out struct {
		Avg float64 `json:"avg_score"`
		Max float64 `json:"max_score"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, 2.0, out.Avg)
	require.Equal(t, 2.0, out.Max)
}

// TestGetScoringMetrics_CrossTenant: tenant B's 9.0 risk does not move tenant A's
// maximum, and a tenant with no risks sees zero, not the deployment's figures.
func TestGetScoringMetrics_CrossTenant(t *testing.T) {
	db := scoringMetricsDB(t)
	a, b := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, score) VALUES (?,?,?)`, uuid.NewString(), b.String(), 9.0).Error)

	resp, err := scoringMetricsApp(t, db, &a).Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var out struct {
		Max float64 `json:"max_score"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, 0.0, out.Max, "another tenant's risk leaked into the maximum")
}

// TestGetScoringMetrics_Unauthorized: no tenant in the session is a 401, never an
// all-tenants answer.
func TestGetScoringMetrics_Unauthorized(t *testing.T) {
	db := scoringMetricsDB(t)
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, score) VALUES (?,?,?)`, uuid.NewString(), uuid.NewString(), 9.0).Error)

	resp, err := scoringMetricsApp(t, db, nil).Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	require.Equal(t, 401, resp.StatusCode)
}
