// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	statusapp "github.com/opendefender/openrisk/internal/application/controlstatus"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlexception"
	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #555 — GET /compliance/internal-controls/:controlId/status through the real HTTP
// stack: the permission guard, the real use case, the real loader. The session is
// stamped from request headers, which stand in for the auth middleware's locals.

var ctlStatusNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

type ctlStatusStack struct {
	app              *fiber.App
	db               *gorm.DB
	tenantA, tenantB uuid.UUID
}

func newCtlStatusStack(t *testing.T) *ctlStatusStack {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open("file:ctlstatus"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE risks (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'open', deleted_at DATETIME)`).Error)
	require.NoError(t, db.AutoMigrate(&domain.Evidence{}))
	require.NoError(t, db.AutoMigrate(controlchain.Models()...))
	require.NoError(t, db.AutoMigrate(controlexception.Models()...))
	require.NoError(t, db.AutoMigrate(controlstatus.Models()...))

	s := &ctlStatusStack{db: db, tenantA: uuid.New(), tenantB: uuid.New()}
	h := handler.NewControlStatusHandler(statusapp.NewGetControlStatusUseCase(controlstatus.NewLoader(db), func() time.Time { return ctlStatusNow }))
	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(func(c *fiber.Ctx) error {
		ctx := &middleware.RequestContext{}
		if v, err := uuid.Parse(c.Get("X-Test-Tenant")); err == nil {
			ctx.OrganizationID = v
		}
		if v, err := uuid.Parse(c.Get("X-Test-User")); err == nil {
			ctx.UserID = v
		}
		middleware.SetContext(c, ctx)
		c.Locals("permissions", strings.Split(c.Get("X-Test-Perms"), ","))
		return c.Next()
	})
	api.Get("/compliance/internal-controls/:controlId/status", middleware.RequirePermission("compliance:controls:read"), h.Get)
	s.app = app
	return s
}

// controlWithAPassingTest stores a control of the tenant with one collector test
// whose latest result is a fresh pass.
func (s *ctlStatusStack) controlWithAPassingTest(t *testing.T, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	ctl, impl, test := uuid.New(), uuid.New(), uuid.New()
	prov := `{}`
	require.NoError(t, s.db.Create(&controlchain.ControlRow{
		ID: ctl, TenantID: tenant, Source: "openrisk_user", SourceID: ctl.String(), Confidence: 0.7, Provenance: prov, Name: "A.8.8",
		Applicability: "applicable", DeclaredStatus: "implemented", Tags: "[]",
	}).Error)
	require.NoError(t, s.db.Create(&controlchain.ControlImplementationRow{
		ID: impl, TenantID: tenant, Source: "openrisk_user", SourceID: impl.String(), Confidence: 0.7, Provenance: prov,
		ControlID: ctl, Description: "patching", Status: "in_place",
	}).Error)
	proc := "run it"
	require.NoError(t, s.db.Create(&controlchain.ControlTestRow{
		ID: test, TenantID: tenant, Source: "openrisk_user", SourceID: test.String(), Confidence: 0.7, Provenance: prov,
		ImplementationID: impl, Name: "scan", Checks: "no critical", Method: "manual", Procedure: &proc, PassCriteria: "none",
	}).Error)
	res := uuid.New()
	require.NoError(t, s.db.Create(&controlchain.ControlTestResultRow{
		ID: res, TenantID: tenant, TestID: test, Outcome: "pass", ObservedAt: ctlStatusNow.Add(-time.Hour), RecordedAt: ctlStatusNow.Add(-time.Hour),
		Source: "openrisk_scanner", SourceID: res.String(), Confidence: 0.9, ExecutedByKind: "system", ExecutedByID: "scanner",
	}).Error)
	return ctl
}

type ctlStatusReq struct {
	tenant, user uuid.UUID
	perms        string
}

func (s *ctlStatusStack) do(t *testing.T, who ctlStatusReq, method, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	if who.tenant != uuid.Nil {
		req.Header.Set("X-Test-Tenant", who.tenant.String())
	}
	if who.user != uuid.Nil {
		req.Header.Set("X-Test-User", who.user.String())
	}
	req.Header.Set("X-Test-Perms", who.perms)
	resp, err := s.app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

const ctlStatusRead = "compliance:controls:read"

func TestControlStatusRoute_Success(t *testing.T) {
	s := newCtlStatusStack(t)
	ctl := s.controlWithAPassingTest(t, s.tenantA)
	a := ctlStatusReq{tenant: s.tenantA, user: uuid.New(), perms: ctlStatusRead}

	status, body := s.do(t, a, http.MethodGet, "/api/v1/compliance/internal-controls/"+ctl.String()+"/status")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "pass", body["status"])
	assert.Equal(t, "pass", body["underlying_status"])
	assert.Equal(t, "implemented", body["declared_status"], "the claim is returned beside the computed status")
	assert.Equal(t, ctl.String(), body["control_id"])
	assert.Equal(t, ctlStatusNow.Format(time.RFC3339), body["evaluated_at"])
	assert.EqualValues(t, 0, body["open_exception_count"])
	assert.EqualValues(t, 0, body["open_risk_count"])
	fr, ok := body["freshness"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1, fr["counted"])
	assert.EqualValues(t, 100, fr["percent"])
	srcs, ok := body["sources"].([]any)
	require.True(t, ok)
	require.Len(t, srcs, 1)
	assert.Equal(t, "test_result", srcs[0].(map[string]any)["kind"])
	assert.NotNil(t, body["next_change_at"])
	assert.NotContains(t, body, "tenant_id", "the response carries no tenant")
}

// TestControlStatusRoute_NotFound: tenant B asks for tenant A's control (404, the
// same body as a fabricated id, never 403) — #555 criterion 6 through the stack.
func TestControlStatusRoute_NotFound(t *testing.T) {
	s := newCtlStatusStack(t)
	ctl := s.controlWithAPassingTest(t, s.tenantA)
	b := ctlStatusReq{tenant: s.tenantB, user: uuid.New(), perms: ctlStatusRead}
	base := "/api/v1/compliance/internal-controls/"

	status, foreign := s.do(t, b, http.MethodGet, base+ctl.String()+"/status")
	assert.Equal(t, http.StatusNotFound, status)
	assert.NotContains(t, foreign, "status", "no status of another tenant's control")
	status, fabricated := s.do(t, b, http.MethodGet, base+uuid.NewString()+"/status")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, fabricated, foreign, "a foreign id answers exactly like an absent one")
	assert.Equal(t, domain.CtlStatusNotFound, foreign["error"])

	// Tenant A is unaffected.
	status, _ = s.do(t, ctlStatusReq{tenant: s.tenantA, user: uuid.New(), perms: ctlStatusRead}, http.MethodGet, base+ctl.String()+"/status")
	assert.Equal(t, http.StatusOK, status)
}

func TestControlStatusRoute_Unauthorized(t *testing.T) {
	s := newCtlStatusStack(t)
	ctl := s.controlWithAPassingTest(t, s.tenantA)
	path := "/api/v1/compliance/internal-controls/" + ctl.String() + "/status"

	// No tenant in the session: fail-closed 401, nothing read.
	status, body := s.do(t, ctlStatusReq{user: uuid.New(), perms: ctlStatusRead}, http.MethodGet, path)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.NotContains(t, body, "status")
	assert.Equal(t, domain.CtlStatusUnauthorized, body["error"])
	// No user either.
	status, _ = s.do(t, ctlStatusReq{tenant: s.tenantA, perms: ctlStatusRead}, http.MethodGet, path)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestControlStatusRoute_ForbiddenWithoutThePermission(t *testing.T) {
	s := newCtlStatusStack(t)
	ctl := s.controlWithAPassingTest(t, s.tenantA)
	status, body := s.do(t, ctlStatusReq{tenant: s.tenantA, user: uuid.New(), perms: "compliance:evidences:read"},
		http.MethodGet, "/api/v1/compliance/internal-controls/"+ctl.String()+"/status")
	assert.Equal(t, http.StatusForbidden, status)
	assert.NotContains(t, body, "status")
}

func TestControlStatusRoute_InvalidId(t *testing.T) {
	s := newCtlStatusStack(t)
	a := ctlStatusReq{tenant: s.tenantA, user: uuid.New(), perms: ctlStatusRead}
	for _, id := range []string{"not-a-uuid", uuid.Nil.String()} {
		status, body := s.do(t, a, http.MethodGet, "/api/v1/compliance/internal-controls/"+id+"/status")
		assert.Equal(t, http.StatusBadRequest, status, id)
		assert.Equal(t, domain.CtlStatusInvalidControlID, body["error"])
	}
}

// TestControlStatusRoute_NoWriteRoute: no route accepts a status as input (#555
// criterion 1): only GET is registered under /status.
func TestControlStatusRoute_NoWriteRoute(t *testing.T) {
	s := newCtlStatusStack(t)
	ctl := s.controlWithAPassingTest(t, s.tenantA)
	a := ctlStatusReq{tenant: s.tenantA, user: uuid.New(), perms: "*"}
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete} {
		status, _ := s.do(t, a, m, "/api/v1/compliance/internal-controls/"+ctl.String()+"/status")
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, status, m)
	}
	// And the engine's table is no resource of the API: nothing was written.
	var n int64
	require.NoError(t, s.db.Model(&controlstatus.EvaluationRow{}).Where("tenant_id = ?", s.tenantA).Count(&n).Error)
	assert.Zero(t, n, "a read never writes the cache")
}
