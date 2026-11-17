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

	ctlchain "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #1414 — the nine routes through the real HTTP stack: the permission guards, the
// real use cases and repository, the real chained audit store and the real
// mutation middleware. The session is stamped from request headers, which stand
// in for the auth middleware's locals.

var intCtlNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

type intCtlStack struct {
	app              *fiber.App
	db               *gorm.DB
	tenantA, tenantB uuid.UUID
	userA, userB     uuid.UUID
	ownerA           uuid.UUID
}

const intCtlAllPerms = "compliance:controls:read,compliance:controls:create,compliance:controls:update,compliance:controls:delete"

// intCtlResultPerms adds the evidence module's create permission, which attaching
// evidence to a result takes (D-108 Q9).
const intCtlResultPerms = intCtlAllPerms + ",compliance:evidences:create"

func newIntCtlStack(t *testing.T) *intCtlStack {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open("file:intctl"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE compliance_controls (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, framework_id TEXT NOT NULL,
		reference_code TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, description TEXT, source_reference TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'not_implemented', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	// The columns the applies_to resolver reads (#1415); the model has columns SQLite cannot hold.
	require.NoError(t, db.Exec(`CREATE TABLE assets (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', deleted_at DATETIME)`).Error)
	require.NoError(t, db.AutoMigrate(&domain.Evidence{}, &domain.AuditEvent{}))
	require.NoError(t, db.AutoMigrate(controlchain.Models()...))

	s := &intCtlStack{db: db, tenantA: uuid.New(), tenantB: uuid.New(), userA: uuid.New(), userB: uuid.New(), ownerA: uuid.New()}

	chain := repository.NewGormAuditChainRepository(db)
	repo := controlchain.NewControlRepository(db, chain)
	clock := func() time.Time { return intCtlNow }
	owners := intCtlMembers{{s.tenantA, s.ownerA}: true}
	h := handler.NewInternalControlHandler(
		ctlchain.NewCreateUseCase(repo, owners, clock), ctlchain.NewGetUseCase(repo), ctlchain.NewListUseCase(repo),
		ctlchain.NewUpdateUseCase(repo, owners, clock), ctlchain.NewDeleteUseCase(repo, clock),
		ctlchain.NewLinkUseCase(repo, clock), ctlchain.NewUnlinkUseCase(repo, clock),
		ctlchain.NewListRequirementsUseCase(repo), ctlchain.NewListForRequirementUseCase(repo))

	implRepo := controlchain.NewImplementationRepository(db, chain)
	testRepo := controlchain.NewTestRepository(db, chain)
	implHandler := handler.NewControlImplementationHandler(
		ctlchain.NewCreateImplementationUseCase(implRepo, owners, clock), ctlchain.NewGetImplementationUseCase(implRepo),
		ctlchain.NewListImplementationsUseCase(implRepo), ctlchain.NewUpdateImplementationUseCase(implRepo, owners, clock),
		ctlchain.NewDeleteImplementationUseCase(implRepo, clock))
	testHandler := handler.NewControlTestDefinitionHandler(
		ctlchain.NewCreateTestUseCase(testRepo, clock), ctlchain.NewGetTestUseCase(testRepo), ctlchain.NewListTestsUseCase(testRepo),
		ctlchain.NewUpdateTestUseCase(testRepo, clock), ctlchain.NewDeleteTestUseCase(testRepo, clock))

	resultRepo := controlchain.NewResultRepository(db, chain)
	resultHandler := handler.NewControlTestResultHandler(
		ctlchain.NewRecordResultUseCase(resultRepo, owners, clock), ctlchain.NewListResultsUseCase(resultRepo),
		ctlchain.NewLatestResultUseCase(resultRepo), ctlchain.NewAttachResultEvidenceUseCase(resultRepo, clock),
		ctlchain.NewListResultEvidenceUseCase(resultRepo))

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
	create := middleware.RequirePermission("compliance:controls:create")
	update := middleware.RequirePermission("compliance:controls:update")
	del := middleware.RequirePermission("compliance:controls:delete")
	api.Get("/compliance/internal-controls", read, h.List)
	api.Post("/compliance/internal-controls", create, h.Create)
	api.Get("/compliance/internal-controls/:controlId", read, h.Get)
	api.Patch("/compliance/internal-controls/:controlId", update, h.Update)
	api.Delete("/compliance/internal-controls/:controlId", del, h.Delete)
	api.Get("/compliance/internal-controls/:controlId/requirements", read, h.ListRequirements)
	api.Post("/compliance/internal-controls/:controlId/requirements/:requirementId", update, h.Link)
	api.Delete("/compliance/internal-controls/:controlId/requirements/:requirementId", update, h.Unlink)
	api.Get("/compliance/controls/:controlId/internal-controls", read, h.ListForRequirement)
	api.Get("/compliance/internal-controls/:controlId/implementations", read, implHandler.List)
	api.Post("/compliance/internal-controls/:controlId/implementations", create, implHandler.Create)
	api.Get("/compliance/implementations/:implementationId", read, implHandler.Get)
	api.Patch("/compliance/implementations/:implementationId", update, implHandler.Update)
	api.Delete("/compliance/implementations/:implementationId", del, implHandler.Delete)
	api.Get("/compliance/implementations/:implementationId/tests", read, testHandler.List)
	api.Post("/compliance/implementations/:implementationId/tests", create, testHandler.Create)
	api.Get("/compliance/tests/:testId", read, testHandler.Get)
	api.Patch("/compliance/tests/:testId", update, testHandler.Update)
	api.Delete("/compliance/tests/:testId", del, testHandler.Delete)
	evidenceCreate := middleware.RequirePermission("compliance:evidences:create")
	api.Get("/compliance/tests/:testId/results", read, resultHandler.List)
	api.Post("/compliance/tests/:testId/results", update, resultHandler.Record)
	api.Get("/compliance/tests/:testId/results/latest", read, resultHandler.Latest)
	api.Get("/compliance/tests/:testId/results/:resultId/evidence", read, resultHandler.ListEvidence)
	api.Post("/compliance/tests/:testId/results/:resultId/evidence", evidenceCreate, resultHandler.AttachEvidence)
	s.app = app
	return s
}

type intCtlMembers map[[2]uuid.UUID]bool

func (m intCtlMembers) Validate(_ context.Context, tenant uuid.UUID, p domain.OwnershipPatch) error {
	for _, id := range p.CandidateIDs() {
		if !m[[2]uuid.UUID{tenant, id}] {
			return domain.NewValidationError("not a member")
		}
	}
	return nil
}

type intCtlReq struct {
	tenant, user uuid.UUID
	perms        string
}

func (s *intCtlStack) asA(perms string) intCtlReq {
	return intCtlReq{tenant: s.tenantA, user: s.userA, perms: perms}
}

func (s *intCtlStack) asB() intCtlReq {
	return intCtlReq{tenant: s.tenantB, user: s.userB, perms: intCtlAllPerms}
}

func (s *intCtlStack) do(t *testing.T, who intCtlReq, method, path, body string) (int, map[string]any) {
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

func (s *intCtlStack) requirement(t *testing.T, tenant uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, s.db.Exec(`INSERT INTO compliance_controls (id, tenant_id, framework_id, reference_code, name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'not_implemented', ?, ?)`, id, tenant, uuid.New(), name[:3], name, intCtlNow, intCtlNow).Error)
	return id
}

func (s *intCtlStack) asset(t *testing.T, tenant uuid.UUID) string {
	t.Helper()
	id := uuid.New()
	require.NoError(t, s.db.Exec(`INSERT INTO assets (id, tenant_id, name) VALUES (?, ?, 'core banking')`, id, tenant).Error)
	return id.String()
}

// evidence stores an evidence of the tenant and returns its id.
func (s *intCtlStack) evidence(t *testing.T, tenant uuid.UUID) string {
	t.Helper()
	ev := &domain.Evidence{TenantID: tenant, Title: "proof", CollectedAt: intCtlNow, Review: domain.EvidenceReviewAccepted}
	require.NoError(t, repository.NewGormEvidenceRepository(s.db).Create(context.Background(), ev))
	return ev.ID.String()
}

func (s *intCtlStack) createControl(t *testing.T, who intCtlReq, name string) string {
	t.Helper()
	status, body := s.do(t, who, http.MethodPost, "/api/v1/compliance/internal-controls", `{"name":"`+name+`"}`)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	id, _ := body["id"].(string)
	require.NotEmpty(t, id)
	return id
}

func (s *intCtlStack) count(t *testing.T, model any) int64 {
	t.Helper()
	var n int64
	// tenant: test helper counting every row of a table, across tenants, to prove nothing was written.
	require.NoError(t, s.db.Model(model).Count(&n).Error)
	return n
}

func TestInternalControlRoutes_Success(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	base := "/api/v1/compliance/internal-controls"
	iso, soc2 := s.requirement(t, s.tenantA, "ISO A.5.15"), s.requirement(t, s.tenantA, "SOC2 CC6.1")

	id := s.createControl(t, a, "Access review")
	for _, req := range []uuid.UUID{iso, soc2} {
		status, body := s.do(t, a, http.MethodPost, base+"/"+id+"/requirements/"+req.String(), "")
		require.Equal(t, http.StatusCreated, status, "%v", body)
	}
	status, _ := s.do(t, a, http.MethodPost, base+"/"+id+"/requirements/"+iso.String(), "")
	assert.Equal(t, http.StatusOK, status, "linking twice is idempotent")

	// AC1: both links read back, with the server-set envelope.
	status, body := s.do(t, a, http.MethodGet, base+"/"+id+"/requirements", "")
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, body["data"], 2)

	// AC2: either requirement returns the control; one edit changes both.
	status, body = s.do(t, a, http.MethodPatch, base+"/"+id, `{"name":"Quarterly access review","tags":["iam"]}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	for _, req := range []uuid.UUID{iso, soc2} {
		status, body = s.do(t, a, http.MethodGet, "/api/v1/compliance/controls/"+req.String()+"/internal-controls", "")
		require.Equal(t, http.StatusOK, status)
		data := body["data"].([]any)
		var found map[string]any
		for _, d := range data {
			if m := d.(map[string]any); m["id"] == id {
				found = m
			}
		}
		require.NotNil(t, found)
		assert.Equal(t, "Quarterly access review", found["name"])
		assert.Equal(t, "openrisk_user", found["source"])
		assert.EqualValues(t, 0.7, found["confidence"])
		prov := found["provenance"].(map[string]any)
		assert.Equal(t, "manual_entry", prov["method"])
		assert.Equal(t, map[string]any{"kind": "user", "id": s.userA.String()}, prov["actor"])
		assert.NotContains(t, found, "tenant_id")
	}

	status, body = s.do(t, a, http.MethodGet, base, "")
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, body["data"], 1)
	status, body = s.do(t, a, http.MethodGet, base+"/"+id, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, id, body["id"])

	status, _ = s.do(t, a, http.MethodDelete, base+"/"+id+"/requirements/"+iso.String(), "")
	assert.Equal(t, http.StatusNoContent, status)
	status, _ = s.do(t, a, http.MethodDelete, base+"/"+id, "")
	assert.Equal(t, http.StatusNoContent, status)
	status, _ = s.do(t, a, http.MethodGet, base+"/"+id, "")
	assert.Equal(t, http.StatusNotFound, status)

	// One audit entry per mutating request, written by the use case in the write's
	// transaction (explicit), never a second one from the middleware.
	var events []domain.AuditEvent
	// tenant: test read of the whole trail to count entries per request.
	require.NoError(t, s.db.Order("sequence ASC").Find(&events).Error)
	require.Len(t, events, 7, "create, two links, edit, one unlink, delete: six explicit; the repeated link changed nothing, so the middleware records it as an http entry")
	var explicit, viaMiddleware int
	for _, e := range events {
		assert.Equal(t, s.tenantA, e.TenantID)
		switch e.Source {
		case domain.AuditSourceExplicit:
			explicit++
			assert.Equal(t, "control", e.EntityType)
		default:
			viaMiddleware++
		}
	}
	assert.Equal(t, 6, explicit)
	assert.Equal(t, 1, viaMiddleware)
}

func TestInternalControlRoutes_Forbidden(t *testing.T) {
	s := newIntCtlStack(t)
	id := s.createControl(t, s.asA(intCtlAllPerms), "c")
	req := s.requirement(t, s.tenantA, "REQ")
	base := "/api/v1/compliance/internal-controls"
	readOnly := s.asA("compliance:controls:read")

	for name, c := range map[string]struct{ method, path, body string }{
		"create": {http.MethodPost, base, `{"name":"x"}`},
		"patch":  {http.MethodPatch, base + "/" + id, `{"name":"x"}`},
		"delete": {http.MethodDelete, base + "/" + id, ""},
		"link":   {http.MethodPost, base + "/" + id + "/requirements/" + req.String(), ""},
		"unlink": {http.MethodDelete, base + "/" + id + "/requirements/" + req.String(), ""},
	} {
		status, _ := s.do(t, readOnly, c.method, c.path, c.body)
		assert.Equal(t, http.StatusForbidden, status, name)
	}
	status, _ := s.do(t, s.asA("compliance:controls:update"), http.MethodGet, base, "")
	assert.Equal(t, http.StatusForbidden, status, "list without read")
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlRow{}))
}

func TestInternalControlRoutes_Unauthorized(t *testing.T) {
	s := newIntCtlStack(t)
	id := s.createControl(t, s.asA(intCtlAllPerms), "c")
	req := s.requirement(t, s.tenantA, "REQ")
	base := "/api/v1/compliance/internal-controls"
	noTenant := intCtlReq{user: s.userA, perms: intCtlAllPerms}
	noUser := intCtlReq{tenant: s.tenantA, perms: intCtlAllPerms}
	for name, who := range map[string]intCtlReq{"no tenant": noTenant, "no user": noUser} {
		for _, c := range []struct{ method, path, body string }{
			{http.MethodGet, base, ""},
			{http.MethodPost, base, `{"name":"x"}`},
			{http.MethodGet, base + "/" + id, ""},
			{http.MethodPatch, base + "/" + id, `{"name":"x"}`},
			{http.MethodDelete, base + "/" + id, ""},
			{http.MethodGet, base + "/" + id + "/requirements", ""},
			{http.MethodPost, base + "/" + id + "/requirements/" + req.String(), ""},
			{http.MethodDelete, base + "/" + id + "/requirements/" + req.String(), ""},
			{http.MethodGet, "/api/v1/compliance/controls/" + req.String() + "/internal-controls", ""},
		} {
			status, _ := s.do(t, who, c.method, c.path, c.body)
			assert.Equal(t, http.StatusUnauthorized, status, name+" "+c.method+" "+c.path)
		}
	}
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlRow{}))
}

func TestInternalControlRoutes_CrossTenantIsNotFound(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	b := s.asB()
	base := "/api/v1/compliance/internal-controls"
	id := s.createControl(t, a, "A's control")
	reqA := s.requirement(t, s.tenantA, "REQ")
	reqB := s.requirement(t, s.tenantB, "REQ")
	status, _ := s.do(t, a, http.MethodPost, base+"/"+id+"/requirements/"+reqA.String(), "")
	require.Equal(t, http.StatusCreated, status)

	for name, c := range map[string]struct{ method, path, body string }{
		"get":                     {http.MethodGet, base + "/" + id, ""},
		"patch":                   {http.MethodPatch, base + "/" + id, `{"name":"hijacked"}`},
		"delete":                  {http.MethodDelete, base + "/" + id, ""},
		"requirements":            {http.MethodGet, base + "/" + id + "/requirements", ""},
		"link own req to A's ctl": {http.MethodPost, base + "/" + id + "/requirements/" + reqB.String(), ""},
		"unlink A's edge":         {http.MethodDelete, base + "/" + id + "/requirements/" + reqA.String(), ""},
		"controls of A's req":     {http.MethodGet, "/api/v1/compliance/controls/" + reqA.String() + "/internal-controls", ""},
	} {
		status, body := s.do(t, b, c.method, c.path, c.body)
		assert.Equal(t, http.StatusNotFound, status, name)
		assert.NotContains(t, body, "data", name)
	}
	// B creates its own control and tries to link it to A's requirement.
	idB := s.createControl(t, b, "B's control")
	status, _ = s.do(t, b, http.MethodPost, base+"/"+idB+"/requirements/"+reqA.String(), "")
	assert.Equal(t, http.StatusNotFound, status, "A's requirement from B's session")
	status, body := s.do(t, b, http.MethodGet, base, "")
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, body["data"], 1, "B lists only its own")

	// Tenant A's control is untouched.
	status, body = s.do(t, a, http.MethodGet, base+"/"+id, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "A's control", body["name"])
	status, body = s.do(t, a, http.MethodGet, base+"/"+id+"/requirements", "")
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, body["data"], 1)
}

// AC4: a body that carries tenant_id, source, confidence or provenance changes
// nothing. The DTOs have no field for the envelope.
func TestCanonicalWrite_EnvelopeNotClientWritable(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	base := "/api/v1/compliance/internal-controls"
	forgedID := uuid.NewString()
	envelope := `"id":"` + forgedID + `","tenant_id":"` + s.tenantB.String() + `","source":"scanner","source_id":"forged",` +
		`"source_version":"9","confidence":1,"observed_at":"2020-01-01T00:00:00Z","created_at":"2001-01-01T00:00:00Z",` +
		`"provenance":{"method":"migration","actor":{"kind":"system","id":"control_split"},"confidence_rule":"x"},` +
		`"applicability_review_pending":true,"relationships":[{"verb":"addressed_by"}]`

	var status int
	var body map[string]any
	status, body = s.do(t, a, http.MethodPost, base, `{"name":"Forged",`+envelope+`}`)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	id := body["id"].(string)
	assert.NotEqual(t, forgedID, id, "the id is the server's")
	assert.Equal(t, "openrisk_user", body["source"])
	assert.Equal(t, id, body["source_id"])
	assert.EqualValues(t, 0.7, body["confidence"])
	assert.Equal(t, false, body["applicability_review_pending"])
	assert.Equal(t, map[string]any{"method": "manual_entry", "actor": map[string]any{"kind": "user", "id": s.userA.String()}, "confidence_rule": "confidence/1"}, body["provenance"])
	assert.Nil(t, body["source_version"])

	var row controlchain.ControlRow
	require.NoError(t, s.db.Where("id = ?", id).Take(&row).Error)
	assert.Equal(t, s.tenantA, row.TenantID, "the tenant is the session's")
	assert.Nil(t, row.ObservedAt)
	assert.Nil(t, row.SourceVersion)
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlRow{}), "no second row under the forged id or tenant")

	// The same body on an edit changes only what the DTO carries.
	before := row
	status, body = s.do(t, a, http.MethodPatch, base+"/"+id, `{"name":"Renamed",`+envelope+`}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	var after controlchain.ControlRow
	require.NoError(t, s.db.Where("id = ?", id).Take(&after).Error)
	assert.Equal(t, "Renamed", after.Name)
	assert.Equal(t, before.TenantID, after.TenantID)
	assert.Equal(t, before.Source, after.Source)
	assert.Equal(t, before.SourceID, after.SourceID)
	assert.Equal(t, before.Confidence, after.Confidence)
	assert.JSONEq(t, before.Provenance, after.Provenance)
	assert.Equal(t, before.ApplicabilityReviewPending, after.ApplicabilityReviewPending)
	assert.Nil(t, after.ObservedAt)

	// #1415 AC5: the same holds for an implementation and for a test, on create and on edit.
	asset := s.asset(t, s.tenantA)
	status, body = s.do(t, a, http.MethodPost, base+"/"+id+"/implementations",
		`{"description":"Quarterly review","applies_to":[{"type":"Asset","id":"`+asset+`"}],`+envelope+`}`)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	implID := body["id"].(string)
	assert.NotEqual(t, forgedID, implID, "the id is the server's")
	assert.Equal(t, "openrisk_user", body["source"])
	assert.Equal(t, implID, body["source_id"])
	assert.EqualValues(t, 0.7, body["confidence"])
	assert.Equal(t, map[string]any{"method": "manual_entry", "actor": map[string]any{"kind": "user", "id": s.userA.String()}, "confidence_rule": "confidence/1"}, body["provenance"])
	assert.Nil(t, body["observed_at"])
	assert.Nil(t, body["source_version"])
	var implRow controlchain.ControlImplementationRow
	require.NoError(t, s.db.Where("id = ?", implID).Take(&implRow).Error)
	assert.Equal(t, s.tenantA, implRow.TenantID, "the tenant is the session's")
	assert.Nil(t, implRow.ObservedAt)
	assert.Nil(t, implRow.SourceVersion)
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlImplementationRow{}), "no second row under the forged id or tenant")

	implBefore := implRow
	status, body = s.do(t, a, http.MethodPatch, "/api/v1/compliance/implementations/"+implID, `{"description":"Renamed",`+envelope+`}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	var implAfter controlchain.ControlImplementationRow
	require.NoError(t, s.db.Where("id = ?", implID).Take(&implAfter).Error)
	assert.Equal(t, "Renamed", implAfter.Description)
	assert.Equal(t, implBefore.TenantID, implAfter.TenantID)
	assert.Equal(t, implBefore.Source, implAfter.Source)
	assert.Equal(t, implBefore.SourceID, implAfter.SourceID)
	assert.Equal(t, implBefore.Confidence, implAfter.Confidence)
	assert.JSONEq(t, implBefore.Provenance, implAfter.Provenance)
	assert.Equal(t, implBefore.ControlID, implAfter.ControlID, "the control is not editable either")
	assert.Nil(t, implAfter.ObservedAt)

	status, body = s.do(t, a, http.MethodPost, "/api/v1/compliance/implementations/"+implID+"/tests",
		`{"name":"Review","checks":"list matches HR","method":"manual","procedure":"export and compare","pass_criteria":"no orphan",`+envelope+`}`)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	testID := body["id"].(string)
	assert.NotEqual(t, forgedID, testID)
	assert.Equal(t, "openrisk_user", body["source"])
	assert.Equal(t, testID, body["source_id"])
	assert.EqualValues(t, 0.7, body["confidence"])
	assert.Equal(t, map[string]any{"method": "manual_entry", "actor": map[string]any{"kind": "user", "id": s.userA.String()}, "confidence_rule": "confidence/1"}, body["provenance"])
	assert.Nil(t, body["source_version"])
	var testRow controlchain.ControlTestRow
	require.NoError(t, s.db.Where("id = ?", testID).Take(&testRow).Error)
	assert.Equal(t, s.tenantA, testRow.TenantID)
	assert.Nil(t, testRow.ObservedAt, "a definition is not observed (D4)")
	assert.Nil(t, testRow.SourceVersion)
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestRow{}))

	testBefore := testRow
	status, body = s.do(t, a, http.MethodPatch, "/api/v1/compliance/tests/"+testID, `{"name":"Renamed",`+envelope+`}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	var testAfter controlchain.ControlTestRow
	require.NoError(t, s.db.Where("id = ?", testID).Take(&testAfter).Error)
	assert.Equal(t, "Renamed", testAfter.Name)
	assert.Equal(t, testBefore.TenantID, testAfter.TenantID)
	assert.Equal(t, testBefore.Source, testAfter.Source)
	assert.Equal(t, testBefore.SourceID, testAfter.SourceID)
	assert.Equal(t, testBefore.Confidence, testAfter.Confidence)
	assert.JSONEq(t, testBefore.Provenance, testAfter.Provenance)
	assert.Equal(t, testBefore.ImplementationID, testAfter.ImplementationID)
	assert.Nil(t, testAfter.ObservedAt)

	// #1416 AC5: the same holds for a result, and for the evidence edge on it.
	ra := s.asA(intCtlResultPerms)
	resultBody := `{"outcome":"pass","observed_at":"2026-11-16T10:00:00Z","note":"ran",` + envelope + `,"recorded_at":"2001-01-01T00:00:00Z","result_id":"` + forgedID + `"}`
	status, body = s.do(t, ra, http.MethodPost, testBase+"/"+testID+"/results", resultBody)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	resultID := body["id"].(string)
	assert.NotEqual(t, forgedID, resultID, "the id is the server's")
	assert.Equal(t, "openrisk_user", body["source"])
	assert.Equal(t, resultID, body["source_id"])
	assert.EqualValues(t, 0.7, body["confidence"])
	assert.Equal(t, map[string]any{"kind": "user", "id": s.userA.String()}, body["executed_by"])
	assert.Equal(t, "2026-11-16T12:00:00Z", body["recorded_at"], "recorded_at is the server's clock")
	var resRow controlchain.ControlTestResultRow
	require.NoError(t, s.db.Where("id = ?", resultID).Take(&resRow).Error)
	assert.Equal(t, s.tenantA, resRow.TenantID, "the tenant is the session's")
	assert.Equal(t, testID, resRow.TestID.String())
	assert.Equal(t, "openrisk_user", resRow.Source)
	assert.Equal(t, resultID, resRow.SourceID)
	assert.Equal(t, 0.7, resRow.Confidence)
	assert.Equal(t, s.userA.String(), resRow.ExecutedByID)
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestResultRow{}), "no second row under the forged id or tenant")

	evID := s.evidence(t, s.tenantA)
	status, body = s.do(t, ra, http.MethodPost, testBase+"/"+testID+"/results/"+resultID+"/evidence",
		`{"evidence_id":"`+evID+`",`+envelope+`,"qualifier":"`+forgedID+`","verb":"mitigated_by"}`)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	assert.Equal(t, "openrisk_user", body["source"])
	assert.EqualValues(t, 0.7, body["confidence"])
	assert.Equal(t, map[string]any{"method": "manual_entry", "actor": map[string]any{"kind": "user", "id": s.userA.String()}, "confidence_rule": "confidence/1"}, body["provenance"])
	var edge controlchain.CanonicalEdgeRow
	require.NoError(t, s.db.Where("from_id = ? AND verb = ?", testID, "evidenced_by").Take(&edge).Error)
	assert.Equal(t, s.tenantA, edge.TenantID)
	assert.Equal(t, resultID, edge.Qualifier.String(), "the qualifier is the path's result, not the body's")
	assert.Equal(t, "openrisk_user", edge.Source)
	assert.Equal(t, 0.7, edge.Confidence)
	assert.JSONEq(t, `{}`, edge.Attributes)

	// The pending flag cannot be set by a request, on a not_applicable control either.
	status, body = s.do(t, a, http.MethodPost, base, `{"name":"NA","applicability":"not_applicable","applicability_review_pending":true}`)
	assert.Equal(t, http.StatusBadRequest, status, "the flag is not a way around the justification: %v", body)
	assert.Contains(t, body["error"], "applicability_justification")
}

