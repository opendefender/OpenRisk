// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/application/entity"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// The universal drawer through the real HTTP stack.
//
// The application-level suite (internal/application/entity) proves the service's
// decisions against in-memory stores. This one proves the wiring: the real
// handler, the real Gorm repositories, real SQL, and the real permission locals
// the auth middleware sets — because a service that refuses a cross-tenant read
// is worth nothing if the handler hands it the wrong tenant.

type drawerFixture struct {
	app              *fiber.App
	db               *gorm.DB
	tenantA, tenantB uuid.UUID
	userA            uuid.UUID

	assetA, assetB *domain.Asset
	riskA, riskB   *domain.Risk
	vulnA          *domain.Vulnerability
	controlA       *domain.ComplianceControl
	incidentA      *domain.Incident
	incidentB      *domain.Incident
	evidenceA      *domain.Evidence
}

// setupDrawer stands the whole stack up on an in-memory database, migrated from
// the SAME models main.go migrates — which is how this test's schema stops
// drifting away from production's.
func setupDrawer(t *testing.T, permissions []string) *drawerFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	// Several models carry Postgres-only DDL — a gen_random_uuid() default, a
	// text[] column — that GORM cannot emit for sqlite. testsupport/sqliteschema
	// exists for exactly this: create the minimal table sqlite needs, then take
	// every remaining column off the model, so this fixture cannot drift away
	// from the schema production runs.
	for _, tbl := range []struct {
		name  string
		model any
	}{
		{"assets", &domain.Asset{}},
		{"asset_dependencies", &domain.AssetDependency{}},
		{"asset_snapshots", &domain.AssetSnapshot{}},
		{"risks", &domain.Risk{}},
		{"risk_histories", &domain.RiskHistory{}},
		{"risk_control_mappings", &domain.RiskControlMapping{}},
		{"vulnerabilities", &domain.Vulnerability{}},
		{"compliance_frameworks", &domain.ComplianceFramework{}},
		{"compliance_controls", &domain.ComplianceControl{}},
		{"mitigations", &domain.Mitigation{}},
	} {
		require.NoError(t, db.Exec("CREATE TABLE "+tbl.name+" (id TEXT PRIMARY KEY);").Error)
		require.NoError(t, sqliteschema.Reconcile(db, tbl.name, tbl.model))
	}
	// risk_assets is a GORM many2many join table with no model of its own.
	require.NoError(t, db.Exec(`
		CREATE TABLE risk_assets (risk_id TEXT NOT NULL, asset_id TEXT NOT NULL);
	`).Error)
	// The rest carry no Postgres-only DDL and migrate straight off the models.
	require.NoError(t, db.AutoMigrate(
		&domain.Evidence{}, &domain.EvidenceControlLink{},
		&domain.Incident{}, &domain.IncidentTimeline{}, &domain.IncidentAction{},
		&domain.AuditEvent{},
	))

	f := &drawerFixture{db: db, tenantA: uuid.New(), tenantB: uuid.New(), userA: uuid.New()}
	now := time.Now().UTC()

	f.assetA = &domain.Asset{
		ID: uuid.New(), TenantID: f.tenantA, Name: "web-prod-01", Type: "Server",
		Criticality: domain.CriticalityCritical, Category: domain.CategoryServer,
		CreatedAt: now, UpdatedAt: now,
	}
	f.assetB = &domain.Asset{
		ID: uuid.New(), TenantID: f.tenantB, Name: "other-tenant-host", Type: "Server",
		Criticality: domain.CriticalityHigh, Category: domain.CategoryServer,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(f.assetA).Error)
	require.NoError(t, db.Create(f.assetB).Error)

	f.riskA = &domain.Risk{
		ID: uuid.New(), TenantID: f.tenantA, Name: "Log4Shell exposure", Title: "Log4Shell exposure",
		Probability: 0.9, Impact: 9.5, Score: 25.65, Criticality: domain.CriticalityCriticalNew,
		Status: domain.RiskStatus("open"), AssetID: &f.assetA.ID, CreatedBy: f.userA,
		CreatedAt: now, UpdatedAt: now,
	}
	f.riskB = &domain.Risk{
		ID: uuid.New(), TenantID: f.tenantB, Name: "Other tenant risk", Title: "Other tenant risk",
		Score: 4, Criticality: domain.CriticalityLowNew, Status: domain.RiskStatus("open"),
		AssetID: &f.assetB.ID, CreatedBy: uuid.New(), CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(f.riskA).Error)
	require.NoError(t, db.Create(f.riskB).Error)

	f.vulnA = &domain.Vulnerability{
		ID: uuid.New(), TenantID: f.tenantA, CVEID: "CVE-2021-44228", Title: "Log4j RCE",
		CVSSScore: 10, Severity: domain.VulnSeverityCritical, KEV: true,
		PriorityScore: 92.5, PriorityTier: "P1", Status: domain.VulnStatus("open"),
		Source: domain.VulnSourceNessus, AssetID: &f.assetA.ID, RiskID: &f.riskA.ID,
		FirstSeen: now, LastSeen: now,
	}
	require.NoError(t, db.Create(f.vulnA).Error)

	framework := &domain.ComplianceFramework{
		ID: uuid.New(), TenantID: f.tenantA, Name: "ISO/IEC 27001", Version: "2022",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(framework).Error)
	f.controlA = &domain.ComplianceControl{
		ID: uuid.New(), TenantID: f.tenantA, FrameworkID: framework.ID,
		ReferenceCode: "A.5.1", Name: "Policies for information security",
		Status: domain.ControlStatusImplemented, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(f.controlA).Error)

	// A real risk → control mapping, so the relation query has something to find.
	require.NoError(t, db.Create(&domain.RiskControlMapping{
		ID: uuid.New(), TenantID: f.tenantA, RiskID: f.riskA.ID,
		FrameworkID: framework.ID, ControlID: &f.controlA.ID,
		CreatedAt: now, UpdatedAt: now,
	}).Error)

	f.evidenceA = &domain.Evidence{
		ID: uuid.New(), TenantID: f.tenantA, Title: "Q1 access review",
		Type: domain.EvidenceTypeDocument, Review: domain.EvidenceReview("accepted"),
		Source: domain.EvidenceSourceManual, CollectedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(f.evidenceA).Error)
	require.NoError(t, db.Create(&domain.EvidenceControlLink{
		TenantID: f.tenantA, EvidenceID: f.evidenceA.ID, ControlID: f.controlA.ID, CreatedAt: now,
	}).Error)

	f.incidentA = &domain.Incident{
		TenantID: f.tenantA.String(), Title: "Phishing campaign", IncidentType: "phishing",
		Severity: "high", Status: "investigating", Origin: "manual",
		RiskIDs: domain.StringList{f.riskA.ID.String()}, AssetIDs: domain.StringList{f.assetA.ID.String()},
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(f.incidentA).Error)
	f.incidentB = &domain.Incident{
		TenantID: f.tenantB.String(), Title: "Other tenant incident",
		Severity: "low", Status: "open", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(f.incidentB).Error)

	// One audit row per tenant, on the SAME entity id, so a leaking query would
	// visibly return two.
	for _, tenant := range []uuid.UUID{f.tenantA, f.tenantB} {
		require.NoError(t, db.Create(&domain.AuditEvent{
			ID: uuid.New(), TenantID: tenant, Action: domain.AuditActionUpdate,
			EntityType: "risk", EntityID: f.riskA.ID.String(),
			Summary: "Updated risk", CreatedAt: now, Sequence: 1,
		}).Error)
	}

	relations := repository.NewGormEntityRelationRepository(db)
	timeline := entity.NewTimelineService(repository.NewGormAuditEventRepository(db)).
		WithSource(entity.TypeRisk, repository.NewRiskHistorySource(db)).
		WithSource(entity.TypeIncident, repository.NewIncidentTimelineSource(db)).
		WithSource(entity.TypeAsset, repository.NewAssetSnapshotSource(db))

	registry := entity.NewRegistry().
		Register(entity.TypeAsset, entity.NewAssetResolver(repository.NewGormAssetRepository(db), relations)).
		Register(entity.TypeVendor, entity.NewVendorResolver(repository.NewGormAssetRepository(db), relations)).
		Register(entity.TypeRisk, entity.NewRiskResolver(repository.NewGormRiskRepository(db), relations)).
		Register(entity.TypeVulnerability, entity.NewVulnerabilityResolver(repository.NewGormVulnerabilityRepository(db), relations)).
		Register(entity.TypeFinding, entity.NewFindingResolver(repository.NewGormVulnerabilityRepository(db), relations)).
		Register(entity.TypeControl, entity.NewControlResolver(repository.NewGormComplianceRepository(db), relations)).
		Register(entity.TypeIncident, entity.NewIncidentResolver(service.NewIncidentService(db), relations)).
		Register(entity.TypeEvidence, entity.NewEvidenceResolver(repository.NewGormEvidenceRepository(db), relations))

	h := NewEntityHandler(entity.NewService(registry).WithTimeline(timeline))

	app := fiber.New()
	// Stand in for the auth middleware: the same locals, set the same way.
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("permissions", permissions)
		c.Locals("tenant_id", f.tenantA)
		c.Locals("user_id", f.userA)
		middleware.SetContext(c, &middleware.RequestContext{UserID: f.userA, OrganizationID: f.tenantA})
		return c.Next()
	})
	app.Get("/timeline", h.GetTenantTimeline)
	app.Get("/entities", h.GetCatalogue)
	app.Get("/entities/:type/:id", h.GetEntity)
	app.Get("/entities/:type/:id/relations", h.GetRelations)
	app.Get("/entities/:type/:id/timeline", h.GetTimeline)
	app.Get("/entities/:type/:id/audit", h.GetAudit)
	f.app = app
	return f
}

