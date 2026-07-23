// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package search

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
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

func sampleUC() (*UseCase, *fakeRisks) {
	risks := &fakeRisks{hits: []domain.Risk{{ID: uuid.New(), Title: "Web exposure", Status: domain.RiskStatus("open"), Criticality: domain.CriticalityLevel("high"), Score: 12.5}}}
	assets := &fakeAssets{items: []domain.Asset{{ID: uuid.New(), Name: "web-01", Type: "Server", Criticality: domain.CriticalityCritical}}}
	vulns := &fakeVulns{page: &domain.PaginatedResult[domain.Vulnerability]{Data: []domain.Vulnerability{{ID: uuid.New(), Title: "Log4Shell", CVEID: "CVE-2021-44228", Severity: domain.VulnSeverityCritical, PriorityScore: 80}}}}
	uc := New().WithRisks(risks).WithAssets(assets).WithVulns(vulns)
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
	if c["risk"] != 1 || c["asset"] != 1 || c["vulnerability"] != 1 {
		t.Fatalf("expected one hit per type, got %+v (results=%+v)", c, out.Results)
	}
	// URLs must be deep-links to the entity.
	for _, r := range out.Results {
		if r.URL == "" || r.ID == "" {
			t.Fatalf("result missing URL/ID: %+v", r)
		}
	}
}

func TestGlobalSearch_PermissionFiltered(t *testing.T) {
	uc, _ := sampleUC()
	// Caller may only read risks — asset/vuln sources must be skipped entirely.
	canRisksOnly := func(p string) bool { return p == "risks:read" }
	out := uc.Execute(context.Background(), uuid.New(), "web", canRisksOnly)
	c := countByType(out.Results)
	if c["risk"] != 1 {
		t.Fatalf("expected the risk hit, got %+v", c)
	}
	if c["asset"] != 0 || c["vulnerability"] != 0 {
		t.Fatalf("expected asset/vuln to be gated out, got %+v", c)
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
	if c["asset"] != 1 || c["vulnerability"] != 1 {
		t.Fatalf("other sources must still return despite risk error, got %+v", c)
	}
}

func TestGlobalSearch_NilTenant(t *testing.T) {
	uc, _ := sampleUC()
	out := uc.Execute(context.Background(), uuid.Nil, "web", allow)
	if len(out.Results) != 0 {
		t.Fatalf("nil tenant must yield no results, got %d", len(out.Results))
	}
}
