// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	applicationrisk "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/crq"
)

// The live two-tenant probe called for by audit finding F-07.
//
// The coverage gate in internal/security/isolation asserts that a decision was
// recorded for every parameterised route; it deliberately does not assert that
// isolation works. This does: it stands up the real handler chain over a real
// repository, gives two tenants their own data, and drives HTTP requests from
// tenant A at tenant B's resources.
//
// Every probe asserts the response is NOT 200 and that B's data is untouched.
// A 404 is the preferred answer over a 403, because 403 confirms the resource
// exists and turns the endpoint into an existence oracle.

// tenantRiskApp builds the risk surface with a switchable acting tenant, so one
// app instance can serve requests as either tenant without rebuilding state.
type tenantRiskApp struct {
	app    *fiber.App
	db     *gorm.DB
	acting *uuid.UUID
}

func newTenantRiskApp(t *testing.T) *tenantRiskApp {
	t.Helper()

	dsn := "file:risk_isolation_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&UserT{}, &MitigationT{}, &AssetT{}, &RiskHistoryT{}))
	require.NoError(t, db.Exec(risksTableDDL).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS risk_assets (risk_id TEXT NOT NULL, asset_id TEXT NOT NULL);`).Error)

	orig := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = orig })

	h := &tenantRiskApp{db: db, acting: new(uuid.UUID)}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{
			UserID:         uuid.New(),
			OrganizationID: *h.acting,
		})
		return c.Next()
	})

	riskRepo := repository.NewGormRiskRepository(db)
	handler := NewRiskHandler(
		applicationrisk.NewCreateRiskUseCase(riskRepo),
		applicationrisk.NewGetRiskUseCase(riskRepo),
		applicationrisk.NewListRisksUseCase(riskRepo),
		applicationrisk.NewUpdateRiskUseCase(riskRepo),
		applicationrisk.NewDeleteRiskUseCase(riskRepo),
		applicationrisk.NewMarkRiskReviewedUseCase(riskRepo),
		applicationrisk.NewTransitionPhaseUseCase(riskRepo),
		nil,
		crq.NewQuantifier(0, crq.Reference{}),
	)

	api := app.Group("/api/v1")
	api.Post("/risks", handler.CreateRisk)
	api.Get("/risks", handler.GetRisks)
	api.Get("/risks/:id", handler.GetRisk)
	api.Patch("/risks/:id", handler.UpdateRisk)
	api.Post("/risks/:id/transition", handler.TransitionPhase)
	api.Post("/risks/:id/review", handler.MarkReviewed)
	api.Delete("/risks/:id", handler.DeleteRisk)

	h.app = app
	return h
}

// as sets the tenant every subsequent request is made on behalf of.
func (h *tenantRiskApp) as(tenant uuid.UUID) { *h.acting = tenant }

func (h *tenantRiskApp) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return resp.StatusCode, decoded
}

// createRisk makes a risk owned by the acting tenant and returns its id.
func (h *tenantRiskApp) createRisk(t *testing.T, title string) string {
	t.Helper()

	status, body := h.do(t, http.MethodPost, "/api/v1/risks", map[string]any{
		"title":       title,
		"description": "probe fixture",
		"impact":      5,
		"probability": 0.4,
	})
	require.Equal(t, fiber.StatusCreated, status, "fixture creation must succeed: %v", body)

	id, _ := body["id"].(string)
	require.NotEmpty(t, id, "created risk must return an id: %v", body)
	return id
}

// TestRiskIsolation_CrossTenantAccessIsRefused drives every id-addressed risk
// route from tenant A against tenant B's risk.
func TestRiskIsolation_CrossTenantAccessIsRefused(t *testing.T) {
	h := newTenantRiskApp(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	h.as(tenantB)
	victimID := h.createRisk(t, "Tenant B confidential risk")

	h.as(tenantA)
	probes := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"read", http.MethodGet, "/api/v1/risks/" + victimID, nil},
		{"update", http.MethodPatch, "/api/v1/risks/" + victimID, map[string]any{"title": "pwned"}},
		{"transition phase", http.MethodPost, "/api/v1/risks/" + victimID + "/transition", map[string]any{"phase": "analyzed"}},
		{"mark reviewed", http.MethodPost, "/api/v1/risks/" + victimID + "/review", map[string]any{}},
		{"delete", http.MethodDelete, "/api/v1/risks/" + victimID, nil},
	}

	for _, p := range probes {
		t.Run(p.name, func(t *testing.T) {
			status, body := h.do(t, p.method, p.path, p.body)
			require.NotEqual(t, fiber.StatusOK, status,
				"tenant A must not %s tenant B's risk (body: %v)", p.name, body)
			require.NotEqual(t, fiber.StatusNoContent, status,
				"tenant A must not %s tenant B's risk (body: %v)", p.name, body)
			require.Equal(t, fiber.StatusNotFound, status,
				"cross-tenant access should answer 404, not %d — a 403 confirms the resource exists", status)
		})
	}

	// The victim must survive every probe intact, including the delete attempt.
	h.as(tenantB)
	status, body := h.do(t, http.MethodGet, "/api/v1/risks/"+victimID, nil)
	require.Equal(t, fiber.StatusOK, status, "tenant B's own risk must still be readable")
	require.Equal(t, "Tenant B confidential risk", body["title"], "title was mutated across tenants")
}

// TestRiskIsolation_ListLeaksNothing covers the subtler failure: not exposing a
// resource directly, but revealing it through a collection or a count.
func TestRiskIsolation_ListLeaksNothing(t *testing.T) {
	h := newTenantRiskApp(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	h.as(tenantB)
	for i := 0; i < 3; i++ {
		h.createRisk(t, fmt.Sprintf("B secret %d", i))
	}

	h.as(tenantA)
	ownID := h.createRisk(t, "A own risk")

	status, body := h.do(t, http.MethodGet, "/api/v1/risks", nil)
	require.Equal(t, fiber.StatusOK, status)

	items, _ := body["items"].([]any)
	require.NotNil(t, items, "list response shape not recognised: %v", body)
	require.Len(t, items, 1, "tenant A must see only its own risk, got %v", body)

	first, _ := items[0].(map[string]any)
	require.Equal(t, ownID, first["id"])

	// A count that includes other tenants is a leak even when the rows are hidden.
	if total, ok := body["total"].(float64); ok {
		require.Equal(t, float64(1), total, "the total count leaked tenant B's rows")
	}
}

// TestRiskIsolation_ForgedAndMalformedIDs checks the endpoints fail closed on
// ids that do not resolve, rather than falling back to an unscoped lookup.
func TestRiskIsolation_ForgedAndMalformedIDs(t *testing.T) {
	h := newTenantRiskApp(t)
	h.as(uuid.New())

	for _, id := range []string{uuid.NewString(), "not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		t.Run(id, func(t *testing.T) {
			status, _ := h.do(t, http.MethodGet, "/api/v1/risks/"+id, nil)
			require.NotEqual(t, fiber.StatusOK, status, "unknown id must not resolve")
		})
	}
}

// TestRiskIsolation_NilTenantIsDenied pins fail-closed behaviour when the tenant
// cannot be resolved. Falling back to an unscoped query here is precisely the
// bug found in the dashboard during the July sweep.
func TestRiskIsolation_NilTenantIsDenied(t *testing.T) {
	h := newTenantRiskApp(t)
	tenantB := uuid.New()

	h.as(tenantB)
	victimID := h.createRisk(t, "B risk")

	h.as(uuid.Nil)
	status, body := h.do(t, http.MethodGet, "/api/v1/risks/"+victimID, nil)
	require.NotEqual(t, fiber.StatusOK, status,
		"a request with no resolvable tenant must not read tenant data (body: %v)", body)

	status, body = h.do(t, http.MethodGet, "/api/v1/risks", nil)
	if status == fiber.StatusOK {
		items, _ := body["items"].([]any)
		require.Empty(t, items, "a nil tenant must never list another tenant's risks")
	}
}

// risksTableDDL mirrors domain.Risk. AutoMigrate cannot build it on sqlite
// (Postgres gen_random_uuid() default + array columns), and every column the
// repository INSERT/RETURNING touches must exist or inserts fail — the drift
// that once broke TestRiskCRUDFlow.
const risksTableDDL = `CREATE TABLE IF NOT EXISTS risks (
	id TEXT PRIMARY KEY,
	tenant_id TEXT,
	organization_id TEXT,
	name TEXT,
	title TEXT NOT NULL,
	description TEXT,
	probability REAL,
	impact REAL,
	score REAL,
	criticality TEXT,
	impact_legacy INTEGER,
	probability_legacy INTEGER,
	status TEXT,
	level TEXT,
	lifecycle_phase TEXT,
	created_by TEXT,
	assigned_to TEXT,
	reviewer_id TEXT,
	owner TEXT,
	asset_id TEXT,
	treatment_plan TEXT,
	residual_risk REAL,
	last_mitigated_at DATETIME,
	slexaf REAL,
	aro REAL,
	downtime_hours REAL,
	hourly_downtime_cost_xaf REAL,
	data_loss_cost_xaf REAL,
	fines_xaf REAL,
	other_direct_cost_xaf REAL,
	remediation_cost_xaf REAL,
	mitigation_effectiveness REAL,
	smart_score REAL,
	smart_level TEXT,
	smart_factors TEXT,
	smart_computed_at DATETIME,
	review_interval_days INTEGER,
	next_review_at DATETIME,
	last_reviewed_at DATETIME,
	source TEXT,
	source_cve_id TEXT,
	external_id TEXT,
	custom_fields TEXT,
	tags TEXT,
	frameworks TEXT,
	control_ids TEXT,
	created_at DATETIME,
	updated_at DATETIME,
	deleted_at DATETIME
);`