// AC5 through the routes: not_applicable without a justification is a 400, a
// pending control (made by the split) accepts none, and clears on the first one.
func TestInternalControlRoutes_ApplicabilityRules(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	base := "/api/v1/compliance/internal-controls"

	status, _ := s.do(t, a, http.MethodPost, base, `{"name":"x","applicability":"not_applicable"}`)
	assert.Equal(t, http.StatusBadRequest, status)

	// A split control of a not_applicable requirement, as Reconcile writes it.
	req := uuid.New()
	split, _ := controlchain.SplitControl(controlchain.RequirementRow{
		ID: req, TenantID: s.tenantA, Name: "A.10.1", Status: "not_applicable", CreatedAt: intCtlNow, UpdatedAt: intCtlNow,
	}, canonical.MethodManualEntry)
	row, err := controlchain.ControlToRow(split, nil)
	require.NoError(t, err)
	require.NoError(t, s.db.Create(&row).Error)
	id := split.ID.String()

	status, body := s.do(t, a, http.MethodPatch, base+"/"+id, `{"tags":["review"]}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, true, body["applicability_review_pending"], "an edit without a justification keeps the flag")

	status, body = s.do(t, a, http.MethodPatch, base+"/"+id, `{"applicability_justification":"no card data is stored"}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, false, body["applicability_review_pending"])
	status, body = s.do(t, a, http.MethodGet, base+"/"+id, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, body["applicability_review_pending"], "persisted")

	// The split control is not deletable here, and a pending-free one needs its justification again.
	status, body = s.do(t, a, http.MethodDelete, base+"/"+id, "")
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, domain.IntCtlSplitControl, body["error"])
	status, _ = s.do(t, a, http.MethodPatch, base+"/"+id, `{"applicability_justification":""}`)
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestInternalControlRoutes_EvidenceRequiredAndConflicts(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	base := "/api/v1/compliance/internal-controls"

	status, body := s.do(t, a, http.MethodPost, base, `{"name":"x","declared_status":"implemented"}`)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "evidence_required", body["error"])

	id := s.createControl(t, a, "x")
	status, body = s.do(t, a, http.MethodPatch, base+"/"+id, `{"declared_status":"implemented"}`)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "evidence_required", body["error"])

	// A split control's own link to its requirement is not unlinkable.
	req := s.requirement(t, s.tenantA, "REQ")
	split, _ := controlchain.SplitControl(controlchain.RequirementRow{ID: req, TenantID: s.tenantA, Name: "REQ", Status: "not_implemented", CreatedAt: intCtlNow, UpdatedAt: intCtlNow}, canonical.MethodManualEntry)
	srow, err := controlchain.ControlToRow(split, nil)
	require.NoError(t, err)
	require.NoError(t, s.db.Create(&srow).Error)
	status, _ = s.do(t, s.asB(), http.MethodDelete, base+"/"+split.ID.String()+"/requirements/"+req.String(), "")
	assert.Equal(t, http.StatusNotFound, status, "another tenant's split pair is 404, not 409")
	status, body = s.do(t, a, http.MethodDelete, base+"/"+split.ID.String()+"/requirements/"+req.String(), "")
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, domain.IntCtlSplitLink, body["error"])

	// Bad ids and bodies are 400s.
	status, _ = s.do(t, a, http.MethodGet, base+"/not-a-uuid", "")
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = s.do(t, a, http.MethodPost, base+"/"+id+"/requirements/not-a-uuid", "")
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = s.do(t, a, http.MethodPost, base, `not json`)
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = s.do(t, a, http.MethodGet, base+"?limit=abc", "")
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = s.do(t, a, http.MethodGet, base+"?limit=501", "")
	assert.Equal(t, http.StatusBadRequest, status)
}

