// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bulkapp "github.com/opendefender/openrisk/internal/application/bulk"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// #599 criterion 3 — each risk bulk action sits behind its own permission, and a
// risks:update holder cannot delete, through the new routes or the legacy one.
//
// Before #599, POST /risks/bulk was mounted behind risks:update alone and read
// the action from the body, so {"type":"delete"} from any risks:update holder
// deleted risks. These tests pin the fix over HTTP, with the same middleware
// main.go uses; TestRiskBulkRoutes_MainGoMountsEachActionBehindItsPermission
// pins that main.go actually mounts them that way.

// countingRiskStore records every write the engine asks for.
type countingRiskStore struct {
	rows    map[uuid.UUID]uuid.UUID // id -> tenant
	applies int
	deletes int
}

func (s *countingRiskStore) EntityType() string { return "risk" }
func (s *countingRiskStore) SupportedBulkActions() []domain.BulkAction {
	return domain.RiskBulkActions
}
func (s *countingRiskStore) ValidateBulkChange(c domain.BulkChange) error {
	return domain.ValidateRiskBulkChange(c)
}
func (s *countingRiskStore) LoadBulk(_ context.Context, tenant uuid.UUID, ids []uuid.UUID) ([]domain.BulkRow, error) {
	out := []domain.BulkRow{}
	for _, id := range ids {
		if s.rows[id] == tenant {
			out = append(out, domain.BulkRow{ID: id, Label: "r", Snapshot: map[string]interface{}{"status": "open"}})
		}
	}
	return out, nil
}
func (s *countingRiskStore) ApplyBulk(_ context.Context, _ uuid.UUID, ids []uuid.UUID, _ domain.BulkChange) ([]domain.BulkMutation, error) {
	s.applies++
	return []domain.BulkMutation{{ID: ids[0]}}, nil
}
func (s *countingRiskStore) DeleteBulk(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]domain.BulkMutation, error) {
	s.deletes++
	return []domain.BulkMutation{{ID: ids[0]}}, nil
}

// legacyRiskRepo is the repository behind the legacy use case. Only the two
// bulk writes are implemented; anything else panics, which fails the test.
type legacyRiskRepo struct {
	domain.RiskRepository
	deletes int
	applies int
}

func (r *legacyRiskRepo) BulkDelete(context.Context, []uuid.UUID, uuid.UUID) (int64, error) {
	r.deletes++
	return 1, nil
}
func (r *legacyRiskRepo) BulkApply(_ context.Context, _ uuid.UUID, ids []uuid.UUID, _ func(*domain.Risk) error) ([]domain.RiskMutation, error) {
	r.applies++
	return []domain.RiskMutation{{RiskID: ids[0]}}, nil
}

type riskBulkFixture struct {
	app    *fiber.App
	store  *countingRiskStore
	legacy *legacyRiskRepo
	tenant uuid.UUID
	id     uuid.UUID
}

// newRiskBulkApp mounts the routes exactly as main.go does, for a caller
// holding perms.
func newRiskBulkApp(t *testing.T, perms ...string) *riskBulkFixture {
	t.Helper()
	tenant, user, id := uuid.New(), uuid.New(), uuid.New()
	store := &countingRiskStore{rows: map[uuid.UUID]uuid.UUID{id: tenant}}
	legacy := &legacyRiskRepo{}

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	protected := app.Group("/api/v1", func(c *fiber.Ctx) error {
		c.Locals("permissions", perms)
		middleware.SetContext(c, &middleware.RequestContext{UserID: user, OrganizationID: tenant})
		return c.Next()
	})

	riskRead := middleware.RequirePermission("risks:read")
	riskUpdate := middleware.RequirePermission("risks:update")
	riskDelete := middleware.RequirePermission("risks:delete")

	riskHandler := (&RiskHandler{}).WithBulkAction(risk.NewBulkActionUseCase(legacy, nil))
	protected.Post("/risks/bulk", riskUpdate, riskHandler.BulkAction)

	riskBulk := NewBulkHandler(bulkapp.New(store, nil))
	protected.Get("/risks/bulk/capabilities", riskRead, riskBulk.Capabilities)
	protected.Post("/risks/bulk/preview", riskRead, riskBulk.Preview)
	protected.Post("/risks/bulk/change-status", riskUpdate, riskBulk.Apply(domain.BulkActionChangeStatus))
	protected.Post("/risks/bulk/add-tags", riskUpdate, riskBulk.Apply(domain.BulkActionAddTags))
	protected.Post("/risks/bulk/remove-tags", riskUpdate, riskBulk.Apply(domain.BulkActionRemoveTags))
	protected.Post("/risks/bulk/delete", riskDelete, riskBulk.Apply(domain.BulkActionDelete))

	return &riskBulkFixture{app: app, store: store, legacy: legacy, tenant: tenant, id: id}
}

