// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/service"
)

// setupIncidentDB creates an in-memory SQLite DB with the incident tables via
// AutoMigrate — the same models cmd/server/main.go now migrates (M5).
func setupIncidentDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Incident{}, &domain.IncidentTimeline{}, &domain.IncidentAction{}))
	return db
}

// buildIncidentApp wires the real handler + service behind a middleware that
// stamps the tenant id local (as the auth middleware does with a UUID string).
// RBAC is exercised elsewhere; here we prove the handler/service/DB path — and
// specifically that the routes use :id (the handlers used to read :incidentId).
func buildIncidentApp(t *testing.T, db *gorm.DB, tenantID uuid.UUID) *fiber.App {
	t.Helper()
	h := NewIncidentHandler(service.NewIncidentService(db))

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", tenantID.String())
		c.Locals("user_id", uuid.New().String())
		return c.Next()
	})
	g := app.Group("/incidents")
	g.Post("", h.CreateIncident)
	g.Get("/stats", h.GetIncidentStats)
	g.Get("", h.ListIncidents)
	g.Get("/:id", h.GetIncident)
	g.Put("/:id", h.UpdateIncident)
	g.Delete("/:id", h.DeleteIncident)
	g.Get("/:id/timeline", h.GetIncidentTimeline)
	g.Post("/:id/risks/:riskId", h.LinkRisk)
	g.Post("/:id/actions", h.CreateIncidentAction)
	g.Get("/:id/actions", h.GetIncidentActions)
	g.Put("/:id/actions/:actionId", h.UpdateIncidentAction)
	return app
}