// QA #1414: a soft-deleted control or requirement is absent on every route that
// names it (A10.3), not only on the two reads: a stale id must never 200, never
// link, never edit.
func TestInternalControlRoutes_SoftDeletedEndsAreNotFound(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	base := "/api/v1/compliance/internal-controls"
	req := s.requirement(t, s.tenantA, "REQ")
	gone := s.requirement(t, s.tenantA, "GON")
	ctl := s.createControl(t, a, "kept")
	deleted := s.createControl(t, a, "deleted")
	for _, r := range []uuid.UUID{req, gone} {
		status, _ := s.do(t, a, http.MethodPost, base+"/"+ctl+"/requirements/"+r.String(), "")
		require.Equal(t, http.StatusCreated, status)
	}
	status, _ := s.do(t, a, http.MethodPost, base+"/"+deleted+"/requirements/"+req.String(), "")
	require.Equal(t, http.StatusCreated, status)

	// Delete the control through the API; soft-delete the requirement under it.
	status, _ = s.do(t, a, http.MethodDelete, base+"/"+deleted, "")
	require.Equal(t, http.StatusNoContent, status)
	require.NoError(t, s.db.Exec("UPDATE compliance_controls SET deleted_at = ? WHERE id = ?", intCtlNow, gone).Error)

	for name, c := range map[string]struct{ method, path, body string }{
		"get deleted control":        {http.MethodGet, base + "/" + deleted, ""},
		"patch deleted control":      {http.MethodPatch, base + "/" + deleted, `{"name":"zombie"}`},
		"delete deleted control":     {http.MethodDelete, base + "/" + deleted, ""},
		"requirements of deleted":    {http.MethodGet, base + "/" + deleted + "/requirements", ""},
		"link deleted control":       {http.MethodPost, base + "/" + deleted + "/requirements/" + req.String(), ""},
		"unlink deleted control":     {http.MethodDelete, base + "/" + deleted + "/requirements/" + req.String(), ""},
		"link to deleted req":        {http.MethodPost, base + "/" + ctl + "/requirements/" + gone.String(), ""},
		"controls of deleted req":    {http.MethodGet, "/api/v1/compliance/controls/" + gone.String() + "/internal-controls", ""},
		"unknown control id":         {http.MethodGet, base + "/" + uuid.NewString(), ""},
		"link to unknown req":        {http.MethodPost, base + "/" + ctl + "/requirements/" + uuid.NewString(), ""},
		"unlink an edge never made":  {http.MethodDelete, base + "/" + ctl + "/requirements/" + uuid.NewString(), ""},
		"unknown requirement's list": {http.MethodGet, "/api/v1/compliance/controls/" + uuid.NewString() + "/internal-controls", ""},
	} {
		st, body := s.do(t, a, c.method, c.path, c.body)
		assert.Equal(t, http.StatusNotFound, st, name)
		assert.NotContains(t, body, "data", name)
	}

	// The requirement soft-deleted since the link drops out of the control's list.
	st, body := s.do(t, a, http.MethodGet, base+"/"+ctl+"/requirements", "")
	require.Equal(t, http.StatusOK, st)
	assert.Len(t, body["data"], 1)
	// The control deleted since the link drops out of the requirement's list.
	st, body = s.do(t, a, http.MethodGet, "/api/v1/compliance/controls/"+req.String()+"/internal-controls", "")
	require.Equal(t, http.StatusOK, st)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, ctl, data[0].(map[string]any)["id"])
	st, body = s.do(t, a, http.MethodGet, base, "")
	require.Equal(t, http.StatusOK, st)
	assert.Len(t, body["data"], 1, "the list hides the deleted control")
	// The refused write did not resurrect it.
	var row controlchain.ControlRow
	require.NoError(t, s.db.Unscoped().Where("id = ?", deleted).Take(&row).Error)
	assert.NotNil(t, row.DeletedAt)
	assert.Equal(t, "deleted", row.Name)
}