func (f *riskBulkFixture) do(t *testing.T, method, path, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out := map[string]interface{}{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.Header.Get("Sunset") != "" {
		out["__sunset"] = resp.Header.Get("Sunset")
	}
	return resp.StatusCode, out
}

func (f *riskBulkFixture) ids() string { return `"ids":["` + f.id.String() + `"]` }

// The defect #599 asked about, answered over HTTP: an update-only caller is
// refused on BOTH delete paths, and nothing is deleted.
func TestRiskBulkRoutes_Unauthorized_UpdateHolderCannotDelete(t *testing.T) {
	f := newRiskBulkApp(t, "risks:read", "risks:update")

	status, _ := f.do(t, "POST", "/api/v1/risks/bulk/delete", `{`+f.ids()+`}`)
	assert.Equal(t, fiber.StatusForbidden, status, "the delete route needs risks:delete")

	status, _ = f.do(t, "POST", "/api/v1/risks/bulk",
		`{"type":"delete","risk_ids":["`+f.id.String()+`"]}`)
	assert.Equal(t, fiber.StatusForbidden, status, "the legacy route needs risks:delete too (#1423)")

	assert.Zero(t, f.store.deletes, "nothing deleted through the governed route")
	assert.Zero(t, f.legacy.deletes, "nothing deleted through the legacy route")
}

// Until the sunset, the legacy route still deletes for a caller who HOLDS
// risks:delete (#1423); the governed route is the replacement, not a removal.
func TestRiskBulkRoutes_LegacyDeleteStillWorksWithTheDeleteRight(t *testing.T) {
	f := newRiskBulkApp(t, "risks:*")
	status, _ := f.do(t, "POST", "/api/v1/risks/bulk",
		`{"type":"delete","risk_ids":["`+f.id.String()+`"]}`)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, 1, f.legacy.deletes)
}

func TestRiskBulkRoutes_Success_DeleteHolderDeletes(t *testing.T) {
	f := newRiskBulkApp(t, "risks:read", "risks:delete")
	status, body := f.do(t, "POST", "/api/v1/risks/bulk/delete", `{`+f.ids()+`}`)
	assert.Equal(t, fiber.StatusOK, status, "%v", body)
	assert.Equal(t, 1, f.store.deletes)
}

// Each update-class action needs risks:update and nothing else.
func TestRiskBulkRoutes_EachActionBehindItsOwnPermission(t *testing.T) {
	routes := map[string]string{
		"/api/v1/risks/bulk/change-status": `{` + "%s" + `,"status":"closed"}`,
		"/api/v1/risks/bulk/add-tags":      `{` + "%s" + `,"tags":["pci"]}`,
		"/api/v1/risks/bulk/remove-tags":   `{` + "%s" + `,"tags":["pci"]}`,
	}
	for path, tmpl := range routes {
		readOnly := newRiskBulkApp(t, "risks:read", "risks:delete")
		status, _ := readOnly.do(t, "POST", path, strings.Replace(tmpl, "%s", readOnly.ids(), 1))
		assert.Equal(t, fiber.StatusForbidden, status, "%s without risks:update", path)
		assert.Zero(t, readOnly.store.applies, path)

		updater := newRiskBulkApp(t, "risks:update")
		status, body := updater.do(t, "POST", path, strings.Replace(tmpl, "%s", updater.ids(), 1))
		assert.Equal(t, fiber.StatusOK, status, "%s with risks:update: %v", path, body)
		assert.Equal(t, 1, updater.store.applies, path)
		assert.Zero(t, updater.store.deletes, path)
	}
}

