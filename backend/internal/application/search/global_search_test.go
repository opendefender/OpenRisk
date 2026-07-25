// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package search

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	cti "github.com/opendefender/openrisk/pkg/cti"
)

type fakeRisks struct {
	hits   []domain.Risk
	err    error
	called bool
}

func (f *fakeRisks) SearchByText(_ context.Context, _ uuid.UUID, _ string, _ int) ([]domain.Risk, error) {
	f.called = true
	return f.hits, f.err
}

type fakeAssets struct {
	items []domain.Asset
	err   error
}

func (f *fakeAssets) List(_ context.Context, _ uuid.UUID) ([]domain.Asset, error) {
	return f.items, f.err
}

type fakeVulns struct {
	page *domain.PaginatedResult[domain.Vulnerability]
	err  error
}

func (f *fakeVulns) List(_ context.Context, _ uuid.UUID, _ domain.VulnerabilityQuery) (*domain.PaginatedResult[domain.Vulnerability], error) {
	return f.page, f.err
}

type fakeControls struct {
	hits []domain.ComplianceControl
	err  error
}

func (f *fakeControls) SearchControls(_ context.Context, _ uuid.UUID, _ string, _ int) ([]domain.ComplianceControl, error) {
	return f.hits, f.err
}

type fakeAudits struct {
	items []domain.ComplianceAudit
	err   error
}

func (f *fakeAudits) ListAudits(_ context.Context, _ uuid.UUID) ([]domain.ComplianceAudit, error) {
	return f.items, f.err
}

type fakeReports struct {
	items []domain.BoardReport
	err   error
}

func (f *fakeReports) List(_ context.Context, _ uuid.UUID) ([]domain.BoardReport, error) {
	return f.items, f.err
}

type fakeCVE struct {
	hits []cti.CTIVulnerability
	err  error
}

func (f *fakeCVE) Search(_ context.Context, _ string, _ cti.CTIFilter) ([]cti.CTIVulnerability, error) {
	return f.hits, f.err
}

type fakeMembers struct {
	items []domain.OrganizationMember
	err   error
}

func (f *fakeMembers) ListMembers(_ context.Context, _ uuid.UUID) ([]domain.OrganizationMember, error) {
	return f.items, f.err
}

// sampleUC wires all eight sources, each with a single "web"-matching hit.
func sampleUC() (*UseCase, *fakeRisks) {
	risks := &fakeRisks{hits: []domain.Risk{{ID: uuid.New(), Title: "Web exposure", Status: domain.RiskStatus("open"), Criticality: domain.CriticalityLevel("high"), Score: 12.5}}}
	assets := &fakeAssets{items: []domain.Asset{{ID: uuid.New(), Name: "web-01", Type: "Server", Criticality: domain.CriticalityCritical}}}
	vulns := &fakeVulns{page: &domain.PaginatedResult[domain.Vulnerability]{Data: []domain.Vulnerability{{ID: uuid.New(), Title: "Log4Shell", CVEID: "CVE-2021-44228", Severity: domain.VulnSeverityCritical, PriorityScore: 80}}}}
	controls := &fakeControls{hits: []domain.ComplianceControl{{ID: uuid.New(), Name: "Web application firewall", ReferenceCode: "A.13.1", Status: domain.ControlStatus("implemented")}}}
	audits := &fakeAudits{items: []domain.ComplianceAudit{{ID: uuid.New(), Title: "Web platform audit", Type: domain.AuditType("internal"), Status: domain.AuditStatus("planned")}}}
	reports := &fakeReports{items: []domain.BoardReport{{ID: uuid.New(), Title: "Web posture Q3", PeriodLabel: "Juillet 2026"}}}
	cve := &fakeCVE{hits: []cti.CTIVulnerability{{CVEID: "CVE-2024-0001", Description: "web server rce", Severity: "high"}}}
	members := &fakeMembers{items: []domain.OrganizationMember{{UserID: uuid.New(), User: &domain.User{ID: uuid.New(), FullName: "Web Admin", Email: "web.admin@corp.io", Username: "webadmin"}}}}
	uc := New().WithRisks(risks).WithAssets(assets).WithVulns(vulns).
		WithControls(controls).WithAudits(audits).WithReports(reports).
		WithCVE(cve).WithMembers(members)
	return uc, risks
}