// TestIncidentE2EFlow walks the register end-to-end: create → stats → list →
// get-by-id → update status → timeline → delete. It is the regression guard for
// the three bugs that made every /incidents route fail before M5 (missing table,
// Preload("Risk") on a non-existent relation, and the :id/:incidentId mismatch).
func TestIncidentE2EFlow(t *testing.T) {
	db := setupIncidentDB(t)
	tenantID := uuid.New()
	app := buildIncidentApp(t, db, tenantID)

	// 1. Create.
	req := httptest.NewRequest(http.MethodPost, "/incidents",
		mustJSON(t, map[string]any{
			"title": "Suspected exfiltration", "description": "Abnormal outbound traffic",
			"incident_type": "breach", "severity": "critical", "source": "internal", "reported_by": "soc",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	var created domain.Incident
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()
	require.NotZero(t, created.ID)
	require.Equal(t, "open", created.Status)
	require.Equal(t, tenantID.String(), created.TenantID)

	idStr := itoa(created.ID)

	// 2. Stats reflect one open incident.
	stats := getJSON(t, app, "/incidents/stats")
	require.EqualValues(t, 1, stats["total_incidents"])
	require.EqualValues(t, 1, stats["open_incidents"])

	// 3. List returns it.
	list := getJSON(t, app, "/incidents")
	require.EqualValues(t, 1, list["total"])

	// 4. Get by id (proves the :id param fix + no Preload 500).
	req = httptest.NewRequest(http.MethodGet, "/incidents/"+idStr, nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	// 5. Update status to resolved.
	req = httptest.NewRequest(http.MethodPut, "/incidents/"+idStr,
		mustJSON(t, map[string]string{"status": "resolved", "resolution": "contained"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	stats = getJSON(t, app, "/incidents/stats")
	require.EqualValues(t, 1, stats["resolved_incidents"])

	// 6. Timeline has the creation + status-change events.
	req = httptest.NewRequest(http.MethodGet, "/incidents/"+idStr+"/timeline", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var timeline []domain.IncidentTimeline
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&timeline))
	resp.Body.Close()
	require.GreaterOrEqual(t, len(timeline), 2)

	// 7. Delete.
	req = httptest.NewRequest(http.MethodDelete, "/incidents/"+idStr, nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()

	list = getJSON(t, app, "/incidents")
	require.EqualValues(t, 0, list["total"])
}

// TestIncident_CrossTenant proves a tenant can neither see nor fetch another
// tenant's incident (CLAUDE.md rule #2 — tenant_id filtered on every query).
func TestIncident_CrossTenant(t *testing.T) {
	db := setupIncidentDB(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	appA := buildIncidentApp(t, db, tenantA)
	appB := buildIncidentApp(t, db, tenantB)

	req := httptest.NewRequest(http.MethodPost, "/incidents",
		mustJSON(t, map[string]any{
			"title": "A's incident", "description": "x", "incident_type": "attack",
			"severity": "high", "source": "internal", "reported_by": "a",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := appA.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	var created domain.Incident
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()

	// Tenant B sees nothing and cannot fetch it.
	list := getJSON(t, appB, "/incidents")
	require.EqualValues(t, 0, list["total"])

	req = httptest.NewRequest(http.MethodGet, "/incidents/"+itoa(created.ID), nil)
	resp, err = appB.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()
}

// TestIncident_CrossTenant_TimelineAndActions proves the sub-resource routes are
// tenant-scoped too. Incident IDs are sequential integers (trivially enumerable),
// so timeline/actions/link must verify the parent incident belongs to the caller
// before reading OR writing — otherwise tenant B reads/mutates tenant A's data by
// guessing an ID.
func TestIncident_CrossTenant_TimelineAndActions(t *testing.T) {
	db := setupIncidentDB(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	appA := buildIncidentApp(t, db, tenantA)
	appB := buildIncidentApp(t, db, tenantB)

	// Tenant A creates an incident + an action on it.
	req := httptest.NewRequest(http.MethodPost, "/incidents",
		mustJSON(t, map[string]any{
			"title": "A's incident", "description": "x", "incident_type": "attack",
			"severity": "high", "source": "internal", "reported_by": "a",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := appA.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	var created domain.Incident
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()
	id := itoa(created.ID)

	req = httptest.NewRequest(http.MethodPost, "/incidents/"+id+"/actions",
		mustJSON(t, map[string]any{"title": "Rotate keys", "assigned_to": "soc", "due_date": "2030-01-01T00:00:00Z"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = appA.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	var action domain.IncidentAction
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&action))
	resp.Body.Close()
	require.NotZero(t, action.ID)
	actionID := itoa(action.ID)

	// Tenant B is blocked from every sub-resource of A's incident (read + write).
	cases := []struct {
		name, method, path string
		body               map[string]any
	}{
		{"read timeline", http.MethodGet, "/incidents/" + id + "/timeline", nil},
		{"read actions", http.MethodGet, "/incidents/" + id + "/actions", nil},
		{"create action", http.MethodPost, "/incidents/" + id + "/actions", map[string]any{"title": "evil", "assigned_to": "b", "due_date": "2030-01-01T00:00:00Z"}},
		{"update action", http.MethodPut, "/incidents/" + id + "/actions/" + actionID, map[string]any{"status": "completed"}},
		{"link risk", http.MethodPost, "/incidents/" + id + "/risks/1", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r *http.Request
			if tc.body != nil {
				r = httptest.NewRequest(tc.method, tc.path, mustJSON(t, tc.body))
				r.Header.Set("Content-Type", "application/json")
			} else {
				r = httptest.NewRequest(tc.method, tc.path, nil)
			}
			rp, err := appB.Test(r)
			require.NoError(t, err)
			require.Equal(t, 404, rp.StatusCode, "tenant B must not reach tenant A's incident sub-resource")
			rp.Body.Close()
		})
	}

	// Sanity: the action A created was NOT mutated by B's forbidden update.
	var fresh domain.IncidentAction
	require.NoError(t, db.First(&fresh, action.ID).Error)
	require.Equal(t, "pending", fresh.Status)

	// And no rogue action leaked onto A's incident from B's create attempt.
	var actionCount int64
	require.NoError(t, db.Model(&domain.IncidentAction{}).Where("incident_id = ?", created.ID).Count(&actionCount).Error)
	require.EqualValues(t, 1, actionCount)
}

// TestUpdateIncident_InvalidStatus rejects an out-of-vocabulary status.
func TestUpdateIncident_InvalidStatus(t *testing.T) {
	db := setupIncidentDB(t)
	tenantID := uuid.New()
	app := buildIncidentApp(t, db, tenantID)

	req := httptest.NewRequest(http.MethodPost, "/incidents",
		mustJSON(t, map[string]any{
			"title": "x", "description": "y", "incident_type": "breach",
			"severity": "low", "source": "internal", "reported_by": "z",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	var created domain.Incident
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodPut, "/incidents/"+itoa(created.ID),
		mustJSON(t, map[string]string{"status": "on_fire"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode, "an invalid status is rejected by the service")
	resp.Body.Close()
}

// --- small helpers (kept local to the incident tests) ---

func itoa(u uint) string {
	return strconv.FormatUint(uint64(u), 10)
}

func getJSON(t *testing.T, app *fiber.App, path string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	resp.Body.Close()
	return out
}
