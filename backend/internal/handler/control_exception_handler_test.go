// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"context"
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

	ctlexc "github.com/opendefender/openrisk/internal/application/controlexception"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlexception"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

// #1428 — the three routes through the real HTTP stack: the permission guards,
// the real use cases and repository, the real chained audit store and the real
// mutation middleware. The session (tenant, user, permissions) is stamped from
// request headers, which stand in for the auth middleware's locals.

var ctlExcNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

type ctlExcOwners map[[2]uuid.UUID]bool

func (m ctlExcOwners) Validate(_ context.Context, tenant uuid.UUID, p domain.OwnershipPatch) error {
	for _, id := range p.CandidateIDs() {
		if !m[[2]uuid.UUID{tenant, id}] {
			return domain.NewValidationError("not a member")
		}
	}
	return nil
}

type ctlExcStack struct {
	app              *fiber.App
	db               *gorm.DB
	tenantA, tenantB uuid.UUID
	userA, userB     uuid.UUID
	ownerA           uuid.UUID
	controlA         uuid.UUID
}

func newCtlExcStack(t *testing.T) *ctlExcStack {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ctlexc"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&controlchain.ControlRow{}, &controlexception.ExceptionRow{}, &domain.AuditEvent{}))

	s := &ctlExcStack{db: db, tenantA: uuid.New(), tenantB: uuid.New(), userA: uuid.New(), userB: uuid.New(), ownerA: uuid.New(), controlA: uuid.New()}
	require.NoError(t, db.Create(&controlchain.ControlRow{
		ID: s.controlA, TenantID: s.tenantA, Source: "openrisk_user", SourceID: s.controlA.String(), Confidence: 0.7,
		Provenance: "{}", Name: "A.5.1", Applicability: "applicable", DeclaredStatus: "implemented", Tags: "[]",
	}).Error)

	chain := repository.NewGormAuditChainRepository(db)
	repo := controlexception.NewRepository(db, chain)
	owners := ctlExcOwners{{s.tenantA, s.ownerA}: true}
	clock := func() time.Time { return ctlExcNow }
	h := handler.NewControlExceptionHandler(
		ctlexc.NewCreateUseCase(repo, owners, clock),
		ctlexc.NewListUseCase(repo, clock),
		ctlexc.NewRevokeUseCase(repo, clock))

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
	api.Use(middleware.AuditMutations(chain))
	read := middleware.RequirePermission("compliance:controls:read")
	update := middleware.RequirePermission("compliance:controls:update")
	api.Post("/controls/:controlId/exceptions", update, h.Create)
	api.Get("/controls/:controlId/exceptions", read, h.List)
	api.Post("/control-exceptions/:exceptionId/revoke", update, h.Revoke)
	s.app = app
	return s
}

type ctlExcReq struct {
	tenant, user uuid.UUID
	perms        string
}

func (s *ctlExcStack) asA(perms string) ctlExcReq {
	return ctlExcReq{tenant: s.tenantA, user: s.userA, perms: perms}
}