func (f *drawerFixture) get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]any
	_ = json.Unmarshal(body, &out)
	return resp.StatusCode, out
}

// ---------------------------------------------------------------------------
// Every type resolves through the real stack
// ---------------------------------------------------------------------------

func TestEntityDrawer_ResolvesEveryTypeOverHTTP(t *testing.T) {
	f := setupDrawer(t, []string{"*"})

	cases := []struct{ typ, id, wantTitle string }{
		{"asset", f.assetA.ID.String(), "web-prod-01"},
		{"risk", f.riskA.ID.String(), "Log4Shell exposure"},
		{"vulnerability", f.vulnA.ID.String(), "Log4j RCE"},
		{"finding", f.vulnA.ID.String(), "Log4j RCE"},
		{"control", f.controlA.ID.String(), "Policies for information security"},
		{"incident", "1", "Phishing campaign"},
		{"evidence", f.evidenceA.ID.String(), "Q1 access review"},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			status, body := f.get(t, "/entities/"+tc.typ+"/"+tc.id)
			require.Equal(t, http.StatusOK, status, "body: %v", body)
			summary, ok := body["summary"].(map[string]any)
			require.True(t, ok, "no summary in %v", body)
			require.Equal(t, tc.wantTitle, summary["title"])
		})
	}
}