// QA #1414 (D-116 points 4 and 5): another tenant asking for a split control gets
// 404 from delete, never the 409 that would confirm the control exists.
func TestInternalControlRoutes_SplitControlOfAnotherTenantIsNotFound(t *testing.T) {
	s := newIntCtlStack(t)
	base := "/api/v1/compliance/internal-controls"
	req := s.requirement(t, s.tenantA, "REQ")
	split, _ := controlchain.SplitControl(controlchain.RequirementRow{ID: req, TenantID: s.tenantA, Name: "REQ", Status: "not_implemented", CreatedAt: intCtlNow, UpdatedAt: intCtlNow}, canonical.MethodManualEntry)
	row, err := controlchain.ControlToRow(split, nil)
	require.NoError(t, err)
	require.NoError(t, s.db.Create(&row).Error)

	status, _ := s.do(t, s.asA(intCtlAllPerms), http.MethodDelete, base+"/"+split.ID.String(), "")
	require.Equal(t, http.StatusConflict, status, "the owner sees the split refusal")

	status, body := s.do(t, s.asB(), http.MethodDelete, base+"/"+split.ID.String(), "")
	assert.Equal(t, http.StatusNotFound, status, "%v", body)
	status, _ = s.do(t, s.asB(), http.MethodPatch, base+"/"+split.ID.String(), `{"tags":["x"]}`)
	assert.Equal(t, http.StatusNotFound, status)
}
