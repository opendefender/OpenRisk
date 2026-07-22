// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
)

// --- fakes ------------------------------------------------------------------

type fakeFinancial struct{ sum *risk.FinancialSummary }

func (f fakeFinancial) Execute(context.Context, uuid.UUID) (*risk.FinancialSummary, error) {
	return f.sum, nil
}

type fakeRisks struct {
	counts map[string]int
	trend  []MonthlyRiskPoint
	top    []domain.Risk
}

func (f fakeRisks) CountRisksByCriticality(context.Context, uuid.UUID) (map[string]int, error) {
	return f.counts, nil
}
func (f fakeRisks) MonthlyRiskTrend(context.Context, uuid.UUID, int) ([]MonthlyRiskPoint, error) {
	return f.trend, nil
}
func (f fakeRisks) TopRisksByScore(context.Context, uuid.UUID, int) ([]domain.Risk, error) {
	return f.top, nil
}

type fakeCompliance struct{ gap *compliance.GapAnalysis }

func (f fakeCompliance) Execute(context.Context, uuid.UUID, uuid.UUID) (*compliance.GapAnalysis, error) {
	return f.gap, nil
}

type fakeVuln struct{ stats *domain.VulnStats }

func (f fakeVuln) Stats(context.Context, uuid.UUID) (*domain.VulnStats, error) { return f.stats, nil }

type fakeIncidents struct{ a *IncidentAnalytics }

func (f fakeIncidents) IncidentAnalytics(context.Context, string, int) (*IncidentAnalytics, error) {
	return f.a, nil
}

// --- tests ------------------------------------------------------------------

func TestExecutiveDashboard_Success(t *testing.T) {
	q := crq.NewQuantifier(crq.DefaultXAFPerUSD, crq.DefaultReference())

	fin := &risk.FinancialSummary{
		Currency:        "XAF",
		XAFPerUSD:       crq.DefaultXAFPerUSD,
		TotalRisks:      7,
		QuantifiedRisks: 3,
		TotalALE:        crq.Money{XAF: 97_500_000, USD: 162_500},
		TotalALEWorst:   crq.Money{XAF: 195_000_000, USD: 325_000},
	}
	risks := &fakeRisks{
		counts: map[string]int{"critical": 2, "high": 3, "medium": 1, "low": 1},
		trend: []MonthlyRiskPoint{
			{Month: "2026-06", AvgScore: 5.2, Critical: 2, High: 3, Total: 7},
			{Month: "2026-07", AvgScore: 6.1, Critical: 2, High: 3, Total: 7},
		},
		top: []domain.Risk{
			{ID: uuid.New(), Title: "Log4Shell", Score: 24.3, Criticality: domain.CriticalityLevel("critical"), Status: domain.RiskStatus("open"), LifecyclePhase: domain.RiskPhase("evaluated")},
			{ID: uuid.New(), Name: "Weak TLS", Score: 8.5, Criticality: domain.CriticalityLevel("high"), Status: domain.RiskStatus("in_progress")},
		},
	}
	gap := &compliance.GapAnalysis{
		Frameworks: []compliance.FrameworkGapSummary{
			{FrameworkID: uuid.New(), FrameworkName: "ISO 27001", Total: 93, Implemented: 40, PercentComplete: 43.0},
			{FrameworkID: uuid.New(), FrameworkName: "NIS2", Total: 12, Implemented: 3, PercentComplete: 25.0},
		},
	}
	vs := &domain.VulnStats{Total: 20, Open: 12, KEVCount: 2, BySeverity: map[string]int64{"critical": 4}}
	inc := &IncidentAnalytics{
		Total: 5, OpenCount: 2, CriticalOpen: 1, AvgMTTRDays: 4.2, ResolutionRate: 60,
		Trend: []IncidentTrendPoint{{Month: "2026-07", Total: 3, Critical: 1, High: 1}},
	}

	uc := NewGetExecutiveDashboardUseCase().
		WithFinancial(fakeFinancial{sum: fin}).
		WithRisks(risks).
		WithCompliance(fakeCompliance{gap: gap}).
		WithVulnerabilities(fakeVuln{stats: vs}).
		WithIncidents(fakeIncidents{a: inc}).
		WithQuantifier(q)

	out, err := uc.Execute(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if out.Financial.TotalALE.XAF != 97_500_000 {
		t.Errorf("financial ALE = %v", out.Financial.TotalALE.XAF)
	}
	if len(out.RiskDistribution) != 4 {
		t.Errorf("distribution = %d, want 4 bands", len(out.RiskDistribution))
	}
	if len(out.TopRisks) != 2 || out.TopRisks[0].Title != "Log4Shell" {
		t.Errorf("top risks = %+v", out.TopRisks)
	}
	if out.TopRisks[0].Probability != 5 || out.TopRisks[0].Impact != 5 {
		t.Errorf("score 24.3 should map to band 5, got P%d/I%d", out.TopRisks[0].Probability, out.TopRisks[0].Impact)
	}
	if len(out.RiskTrend) != 2 || len(out.IncidentTrend) != 1 {
		t.Errorf("trends: risk=%d incident=%d", len(out.RiskTrend), len(out.IncidentTrend))
	}
	if len(out.Compliance) != 2 || out.Compliance[0].Name != "ISO 27001" {
		t.Errorf("compliance = %+v", out.Compliance)
	}
	// Cyber score must be computed with all four axes present.
	if out.CyberScore.Grade == "" || len(out.CyberScore.Components) != 4 {
		t.Errorf("cyber score not fully computed: %+v", out.CyberScore)
	}
	// MTTR + compliance-coverage KRIs must be present.
	var haveMTTR, haveCoverage bool
	for _, k := range out.KRIs {
		if k.Key == "avg_mttr_days" {
			haveMTTR = true
		}
		if k.Key == "compliance_coverage" {
			haveCoverage = true
		}
	}
	if !haveMTTR || !haveCoverage {
		t.Errorf("missing KRIs: mttr=%v coverage=%v", haveMTTR, haveCoverage)
	}
}

func TestExecutiveDashboard_DegradesWithNoSources(t *testing.T) {
	// No sources attached: the board must still render (empty slices, neutral score).
	uc := NewGetExecutiveDashboardUseCase()
	out, err := uc.Execute(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out == nil {
		t.Fatal("nil dashboard")
	}
	if len(out.TopRisks) != 0 || len(out.Compliance) != 0 || len(out.RiskTrend) != 0 {
		t.Errorf("expected empty slices, got %+v", out)
	}
	// Only the always-on critical_risks KRI (value 0) should be present.
	if out.CyberScore.Score != 50 || out.CyberScore.Grade != "E" {
		t.Errorf("neutral score expected, got %d/%s", out.CyberScore.Score, out.CyberScore.Grade)
	}
}