func (s *ctlExcStack) do(t *testing.T, who ctlExcReq, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func (s *ctlExcStack) createBody(expires time.Time) string {
	b, _ := json.Marshal(map[string]any{"owner_id": s.ownerA, "reason": "vendor patch pending", "expires_at": expires})
	return string(b)
}

func (s *ctlExcStack) path() string { return "/api/v1/controls/" + s.controlA.String() + "/exceptions" }

func (s *ctlExcStack) count(t *testing.T, model any) int64 {
	t.Helper()
	var n int64
	// tenant: test helper counting every row of a table, across tenants, to prove nothing was written.
	require.NoError(t, s.db.Model(model).Count(&n).Error)
	return n
}

const ctlExcFullPerms = "compliance:controls:read,compliance:controls:update"

func TestControlExceptionRoutes_Success(t *testing.T) {
	s := newCtlExcStack(t)
	a := s.asA(ctlExcFullPerms)

	status, body := s.do(t, a, http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(48*time.Hour)))
	require.Equal(t, http.StatusCreated, status, "%v", body)
	assert.Equal(t, "open", body["status"])
	assert.Equal(t, s.userA.String(), body["approved_by"], "approved_by is the session's user")
	assert.NotContains(t, body, "tenant_id")
	id, _ := body["id"].(string)
	require.NotEmpty(t, id)

	status, body = s.do(t, a, http.MethodGet, s.path(), "")
	require.Equal(t, http.StatusOK, status)
	data, _ := body["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "open", data[0].(map[string]any)["status"])

	status, body = s.do(t, a, http.MethodPost, "/api/v1/control-exceptions/"+id+"/revoke", `{"reason":"fixed"}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, "revoked", body["status"])

	// Exactly one audit entry per mutating request: the middleware skipped its
	// own because the use case journaled in the write's transaction.
	var events []domain.AuditEvent
	// tenant: test read of the whole trail to count entries per request.
	require.NoError(t, s.db.Order("sequence ASC").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, domain.AuditActionCreate, events[0].Action)
	assert.Equal(t, domain.AuditActionUpdate, events[1].Action)
	for i, e := range events {
		assert.Equal(t, "control_exception", e.EntityType)
		assert.Equal(t, id, e.EntityID)
		assert.Equal(t, domain.AuditSourceExplicit, e.Source, "not the middleware's http entry")
		assert.Equal(t, s.tenantA, e.TenantID)
		assert.Equal(t, int64(i+1), e.Sequence)
	}
	assert.Equal(t, events[0].Hash, events[1].PrevHash)
}

func TestControlExceptionRoutes_Forbidden(t *testing.T) {
	s := newCtlExcStack(t)
	readOnly := s.asA("compliance:controls:read")

	status, _ := s.do(t, readOnly, http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(time.Hour)))
	assert.Equal(t, http.StatusForbidden, status, "create without update")
	status, _ = s.do(t, readOnly, http.MethodPost, "/api/v1/control-exceptions/"+uuid.NewString()+"/revoke", "")
	assert.Equal(t, http.StatusForbidden, status, "revoke without update")
	status, _ = s.do(t, s.asA("compliance:controls:update"), http.MethodGet, s.path(), "")
	assert.Equal(t, http.StatusForbidden, status, "list without read")

	assert.Zero(t, s.count(t, &controlexception.ExceptionRow{}))
	assert.Zero(t, s.count(t, &domain.AuditEvent{}))
}

func TestControlExceptionRoutes_Unauthorized(t *testing.T) {
	s := newCtlExcStack(t)
	noTenant := ctlExcReq{user: s.userA, perms: ctlExcFullPerms}
	noUser := ctlExcReq{tenant: s.tenantA, perms: ctlExcFullPerms}
	for name, who := range map[string]ctlExcReq{"no tenant": noTenant, "no user": noUser} {
		t.Run(name, func(t *testing.T) {
			status, _ := s.do(t, who, http.MethodGet, s.path(), "")
			assert.Equal(t, http.StatusUnauthorized, status)
			status, _ = s.do(t, who, http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(time.Hour)))
			assert.Equal(t, http.StatusUnauthorized, status)
			status, _ = s.do(t, who, http.MethodPost, "/api/v1/control-exceptions/"+uuid.NewString()+"/revoke", "")
			assert.Equal(t, http.StatusUnauthorized, status)
		})
	}
	assert.Zero(t, s.count(t, &controlexception.ExceptionRow{}))
}

func TestControlExceptionRoutes_CrossTenantIsNotFound(t *testing.T) {
	s := newCtlExcStack(t)
	status, body := s.do(t, s.asA(ctlExcFullPerms), http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(time.Hour)))
	require.Equal(t, http.StatusCreated, status)
	id := body["id"].(string)

	b := ctlExcReq{tenant: s.tenantB, user: s.userB, perms: ctlExcFullPerms}
	status, _ = s.do(t, b, http.MethodGet, s.path(), "")
	assert.Equal(t, http.StatusNotFound, status, "list a control of tenant A")
	status, _ = s.do(t, b, http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(time.Hour)))
	assert.Equal(t, http.StatusNotFound, status, "create on a control of tenant A")
	status, body = s.do(t, b, http.MethodPost, "/api/v1/control-exceptions/"+id+"/revoke", "")
	assert.Equal(t, http.StatusNotFound, status, "revoke an exception of tenant A")
	assert.Equal(t, domain.CtlExcNotFound, body["error"])

	status, body = s.do(t, s.asA(ctlExcFullPerms), http.MethodGet, s.path(), "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "open", body["data"].([]any)[0].(map[string]any)["status"], "tenant A's exception is untouched")
}

func TestControlExceptionRoutes_ValidationAndConflict(t *testing.T) {
	s := newCtlExcStack(t)
	a := s.asA(ctlExcFullPerms)

	status, body := s.do(t, a, http.MethodPost, s.path(), `{"owner_id":"`+s.ownerA.String()+`","reason":"x"}`)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, domain.CtlExcExpiresRequired, body["error"])

	status, body = s.do(t, a, http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(-time.Hour)))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, domain.CtlExcExpiresInPast, body["error"])

	status, body = s.do(t, a, http.MethodPost, s.path(), `not json`)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, domain.CtlExcInvalidBody, body["error"])

	status, body = s.do(t, a, http.MethodPost, "/api/v1/controls/not-a-uuid/exceptions", s.createBody(ctlExcNow.Add(time.Hour)))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, domain.CtlExcInvalidControlID, body["error"])

	other := uuid.New() // a user of no tenant
	bad, _ := json.Marshal(map[string]any{"owner_id": other, "reason": "x", "expires_at": ctlExcNow.Add(time.Hour)})
	status, body = s.do(t, a, http.MethodPost, s.path(), string(bad))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, domain.CtlExcOwnerInvalid, body["error"])
	assert.Zero(t, s.count(t, &controlexception.ExceptionRow{}))

	status, body = s.do(t, a, http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(time.Hour)))
	require.Equal(t, http.StatusCreated, status)
	id := body["id"].(string)
	status, _ = s.do(t, a, http.MethodPost, "/api/v1/control-exceptions/"+id+"/revoke", "")
	require.Equal(t, http.StatusOK, status, "an empty body is a revocation with no reason")
	status, body = s.do(t, a, http.MethodPost, "/api/v1/control-exceptions/"+id+"/revoke", "")
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, domain.CtlExcAlreadyRevoked, body["error"])
}

// A request that fails writes nothing to the trail.
func TestControlExceptionRoutes_FailedRequestIsNotJournaled(t *testing.T) {
	s := newCtlExcStack(t)
	status, _ := s.do(t, s.asA(ctlExcFullPerms), http.MethodPost, s.path(), s.createBody(ctlExcNow.Add(-time.Hour)))
	require.Equal(t, http.StatusBadRequest, status)
	assert.Zero(t, s.count(t, &domain.AuditEvent{}))
}