// The action comes from the route. An "action":"delete" in the body of the
// change-status route changes a status, it does not delete.
func TestRiskBulkRoutes_BodyActionIsIgnoredOnApply(t *testing.T) {
	f := newRiskBulkApp(t, "risks:update")
	status, _ := f.do(t, "POST", "/api/v1/risks/bulk/change-status",
		`{`+f.ids()+`,"action":"delete","status":"closed"}`)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Zero(t, f.store.deletes)
	assert.Equal(t, 1, f.store.applies)
}

// Criterion 1 over HTTP: the preview is behind risks:read and writes nothing.
func TestRiskBulkRoutes_Preview_ReadOnly(t *testing.T) {
	f := newRiskBulkApp(t, "risks:read")
	status, body := f.do(t, "POST", "/api/v1/risks/bulk/preview", `{`+f.ids()+`,"action":"delete"}`)
	assert.Equal(t, fiber.StatusOK, status, "%v", body)
	assert.EqualValues(t, 1, body["found"])
	assert.Zero(t, f.store.deletes)
	assert.Zero(t, f.store.applies)

	none := newRiskBulkApp(t)
	status, _ = none.do(t, "POST", "/api/v1/risks/bulk/preview", `{`+none.ids()+`,"action":"delete"}`)
	assert.Equal(t, fiber.StatusForbidden, status)
}

// Criterion 4 — the capabilities name what the register can do, and no more.
func TestRiskBulkRoutes_CapabilitiesAreTheRegistersOwn(t *testing.T) {
	f := newRiskBulkApp(t, "risks:read")
	status, body := f.do(t, "GET", "/api/v1/risks/bulk/capabilities", "")
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "risk", body["entity_type"])
	assert.ElementsMatch(t,
		[]interface{}{"change_status", "add_tags", "remove_tags", "delete"}, body["actions"])
}

// Criterion 5 — the legacy route announces its end.
func TestRiskBulkRoutes_LegacyRouteIsDeprecatedWithADate(t *testing.T) {
	f := newRiskBulkApp(t, "risks:update")
	status, body := f.do(t, "POST", "/api/v1/risks/bulk",
		`{"type":"add_tags","risk_ids":["`+f.id.String()+`"],"tags":["pci"]}`)
	assert.Equal(t, fiber.StatusOK, status, "%v", body)
	assert.Equal(t, LegacyBulkSunset, body["__sunset"])
	assert.Equal(t, 1, f.legacy.applies, "non-delete actions still work until the sunset")
}

// The HTTP tests above mount the routes themselves. This pins that main.go
// mounts them the same way, so the two cannot drift apart.
func TestRiskBulkRoutes_MainGoMountsEachActionBehindItsPermission(t *testing.T) {
	src := readMainGo(t)

	guards := map[string]string{
		`riskRead := middleware.RequirePermission("risks:read")`:     "riskRead",
		`riskUpdate := middleware.RequirePermission("risks:update")`: "riskUpdate",
		`riskDelete := middleware.RequirePermission("risks:delete")`: "riskDelete",
	}
	for decl := range guards {
		assert.Contains(t, src, decl)
	}

	want := map[string]string{
		`Get("/risks/bulk/capabilities"`:   "riskRead",
		`Post("/risks/bulk/preview"`:       "riskRead",
		`Post("/risks/bulk/change-status"`: "riskUpdate",
		`Post("/risks/bulk/add-tags"`:      "riskUpdate",
		`Post("/risks/bulk/remove-tags"`:   "riskUpdate",
		`Post("/risks/bulk/delete"`:        "riskDelete",
	}
	for route, guard := range want {
		re := regexp.MustCompile(`protected\.` + regexp.QuoteMeta(route) + `,\s*(\w+)\s*,`)
		m := re.FindAllStringSubmatch(src, -1)
		require.Len(t, m, 1, "%s must be mounted exactly once", route)
		assert.Equal(t, guard, m[0][1], "%s is behind the wrong permission", route)
	}
}