func TestEntityDrawer_UnknownTypeIs400(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	status, _ := f.get(t, "/entities/employee/"+uuid.New().String())
	require.Equal(t, http.StatusBadRequest, status)
}

// ---------------------------------------------------------------------------
// Tenant isolation over the real stack — the registry's Covered claim
// ---------------------------------------------------------------------------

func TestEntityDrawer_CrossTenantAccessDenied(t *testing.T) {
	f := setupDrawer(t, []string{"*"})

	// Real ids that belong to tenant B, requested by a tenant A session.
	for _, tc := range []struct{ name, path string }{
		{"asset", "/entities/asset/" + f.assetB.ID.String()},
		{"risk", "/entities/risk/" + f.riskB.ID.String()},
		{"incident", "/entities/incident/2"},
		{"asset relations", "/entities/asset/" + f.assetB.ID.String() + "/relations"},
		{"risk timeline", "/entities/risk/" + f.riskB.ID.String() + "/timeline"},
		{"risk audit", "/entities/risk/" + f.riskB.ID.String() + "/audit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, body := f.get(t, tc.path)
			require.Equal(t, http.StatusNotFound, status, "body: %v", body)
		})
	}
}

// A relation list must contain only the caller's tenant's rows. This is the
// query-level proof: both tenants own an asset and a risk, and the SQL runs for
// real.
func TestEntityDrawer_RelationsAreTenantScoped(t *testing.T) {
	f := setupDrawer(t, []string{"*"})

	status, body := f.get(t, "/entities/asset/"+f.assetA.ID.String()+"/relations")
	require.Equal(t, http.StatusOK, status)

	groups, ok := body["groups"].([]any)
	require.True(t, ok)

	var sawRisk bool
	for _, raw := range groups {
		g := raw.(map[string]any)
		for _, rawItem := range g["items"].([]any) {
			item := rawItem.(map[string]any)
			require.NotEqual(t, f.riskB.ID.String(), item["id"],
				"a relation named the other tenant's risk")
			require.NotEqual(t, f.assetB.ID.String(), item["id"],
				"a relation named the other tenant's asset")
			if g["key"] == "risks" {
				sawRisk = true
				require.Equal(t, f.riskA.ID.String(), item["id"])
			}
		}
	}
	require.True(t, sawRisk, "the asset's own risk was not returned; the relation query is not finding real edges")
}