func allow(_ string) bool { return true }

func countByType(res []Result) map[string]int {
	m := map[string]int{}
	for _, r := range res {
		m[r.Type]++
	}
	return m
}

func TestGlobalSearch_Success(t *testing.T) {
	uc, _ := sampleUC()
	out := uc.Execute(context.Background(), uuid.New(), "web", allow)
	c := countByType(out.Results)
	for _, typ := range []string{"risk", "asset", "vulnerability", "control", "audit", "report", "cve", "user"} {
		if c[typ] != 1 {
			t.Fatalf("expected one %q hit, got %+v (results=%+v)", typ, c, out.Results)
		}
	}
	for _, r := range out.Results {
		if r.URL == "" || r.ID == "" {
			t.Fatalf("result missing URL/ID: %+v", r)
		}
	}
}

func TestGlobalSearch_PermissionFiltered(t *testing.T) {
	uc, _ := sampleUC()
	// Caller may only read risks — that gate also covers CVE (threat intel), but
	// everything else must be skipped.
	canRisksOnly := func(p string) bool { return p == "risks:read" }
	out := uc.Execute(context.Background(), uuid.New(), "web", canRisksOnly)
	c := countByType(out.Results)
	if c["risk"] != 1 || c["cve"] != 1 {
		t.Fatalf("expected risk + cve (both risks:read), got %+v", c)
	}
	for _, typ := range []string{"asset", "vulnerability", "control", "audit", "report", "user"} {
		if c[typ] != 0 {
			t.Fatalf("expected %q to be gated out, got %+v", typ, c)
		}
	}
}

func TestGlobalSearch_UsersAdminOnly(t *testing.T) {
	uc, _ := sampleUC()
	// A broad non-admin who can read every section but is NOT admin ("*") must not
	// see users in search.
	canNonAdmin := func(p string) bool { return p != "*" }
	out := uc.Execute(context.Background(), uuid.New(), "web", canNonAdmin)
	if countByType(out.Results)["user"] != 0 {
		t.Fatal("user results must be admin-only")
	}
	// An admin sees them.
	if countByType(uc.Execute(context.Background(), uuid.New(), "web", allow).Results)["user"] != 1 {
		t.Fatal("admin must see user results")
	}
}

func TestGlobalSearch_EmptyQuery(t *testing.T) {
	uc, risks := sampleUC()
	out := uc.Execute(context.Background(), uuid.New(), "   ", allow)
	if len(out.Results) != 0 {
		t.Fatalf("empty query must yield no results, got %d", len(out.Results))
	}
	if risks.called {
		t.Fatal("no source should be queried for an empty query")
	}
}

func TestGlobalSearch_SourceErrorDegrades(t *testing.T) {
	uc, risks := sampleUC()
	risks.err = context.DeadlineExceeded // risk source fails
	out := uc.Execute(context.Background(), uuid.New(), "web", allow)
	c := countByType(out.Results)
	if c["risk"] != 0 {
		t.Fatalf("a failing risk source must contribute nothing, got %+v", c)
	}
	// Every other source must still return despite the risk error.
	for _, typ := range []string{"asset", "vulnerability", "control", "audit", "report", "cve", "user"} {
		if c[typ] != 1 {
			t.Fatalf("expected %q despite risk error, got %+v", typ, c)
		}
	}
}

func TestGlobalSearch_NilTenant(t *testing.T) {
	uc, _ := sampleUC()
	out := uc.Execute(context.Background(), uuid.Nil, "web", allow)
	if len(out.Results) != 0 {
		t.Fatalf("nil tenant must yield no results, got %d", len(out.Results))
	}
}
