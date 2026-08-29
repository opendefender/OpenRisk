// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// world builds a two-tenant fixture with one entity of every type in EACH
// tenant. Two tenants is not decoration: a test that only ever holds one
// tenant's data cannot tell a correct predicate from a missing one, because both
// return the same rows.
type world struct {
	tb        testing.TB
	svc       *Service
	relations *fakeRelations
	audit     *fakeAudit

	tenantA, tenantB uuid.UUID

	registry *Registry

	assetA, assetB       *domain.Asset
	vendorA, vendorB     *domain.Asset
	riskA, riskB         *domain.Risk
	vulnA, vulnB         *domain.Vulnerability
	controlA, controlB   *domain.ComplianceControl
	incidentA, incidentB *domain.Incident
	evidenceA, evidenceB *domain.Evidence
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{tb: t, tenantA: uuid.New(), tenantB: uuid.New()}

	assets := newFakeAssets()
	risks := newFakeRisks()
	vulns := newFakeVulns()
	controls := newFakeControls()
	incidents := newFakeIncidents()
	evidences := newFakeEvidence()
	w.relations = newFakeRelations()
	w.audit = &fakeAudit{}

	now := time.Now().UTC()

	w.assetA = assets.add(&domain.Asset{
		TenantID: w.tenantA, Name: "web-prod-01", Type: "Server",
		Criticality: domain.CriticalityCritical, Category: domain.CategoryServer,
		Owner: "ops@a.example", CreatedAt: now, UpdatedAt: now,
	})
	w.assetB = assets.add(&domain.Asset{
		TenantID: w.tenantB, Name: "other-tenant-host", Type: "Server",
		Criticality: domain.CriticalityHigh, Category: domain.CategoryServer,
		CreatedAt: now, UpdatedAt: now,
	})
	w.vendorA = assets.add(&domain.Asset{
		TenantID: w.tenantA, Name: "Acme Hosting", Type: "Supplier",
		Criticality: domain.CriticalityHigh, Category: domain.CategoryVendor,
		CreatedAt: now, UpdatedAt: now,
	})
	// A REAL vendor in tenant B, not merely another tenant's server.
	//
	// This matters more than it looks. The vendor fixture used to point at
	// assetB, a Server, so a cross-tenant vendor read answered 404 for two
	// independent reasons — wrong tenant AND wrong category — and the test
	// would still have passed with the tenant predicate removed entirely. A
	// test that cannot fail for the reason it exists to check is not coverage.
	w.vendorB = assets.add(&domain.Asset{
		TenantID: w.tenantB, Name: "Other tenant supplier", Type: "Supplier",
		Criticality: domain.CriticalityHigh, Category: domain.CategoryVendor,
		CreatedAt: now, UpdatedAt: now,
	})

	w.riskA = risks.add(&domain.Risk{
		TenantID: w.tenantA, Name: "Log4Shell exposure", Description: "RCE on a public host",
		Probability: 0.9, Impact: 9.5, Score: 25.65,
		Criticality: domain.CriticalityCriticalNew, Status: domain.RiskStatus("open"),
		LifecycleState: domain.RiskState("IDENTIFIED"),
		CreatedAt:      now, UpdatedAt: now,
	})
	w.riskB = risks.add(&domain.Risk{
		TenantID: w.tenantB, Name: "Other tenant risk", Score: 4,
		Criticality: domain.CriticalityLowNew, Status: domain.RiskStatus("open"),
		CreatedAt: now, UpdatedAt: now,
	})

	w.vulnA = vulns.add(&domain.Vulnerability{
		TenantID: w.tenantA, CVEID: "CVE-2021-44228", Title: "Log4j RCE",
		CVSSScore: 10, Severity: domain.VulnSeverityCritical, KEV: true,
		PriorityScore: 92.5, PriorityTier: "P1", Status: domain.VulnStatus("open"),
		Source: domain.VulnSourceNessus, FirstSeen: now, LastSeen: now,
	})
	w.vulnB = vulns.add(&domain.Vulnerability{
		TenantID: w.tenantB, CVEID: "CVE-2020-0001", Title: "Other tenant finding",
		Severity: domain.VulnSeverityLow, PriorityTier: "P4", FirstSeen: now, LastSeen: now,
	})

	w.controlA = controls.add(&domain.ComplianceControl{
		TenantID: w.tenantA, ReferenceCode: "A.5.1", Name: "Policies for information security",
		Status: domain.ControlStatusImplemented, EvidenceCount: 2, CreatedAt: now, UpdatedAt: now,
	})
	w.controlB = controls.add(&domain.ComplianceControl{
		TenantID: w.tenantB, ReferenceCode: "A.5.1", Name: "Other tenant control",
		Status: domain.ControlStatusNotImplemented, CreatedAt: now, UpdatedAt: now,
	})

	w.incidentA = incidents.add(&domain.Incident{
		TenantID: w.tenantA.String(), Title: "Phishing campaign", IncidentType: "phishing",
		Severity: "high", Status: "investigating", Origin: "manual", CreatedAt: now, UpdatedAt: now,
	})
	w.incidentB = incidents.add(&domain.Incident{
		TenantID: w.tenantB.String(), Title: "Other tenant incident",
		Severity: "low", Status: "open", CreatedAt: now, UpdatedAt: now,
	})

	w.evidenceA = evidences.add(&domain.Evidence{
		TenantID: w.tenantA, Title: "Q1 access review", Type: domain.EvidenceTypeDocument,
		Review: domain.EvidenceReview("accepted"), Status: domain.EvidenceStatusValid,
		Source: domain.EvidenceSourceManual, CollectedAt: now, CreatedAt: now, UpdatedAt: now,
	})
	w.evidenceB = evidences.add(&domain.Evidence{
		TenantID: w.tenantB, Title: "Other tenant proof", Status: domain.EvidenceStatusValid,
		CollectedAt: now, CreatedAt: now, UpdatedAt: now,
	})

	registry := NewRegistry().
		Register(Bind(TypeAsset, NewAssetResolver(assets, w.relations))).
		Register(Bind(TypeVendor, NewVendorResolver(assets, w.relations))).
		Register(Bind(TypeRisk, NewRiskResolver(risks, w.relations))).
		Register(Bind(TypeVulnerability, NewVulnerabilityResolver(vulns, w.relations))).
		Register(Bind(TypeFinding, NewFindingResolver(vulns, w.relations))).
		Register(Bind(TypeControl, NewControlResolver(controls, w.relations))).
		Register(Bind(TypeIncident, NewIncidentResolver(incidents, w.relations))).
		Register(Bind(TypeEvidence, NewEvidenceResolver(evidences, w.relations)))

	w.registry = registry
	w.svc = NewService(registry).WithTimeline(NewTimelineService(w.audit))
	return w
}