// The audit trail must return only the caller's tenant's row, even though both
// tenants have one against the SAME entity id.
func TestEntityDrawer_AuditIsTenantScoped(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	status, body := f.get(t, "/entities/risk/"+f.riskA.ID.String()+"/audit")
	require.Equal(t, http.StatusOK, status)

	events := body["events"].([]any)
	require.Len(t, events, 1, "the other tenant's audit row surfaced")
	require.Equal(t, float64(1), body["total"])
}

// ---------------------------------------------------------------------------
// Permissions over the real stack
// ---------------------------------------------------------------------------

func TestEntityDrawer_PermissionsGateEachType(t *testing.T) {
	f := setupDrawer(t, []string{"risks:read"})

	status, _ := f.get(t, "/entities/risk/"+f.riskA.ID.String())
	require.Equal(t, http.StatusOK, status, "risks:read should open a risk")

	status, _ = f.get(t, "/entities/asset/"+f.assetA.ID.String())
	require.Equal(t, http.StatusForbidden, status, "risks:read must not open an asset")
}

func TestEntityDrawer_AuditNeedsItsOwnPermission(t *testing.T) {
	f := setupDrawer(t, []string{"risks:read"})
	status, _ := f.get(t, "/entities/risk/"+f.riskA.ID.String()+"/audit")
	require.Equal(t, http.StatusForbidden, status)

	// And the audit section is not even offered.
	_, body := f.get(t, "/entities/risk/"+f.riskA.ID.String())
	for _, s := range body["sections"].([]any) {
		require.NotEqual(t, "audit", s, "an audit tab was offered that would always 403")
	}
}

func TestEntityDrawer_DeniedRelationGroupIsEmptyAndLabelled(t *testing.T) {
	f := setupDrawer(t, []string{"assets:read"}) // no risks:read

	status, body := f.get(t, "/entities/asset/"+f.assetA.ID.String()+"/relations")
	require.Equal(t, http.StatusOK, status)

	for _, raw := range body["groups"].([]any) {
		g := raw.(map[string]any)
		if g["key"] != "risks" {
			continue
		}
		require.Equal(t, true, g["denied"])
		require.Empty(t, g["items"])
		require.Equal(t, float64(0), g["total"], "a denied group leaked its count")
		return
	}
	t.Fatal("no risks group returned")
}

// ---------------------------------------------------------------------------
// Timeline over the real stack
// ---------------------------------------------------------------------------

func TestEntityDrawer_TimelineReadsTheCanonicalTrail(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	status, body := f.get(t, "/entities/risk/"+f.riskA.ID.String()+"/timeline")
	require.Equal(t, http.StatusOK, status)

	events := body["events"].([]any)
	require.NotEmpty(t, events, "the timeline read nothing")

	// The trail entry seeded for THIS tenant must be there. (The risk's own
	// AfterSave hook also writes a risk_histories row, so this stream is already
	// two sources deep — which is the point of the merge, and is asserted
	// separately below.)
	var sawAudit bool
	for _, raw := range events {
		ev := raw.(map[string]any)
		if ev["source"] == "audit" {
			sawAudit = true
			require.Equal(t, "update", ev["kind"])
			require.Equal(t, "Updated risk", ev["summary"])
		}
		require.Equal(t, f.riskA.ID.String(), ev["target"].(map[string]any)["id"],
			"an event about a different risk appeared")
	}
	require.True(t, sawAudit, "the canonical trail did not reach the timeline")

	// Newest first, everywhere (§20).
	for i := 1; i < len(events); i++ {
		prev := events[i-1].(map[string]any)["occurred_at"].(string)
		cur := events[i].(map[string]any)["occurred_at"].(string)
		require.LessOrEqual(t, cur, prev, "the feed is not newest-first")
	}
}

// A risk's score movements come from risk_histories, which the trail never sees
// because the score worker runs outside any request.
func TestEntityDrawer_TimelineMergesTheScoreJournal(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	require.NoError(t, f.db.Create(&domain.RiskHistory{
		ID: uuid.New(), RiskID: f.riskA.ID, Score: 25.65,
		Status: domain.RiskStatus("open"), ChangedBy: "System", ChangeType: "UPDATE",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
	}).Error)

	status, body := f.get(t, "/entities/risk/"+f.riskA.ID.String()+"/timeline")
	require.Equal(t, http.StatusOK, status)

	var sawJournal bool
	for _, raw := range body["events"].([]any) {
		if raw.(map[string]any)["source"] == "risk_history" {
			sawJournal = true
		}
	}
	require.True(t, sawJournal, "the score journal did not reach the timeline")
}