// allPerms is an admin.
func (w *world) admin(tenant uuid.UUID) Caller { return callerIn(tenant, "*") }

// typeFixture is one type's seeded row in each tenant.
type typeFixture struct {
	t      Type
	idA    string
	idB    string
	pretty string
}

// fixtures is the seeded data, keyed by type. It is NOT the list of cases any
// test iterates — idsOfEveryType derives that from the registry, so a type that
// is registered but absent here is a failure rather than a silent gap.
func (w *world) fixtures() map[Type]typeFixture {
	return map[Type]typeFixture{
		TypeAsset:         {TypeAsset, w.assetA.ID.String(), w.assetB.ID.String(), "asset"},
		TypeVendor:        {TypeVendor, w.vendorA.ID.String(), w.vendorB.ID.String(), "vendor"},
		TypeRisk:          {TypeRisk, w.riskA.ID.String(), w.riskB.ID.String(), "risk"},
		TypeVulnerability: {TypeVulnerability, w.vulnA.ID.String(), w.vulnB.ID.String(), "vulnerability"},
		TypeFinding:       {TypeFinding, w.vulnA.ID.String(), w.vulnB.ID.String(), "finding"},
		TypeControl:       {TypeControl, w.controlA.ID.String(), w.controlB.ID.String(), "control"},
		TypeIncident:      {TypeIncident, "1", "2", "incident"},
		TypeEvidence:      {TypeEvidence, w.evidenceA.ID.String(), w.evidenceB.ID.String(), "evidence"},
	}
}