// The journals carry no tenant column of their own, so they are gated through
// their parent. This proves the gate with a history row planted under a risk the
// caller cannot read.
func TestEntityDrawer_ScoreJournalIsGatedByItsParentRisk(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	require.NoError(t, f.db.Create(&domain.RiskHistory{
		ID: uuid.New(), RiskID: f.riskB.ID, Score: 99, ChangedBy: "System",
		ChangeType: "UPDATE", CreatedAt: time.Now().UTC(),
	}).Error)

	// Reading the other tenant's risk is refused outright.
	status, _ := f.get(t, "/entities/risk/"+f.riskB.ID.String()+"/timeline")
	require.Equal(t, http.StatusNotFound, status)

	// And the caller's own risk timeline does not pick it up.
	_, body := f.get(t, "/entities/risk/"+f.riskA.ID.String()+"/timeline")
	for _, raw := range body["events"].([]any) {
		ev := raw.(map[string]any)
		require.NotContains(t, ev["summary"], "99", "the other risk's history bled into this timeline")
	}
}

func TestEntityDrawer_TenantTimelineIsScopedAndLinked(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	status, body := f.get(t, "/timeline")
	require.Equal(t, http.StatusOK, status)

	events := body["events"].([]any)
	require.Len(t, events, 1, "the tenant feed crossed a tenant boundary")
	ev := events[0].(map[string]any)
	require.Equal(t, "/risks?drawer=risk&entity="+f.riskA.ID.String(), ev["target_url"])
}

func TestEntityDrawer_TenantTimelineFiltersByPermission(t *testing.T) {
	f := setupDrawer(t, []string{"assets:read"}) // no risks:read, no audit permission
	status, body := f.get(t, "/timeline")
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, body["events"], "a risk event reached a caller who may not read risks")
}

func TestEntityDrawer_MalformedCursorIs400(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	status, _ := f.get(t, "/entities/risk/"+f.riskA.ID.String()+"/timeline?cursor=%21%21%21")
	require.Equal(t, http.StatusBadRequest, status)
}

func TestEntityDrawer_MalformedFilterIs400(t *testing.T) {
	f := setupDrawer(t, []string{"*"})
	status, _ := f.get(t, "/timeline?actor_id=not-a-uuid")
	require.Equal(t, http.StatusBadRequest, status,
		"an unparseable filter must not be silently ignored — the caller would read the unfiltered result as filtered")
}

// ---------------------------------------------------------------------------
// The audit trail names what was created
// ---------------------------------------------------------------------------

// A collection POST names no id in its route, and a model that is not Auditable
// is never observed by the row layer either — so a creation used to be
// journalled with an empty entity_id, and an entity's own audit tab could never
// show its creation.
//
// This drives the middleware directly rather than through the drawer, because
// the defect is in what gets WRITTEN; the drawer only made it visible.
func TestAuditMutations_RecordsTheCreatedRecordsID(t *testing.T) {
	f := setupDrawer(t, []string{"*"})

	appender := repository.NewGormAuditChainRepository(f.db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: f.userA, OrganizationID: f.tenantA})
		return c.Next()
	})
	app.Use(middleware.AuditMutations(appender))
	// A handler shaped like every create in this codebase: 201 with the new
	// record as JSON.
	app.Post("/api/v1/things", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": "created-id-42", "name": "a thing"})
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/things", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	events, _, err := appender.List(context.Background(), f.tenantA, domain.AuditEventFilter{
		EntityType: "thing", Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "created-id-42", events[0].EntityID,
		"the trail recorded a creation without saying what was created")
	require.Equal(t, domain.AuditActionCreate, events[0].Action)
}

// ---------------------------------------------------------------------------
// Catalogue
// ---------------------------------------------------------------------------

func TestEntityDrawer_CatalogueMarksReadableTypes(t *testing.T) {
	f := setupDrawer(t, []string{"risks:read"})
	status, body := f.get(t, "/entities")
	require.Equal(t, http.StatusOK, status)

	types := body["types"].([]any)
	require.Len(t, types, 8)
	for _, raw := range types {
		e := raw.(map[string]any)
		require.Equal(t, e["type"] == "risk", e["readable"], "type %v", e["type"])
	}
}