// idsOfEveryType returns one case per REGISTERED type, in registry order.
//
// It is derived from Registry.Registrations() and not written out as a literal,
// and that is the entire point of the shape. The previous literal silently held
// seven of eight types for the HTTP test — `vendor` was missing and nothing went
// red, because the list of things to check was maintained by hand alongside the
// list of things that exist. Deriving it means adding a type to the registry
// without seeding it here FAILS, loudly, naming the type.
func (w *world) idsOfEveryType() []typeFixture {
	t := w.tb
	t.Helper()
	fx := w.fixtures()
	regs := w.registry.Registrations()
	if len(regs) == 0 {
		t.Fatal("the registry is empty; every derived isolation test would vacuously pass")
	}
	out := make([]typeFixture, 0, len(regs))
	for _, reg := range regs {
		f, ok := fx[reg.Type]
		if !ok {
			t.Fatalf("type %q is registered but has no two-tenant fixture: "+
				"seed one in world.fixtures() so its isolation is actually tested",
				reg.Type)
		}
		out = append(out, f)
	}
	return out
}

// ---------------------------------------------------------------------------
// Every type resolves
// ---------------------------------------------------------------------------

func TestGet_ResolvesEveryEntityType(t *testing.T) {
	w := newWorld(t)
	for _, tc := range w.idsOfEveryType() {
		t.Run(tc.pretty, func(t *testing.T) {
			view, err := w.svc.Get(context.Background(), w.admin(w.tenantA), tc.t, tc.idA)
			if err != nil {
				t.Fatalf("Get(%s) = %v, want a view", tc.t, err)
			}
			if view.Summary.ID != tc.idA {
				t.Errorf("summary id = %q, want %q", view.Summary.ID, tc.idA)
			}
			if view.Summary.Title == "" {
				t.Error("summary has no title; a drawer with no name is unusable")
			}
			if view.Summary.Type != tc.t {
				t.Errorf("summary type = %q, want %q", view.Summary.Type, tc.t)
			}
			if view.Summary.URL == "" {
				t.Error("summary has no deep link")
			}
			if len(view.Sections) == 0 {
				t.Error("no sections offered")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tenant isolation — the claim the isolation registry records as Covered
// ---------------------------------------------------------------------------

func TestGet_CrossTenantIsNotFound(t *testing.T) {
	w := newWorld(t)
	for _, tc := range w.idsOfEveryType() {
		t.Run(tc.pretty, func(t *testing.T) {
			// A caller in tenant A naming a real id that belongs to tenant B.
			_, err := w.svc.Get(context.Background(), w.admin(w.tenantA), tc.t, tc.idB)
			if err == nil {
				t.Fatalf("Get(%s, other tenant's id) succeeded — cross-tenant leak", tc.t)
			}
			if domain.HTTPStatusFromError(err) != 404 {
				t.Fatalf("cross-tenant read answered %d, want 404: %v", domain.HTTPStatusFromError(err), err)
			}
		})
	}
}

// A forged id and a real-but-foreign id must be indistinguishable, or the
// difference becomes an oracle for enumerating another tenant's records.
func TestGet_ForgedAndForeignIDsAnswerIdentically(t *testing.T) {
	w := newWorld(t)
	caller := w.admin(w.tenantA)

	forged := uuid.New().String()
	_, forgedErr := w.svc.Get(context.Background(), caller, TypeRisk, forged)
	_, foreignErr := w.svc.Get(context.Background(), caller, TypeRisk, w.riskB.ID.String())

	if forgedErr == nil || foreignErr == nil {
		t.Fatal("both reads should fail")
	}
	if domain.HTTPStatusFromError(forgedErr) != domain.HTTPStatusFromError(foreignErr) {
		t.Fatalf("status differs: forged=%d foreign=%d",
			domain.HTTPStatusFromError(forgedErr), domain.HTTPStatusFromError(foreignErr))
	}
	// The message must not name the other tenant or say the row exists.
	msg := strings.ToLower(domain.MessageFromError(foreignErr))
	for _, leak := range []string{"another", "other tenant", "organization", "exists"} {
		if strings.Contains(msg, leak) {
			t.Errorf("not-found message leaks existence: %q", msg)
		}
	}
}

// A malformed id must answer the same way, for the same reason.
func TestGet_MalformedIDIsNotFound(t *testing.T) {
	w := newWorld(t)
	_, err := w.svc.Get(context.Background(), w.admin(w.tenantA), TypeAsset, "not-a-uuid")
	if got := domain.HTTPStatusFromError(err); got != 404 {
		t.Fatalf("malformed id answered %d, want 404", got)
	}
}

// ---------------------------------------------------------------------------
// Authorisation
// ---------------------------------------------------------------------------

func TestGet_RequiresTheTypesReadPermission(t *testing.T) {
	w := newWorld(t)
	// Holds risks:read and nothing else.
	caller := callerIn(w.tenantA, "risks:read")

	if _, err := w.svc.Get(context.Background(), caller, TypeRisk, w.riskA.ID.String()); err != nil {
		t.Fatalf("risk read with risks:read failed: %v", err)
	}
	_, err := w.svc.Get(context.Background(), caller, TypeAsset, w.assetA.ID.String())
	if got := domain.HTTPStatusFromError(err); got != 403 {
		t.Fatalf("asset read without assets:read answered %d, want 403", got)
	}
}

// The permission gate must run BEFORE the lookup. If it ran after, a caller with
// no permission would get 404 for a fabricated id and 403 for a real one, which
// tells them which ids exist.
func TestGet_PermissionGateRunsBeforeLookup(t *testing.T) {
	w := newWorld(t)
	caller := callerIn(w.tenantA, "risks:read") // no assets:read

	_, realErr := w.svc.Get(context.Background(), caller, TypeAsset, w.assetA.ID.String())
	_, fakeErr := w.svc.Get(context.Background(), caller, TypeAsset, uuid.New().String())

	if domain.HTTPStatusFromError(realErr) != domain.HTTPStatusFromError(fakeErr) {
		t.Fatalf("real id answered %d but fabricated id answered %d — the pair is an existence oracle",
			domain.HTTPStatusFromError(realErr), domain.HTTPStatusFromError(fakeErr))
	}
}

func TestGet_TenantlessCallerIsRefused(t *testing.T) {
	w := newWorld(t)
	c := Caller{UserID: uuid.New(), TenantID: uuid.Nil, Perms: perms("*")}
	if _, err := w.svc.Get(context.Background(), c, TypeRisk, w.riskA.ID.String()); err == nil {
		t.Fatal("a caller with no tenant was served — the read would have been unscoped")
	}
}

func TestGet_CallerWithNoPermissionSourceFailsClosed(t *testing.T) {
	w := newWorld(t)
	c := Caller{UserID: uuid.New(), TenantID: w.tenantA} // Perms nil
	if _, err := w.svc.Get(context.Background(), c, TypeRisk, w.riskA.ID.String()); err == nil {
		t.Fatal("a caller with no permission source was served")
	}
}

func TestParseType_RejectsUnknown(t *testing.T) {
	if _, err := ParseType("employee"); err == nil {
		t.Fatal("unknown type accepted")
	}
	if got, err := ParseType("  RISK  "); err != nil || got != TypeRisk {
		t.Fatalf("ParseType(padded) = (%q, %v), want risk", got, err)
	}
}

// ---------------------------------------------------------------------------
// Type aliases
// ---------------------------------------------------------------------------

// A vendor is an asset of category vendor, and the restriction is real: asking
// for a server under the vendor type must not render it as a vendor.
func TestVendor_RefusesNonVendorAsset(t *testing.T) {
	w := newWorld(t)
	caller := w.admin(w.tenantA)

	if _, err := w.svc.Get(context.Background(), caller, TypeVendor, w.vendorA.ID.String()); err != nil {
		t.Fatalf("vendor read failed: %v", err)
	}
	_, err := w.svc.Get(context.Background(), caller, TypeVendor, w.assetA.ID.String())
	if got := domain.HTTPStatusFromError(err); got != 404 {
		t.Fatalf("a server read as a vendor answered %d, want 404", got)
	}
}

// A finding IS a vulnerability — same row, different label. The test pins that
// deliberate aliasing so nobody later "fixes" it into a fabricated filter.
func TestFinding_IsAnAliasOfVulnerability(t *testing.T) {
	w := newWorld(t)
	caller := w.admin(w.tenantA)

	asVuln, err := w.svc.Get(context.Background(), caller, TypeVulnerability, w.vulnA.ID.String())
	if err != nil {
		t.Fatalf("vulnerability read failed: %v", err)
	}
	asFinding, err := w.svc.Get(context.Background(), caller, TypeFinding, w.vulnA.ID.String())
	if err != nil {
		t.Fatalf("finding read failed: %v", err)
	}
	if asVuln.Summary.Title != asFinding.Summary.Title {
		t.Errorf("the alias resolved a different row: %q vs %q", asVuln.Summary.Title, asFinding.Summary.Title)
	}
	if asFinding.Summary.TypeLabel != "Finding" {
		t.Errorf("finding label = %q, want Finding", asFinding.Summary.TypeLabel)
	}
}

// ---------------------------------------------------------------------------
// Scores are read, never invented
// ---------------------------------------------------------------------------

func TestScore_IsUnavailableRatherThanZero(t *testing.T) {
	w := newWorld(t)
	caller := w.admin(w.tenantA)

	// A control, an incident and an artifact have no numeric score in this
	// domain. None of them may render as 0.
	for _, tc := range []struct {
		t  Type
		id string
	}{
		{TypeControl, w.controlA.ID.String()},
		{TypeIncident, "1"},
		{TypeEvidence, w.evidenceA.ID.String()},
	} {
		view, err := w.svc.Get(context.Background(), caller, tc.t, tc.id)
		if err != nil {
			t.Fatalf("%s: %v", tc.t, err)
		}
		if view.Summary.Score.Available {
			t.Errorf("%s claims a score of %v; this domain has none for it", tc.t, view.Summary.Score.Value)
		}
		if view.Summary.Score.Unavailable == "" {
			t.Errorf("%s gives no reason for the missing score", tc.t)
		}
	}
}

func TestScore_ReadsTheEnginesValue(t *testing.T) {
	w := newWorld(t)
	view, err := w.svc.Get(context.Background(), w.admin(w.tenantA), TypeRisk, w.riskA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if !view.Summary.Score.Available {
		t.Fatal("a scored risk reports no score")
	}
	if view.Summary.Score.Value != w.riskA.Score {
		t.Errorf("score = %v, want the stored %v — the drawer must not recompute", view.Summary.Score.Value, w.riskA.Score)
	}
	if view.Summary.Score.Basis == "" {
		t.Error("score does not name the engine that produced it")
	}
}

// An unscored risk says so instead of showing 0.
func TestScore_UnscoredRiskIsHonest(t *testing.T) {
	risks := newFakeRisks()
	tenant := uuid.New()
	r := risks.add(&domain.Risk{TenantID: tenant, Name: "Never scored"})
	svc := NewService(NewRegistry().Register(Bind(TypeRisk, NewRiskResolver(risks, newFakeRelations()))))

	view, err := svc.Get(context.Background(), callerIn(tenant, "*"), TypeRisk, r.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if view.Summary.Score.Available {
		t.Fatal("an unscored risk claims a score")
	}
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

// Only allowed actions are returned. A disabled control advertising a permission
// the caller lacks is the "button that lies" this codebase forbids.
func TestActions_OnlyIncludeWhatTheCallerMayDo(t *testing.T) {
	w := newWorld(t)

	readOnly, err := w.svc.Get(context.Background(), callerIn(w.tenantA, "risks:read"), TypeRisk, w.riskA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range readOnly.Actions {
		t.Errorf("a read-only caller was offered %q (%s %s)", a.Key, a.Method, a.Path)
	}

	editor, err := w.svc.Get(context.Background(), callerIn(w.tenantA, "risks:read", "risks:update"), TypeRisk, w.riskA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(editor.Actions) == 0 {
		t.Fatal("an editor was offered no actions")
	}
	for _, a := range editor.Actions {
		if a.Method == "" || a.Path == "" {
			t.Errorf("action %q names no endpoint — it cannot be honest about what it does", a.Key)
		}
		if a.Key == "delete" {
			t.Errorf("delete offered to a caller without risks:delete")
		}
	}
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

// A tab that would always answer 403 must not be offered.
func TestSections_AuditHiddenWithoutTheAuditPermission(t *testing.T) {
	w := newWorld(t)

	plain, err := w.svc.Get(context.Background(), callerIn(w.tenantA, "risks:read"), TypeRisk, w.riskA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if containsSection(plain.Sections, SectionAudit) {
		t.Error("audit tab offered to a caller who cannot read the audit trail")
	}

	auditor, err := w.svc.Get(context.Background(),
		callerIn(w.tenantA, "risks:read", AuditPermission), TypeRisk, w.riskA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if !containsSection(auditor.Sections, SectionAudit) {
		t.Error("audit tab hidden from a caller who holds the audit permission")
	}
}

func containsSection(list []Section, want Section) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Relations
// ---------------------------------------------------------------------------

func TestRelations_AreTenantScoped(t *testing.T) {
	w := newWorld(t)
	caller := w.admin(w.tenantA)

	// Rows exist for tenant A's asset under tenant A only.
	w.relations.set("risks_for_asset", w.tenantA, w.assetA.ID.String(),
		RelationRow{ID: w.riskA.ID.String(), Title: "Log4Shell exposure", Status: "open", Severity: "critical"})
	// And a row planted under tenant B that must never surface for A.
	w.relations.set("risks_for_asset", w.tenantB, w.assetA.ID.String(),
		RelationRow{ID: w.riskB.ID.String(), Title: "Other tenant risk"})

	groups, err := w.svc.Relations(context.Background(), caller, TypeAsset, w.assetA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	risks := findGroup(t, groups, "risks")
	if len(risks.Items) != 1 || risks.Items[0].Title != "Log4Shell exposure" {
		t.Fatalf("relation list = %+v, want only tenant A's risk", risks.Items)
	}

	// Every relation query must have been made with the CALLER's tenant.
	for _, call := range w.relations.calls {
		if !strings.Contains(call, w.tenantA.String()) {
			t.Errorf("a relation query used a tenant other than the caller's: %s", call)
		}
	}
}

func TestRelations_CrossTenantEntityIsNotFound(t *testing.T) {
	w := newWorld(t)
	_, err := w.svc.Relations(context.Background(), w.admin(w.tenantA), TypeAsset, w.assetB.ID.String())
	if got := domain.HTTPStatusFromError(err); got != 404 {
		t.Fatalf("relations for another tenant's asset answered %d, want 404", got)
	}
}

// A group whose TARGET type the caller cannot read comes back marked denied and
// empty — not populated, and not silently missing. "No linked vulnerabilities"
// and "you may not see them" are different facts and the second one has an
// action attached.
func TestRelations_DeniedGroupIsEmptyAndLabelled(t *testing.T) {
	w := newWorld(t)
	w.relations.set("vulns_for_asset", w.tenantA, w.assetA.ID.String(),
		RelationRow{ID: w.vulnA.ID.String(), Title: "Log4j RCE", Severity: "critical"})

	caller := callerIn(w.tenantA, "assets:read") // no vulnerabilities:read
	groups, err := w.svc.Relations(context.Background(), caller, TypeAsset, w.assetA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	g := findGroup(t, groups, "vulnerabilities")
	if !g.Denied {
		t.Error("group not marked denied")
	}
	if len(g.Items) != 0 {
		t.Errorf("denied group leaked %d items", len(g.Items))
	}
	// Empty, not nil. The client types this as an array and reads .length on
	// it; a nil marshals to JSON null and throws in the browser on exactly the
	// path that is hardest to notice in testing. Caught by the live run, not by
	// this suite, because the first version of this test mocked an empty slice.
	if g.Items == nil {
		t.Error("denied group returned a nil item list; it marshals to null and breaks the client")
	}
	if g.Total != 0 {
		t.Errorf("denied group leaked a count of %d — the count alone tells the caller how many exist", g.Total)
	}
}

// One failing relation source degrades one group, never the drawer (§27).
func TestRelations_OneFailingSourceDegradesOnlyItsGroup(t *testing.T) {
	w := newWorld(t)
	w.relations.failFor = "vulns_for_asset"
	w.relations.set("risks_for_asset", w.tenantA, w.assetA.ID.String(),
		RelationRow{ID: w.riskA.ID.String(), Title: "Log4Shell exposure"})

	groups, err := w.svc.Relations(context.Background(), w.admin(w.tenantA), TypeAsset, w.assetA.ID.String())
	if err != nil {
		t.Fatalf("a failing relation source failed the whole read: %v", err)
	}
	if g := findGroup(t, groups, "vulnerabilities"); g.Error == "" {
		t.Error("the failing group does not report its failure")
	}
	if g := findGroup(t, groups, "risks"); len(g.Items) != 1 {
		t.Error("a healthy group was lost because a sibling failed")
	}
}

// Relations carry a deep link so the drawer can chain into them.
func TestRelations_CarryDeepLinks(t *testing.T) {
	w := newWorld(t)
	w.relations.set("risks_for_asset", w.tenantA, w.assetA.ID.String(),
		RelationRow{ID: w.riskA.ID.String(), Title: "Log4Shell exposure"})

	groups, err := w.svc.Relations(context.Background(), w.admin(w.tenantA), TypeAsset, w.assetA.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	item := findGroup(t, groups, "risks").Items[0]
	want := "/risks?drawer=risk&entity=" + w.riskA.ID.String()
	if item.URL != want {
		t.Errorf("relation URL = %q, want %q", item.URL, want)
	}
}

func TestRelations_ReportTruncation(t *testing.T) {
	rows := make([]RelationRow, 0, relationCap)
	for i := 0; i < relationCap; i++ {
		rows = append(rows, RelationRow{ID: uuid.New().String(), Title: "finding"})
	}
	g := group("f", "Findings", TypeFinding, rows, 312, nil, nil)
	if !g.Truncated {
		t.Error("a capped list does not report itself truncated")
	}
	if g.Total != 312 {
		t.Errorf("total = %d, want the real 312 so the drawer can say 'of 312'", g.Total)
	}
}

func findGroup(t *testing.T, groups []RelationGroup, key string) RelationGroup {
	t.Helper()
	for _, g := range groups {
		if g.GroupKey == key {
			return g
		}
	}
	t.Fatalf("no relation group %q in %v", key, groupKeys(groups))
	return RelationGroup{}
}

func groupKeys(groups []RelationGroup) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.GroupKey)
	}
	return out
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

func TestAudit_RequiresAuditPermission(t *testing.T) {
	w := newWorld(t)
	_, err := w.svc.Audit(context.Background(), callerIn(w.tenantA, "risks:read"), TypeRisk, w.riskA.ID.String(), 10, 0)
	if got := domain.HTTPStatusFromError(err); got != 403 {
		t.Fatalf("audit read without the permission answered %d, want 403", got)
	}

	if _, err := w.svc.Audit(context.Background(),
		callerIn(w.tenantA, "risks:read", AuditPermission), TypeRisk, w.riskA.ID.String(), 10, 0); err != nil {
		t.Fatalf("audit read with the permission failed: %v", err)
	}
}

func TestAudit_CrossTenantIsNotFound(t *testing.T) {
	w := newWorld(t)
	_, err := w.svc.Audit(context.Background(), callerIn(w.tenantA, "*"), TypeRisk, w.riskB.ID.String(), 10, 0)
	if got := domain.HTTPStatusFromError(err); got != 404 {
		t.Fatalf("audit for another tenant's risk answered %d, want 404", got)
	}
}

// The audit query must be scoped to the caller's tenant, whatever id was named.
func TestAudit_QueriesTheCallersTenant(t *testing.T) {
	w := newWorld(t)
	now := time.Now().UTC()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", w.riskA.ID.String(), domain.AuditActionUpdate, now),
		auditEvent(w.tenantB, "risk", w.riskA.ID.String(), domain.AuditActionUpdate, now),
	}
	page, err := w.svc.Audit(context.Background(), callerIn(w.tenantA, "*"), TypeRisk, w.riskA.ID.String(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if w.audit.lastTenant != w.tenantA {
		t.Fatalf("audit queried tenant %s, want the caller's %s", w.audit.lastTenant, w.tenantA)
	}
	if len(page.Events) != 1 {
		t.Fatalf("got %d events, want only the caller's tenant's one", len(page.Events))
	}
}

// A control's history is recorded under two names by two writers. Reading only
// one would lose half the trail.
func TestAudit_ReadsBothEntityTypeAliasesForControls(t *testing.T) {
	w := newWorld(t)
	now := time.Now().UTC()
	id := w.controlA.ID.String()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "compliance_control", id, domain.AuditActionUpdate, now),
		auditEvent(w.tenantA, "control", id, domain.AuditActionUpdate, now.Add(-time.Minute)),
	}
	page, err := w.svc.Audit(context.Background(), callerIn(w.tenantA, "*"), TypeControl, id, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 {
		t.Fatalf("got %d audit rows, want 2 — both writers' names must be read", len(page.Events))
	}
}

// ---------------------------------------------------------------------------
// Catalogue and deep links
// ---------------------------------------------------------------------------

func TestCatalogue_MarksWhatTheCallerMayRead(t *testing.T) {
	w := newWorld(t)
	entries := w.svc.Catalogue(callerIn(w.tenantA, "risks:read"))
	if len(entries) != len(Types) {
		t.Fatalf("catalogue lists %d types, want %d", len(entries), len(Types))
	}
	for _, e := range entries {
		want := e.Type == TypeRisk
		if e.Readable != want {
			t.Errorf("%s readable = %v, want %v", e.Type, e.Readable, want)
		}
	}
}

func TestDeepLink_IsStableAndTyped(t *testing.T) {
	id := uuid.New().String()
	cases := map[Type]string{
		TypeRisk:     "/risks?drawer=risk&entity=" + id,
		TypeAsset:    "/assets?drawer=asset&entity=" + id,
		TypeIncident: "/incidents?drawer=incident&entity=" + id,
		TypeEvidence: "/evidence?drawer=evidence&entity=" + id,
	}
	for typ, want := range cases {
		if got := DeepLink(typ, id); got != want {
			t.Errorf("DeepLink(%s) = %q, want %q", typ, got, want)
		}
	}
	if DeepLink(Type("nope"), id) != "" {
		t.Error("an unknown type produced a link")
	}
	if DeepLink(TypeRisk, "") != "" {
		t.Error("an empty id produced a link")
	}
}

func TestRegistry_UnwiredTypeIsATypedError(t *testing.T) {
	svc := NewService(NewRegistry()) // nothing registered
	_, err := svc.Get(context.Background(), callerIn(uuid.New(), "*"), TypeRisk, uuid.New().String())
	if err == nil {
		t.Fatal("an unwired type resolved")
	}
	if domain.HTTPStatusFromError(err) == 500 {
		t.Error("an unwired type crashed rather than answering a typed error")
	}
}
