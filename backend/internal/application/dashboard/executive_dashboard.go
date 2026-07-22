// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package dashboard assembles the executive ("Tableau de bord exécutif", spec §11)
// dashboard: a single tenant-scoped aggregation that consolidates risk, financial,
// compliance, vulnerability and incident posture so the frontend makes ONE request
// instead of a dozen. It composes the existing well-built, tenant-scoped use cases
// and repositories rather than re-querying — every source port is optional and
// nil-safe, so a missing capability degrades its slice of the dashboard to an empty
// state instead of failing the whole board (the same contract as the smart-score
// and financial-summary use cases).
package dashboard

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
)

// ---------------------------------------------------------------------------
// Ports (narrow, all optional / nil-safe)
// ---------------------------------------------------------------------------

// FinancialSource yields the tenant-wide CRQ posture (portfolio ALE, worst-case,
// top exposures). Satisfied by *risk.FinancialSummaryUseCase.
type FinancialSource interface {
	Execute(ctx context.Context, tenantID uuid.UUID) (*risk.FinancialSummary, error)
}

// RiskSource yields register aggregates. Satisfied directly by the concrete
// *repository.GormRiskRepository (these are concrete methods kept off the
// domain.RiskRepository port so existing mocks stay valid).
type RiskSource interface {
	CountRisksByCriticality(ctx context.Context, tenantID uuid.UUID) (map[string]int, error)
	MonthlyRiskTrend(ctx context.Context, tenantID uuid.UUID, months int) ([]MonthlyRiskPoint, error)
	TopRisksByScore(ctx context.Context, tenantID uuid.UUID, limit int) ([]domain.Risk, error)
}

// ComplianceSource yields the per-framework coverage roll-up. Satisfied by
// *compliance.GetGapAnalysisUseCase.
type ComplianceSource interface {
	Execute(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (*compliance.GapAnalysis, error)
}

// VulnSource yields the tenant vulnerability stats. Satisfied by
// domain.VulnerabilityRepository.
type VulnSource interface {
	Stats(ctx context.Context, tenantID uuid.UUID) (*domain.VulnStats, error)
}

// IncidentSource yields incident analytics (trend + open/critical/MTTR). Wired via
// a thin adapter over the legacy incident service in main.go.
type IncidentSource interface {
	IncidentAnalytics(ctx context.Context, tenantID string, months int) (*IncidentAnalytics, error)
}

// MonthlyRiskPoint is one month of the register's risk evolution.
type MonthlyRiskPoint struct {
	Month    string  `json:"month"` // "YYYY-MM"
	AvgScore float64 `json:"avg_score"`
	Critical int     `json:"critical"`
	High     int     `json:"high"`
	Total    int     `json:"total"`
}

// IncidentAnalytics is the incident slice of the dashboard, gathered by the source
// adapter (open volume, still-open criticals, mean-time-to-resolve, resolution
// rate and a monthly trend for the histogram).
type IncidentAnalytics struct {
	Total          int                  `json:"total"`
	OpenCount      int                  `json:"open_count"`
	CriticalOpen   int                  `json:"critical_open"`
	AvgMTTRDays    float64              `json:"avg_mttr_days"`
	ResolutionRate float64              `json:"resolution_rate"`
	Trend          []IncidentTrendPoint `json:"trend"`
}

// ---------------------------------------------------------------------------
// Response DTO
// ---------------------------------------------------------------------------

// FinancialHeadline is the exposure KPI shown at the top of the board.
type FinancialHeadline struct {
	TotalALE        crq.Money `json:"total_ale"`
	TotalALEWorst   crq.Money `json:"total_ale_worst"`
	TotalRisks      int       `json:"total_risks"`
	QuantifiedRisks int       `json:"quantified_risks"`
}

// KRI is one key risk indicator card. Severity drives the color (ok/warn/critical).
type KRI struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`     // "", "days", "%"
	Severity string  `json:"severity"` // ok | warn | critical
}

// ExecRisk is one row of the "top 10 risks" table/heatmap.
type ExecRisk struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Score       float64   `json:"score"`
	Probability int       `json:"probability"` // 1..5 band for the heatmap
	Impact      int       `json:"impact"`      // 1..5 band for the heatmap
	Criticality string    `json:"criticality"`
	Status      string    `json:"status"`
	Phase       string    `json:"lifecycle_phase"`
	ALE         crq.Money `json:"ale"`
}

// ComplianceCoverage is one framework's completion — powers both the compliance
// donuts and the control-coverage radar on the frontend.
type ComplianceCoverage struct {
	FrameworkID string  `json:"framework_id"`
	Name        string  `json:"name"`
	Percent     float64 `json:"percent"`
	Implemented int     `json:"implemented"`
	Total       int     `json:"total"`
}

// DistributionSlice is one wedge of the risk-by-criticality donut.
type DistributionSlice struct {
	Criticality string `json:"criticality"`
	Count       int    `json:"count"`
}

// IncidentTrendPoint is one bar of the incident histogram.
type IncidentTrendPoint struct {
	Month    string `json:"month"` // "YYYY-MM"
	Total    int    `json:"total"`
	Critical int    `json:"critical"`
	High     int    `json:"high"`
}

// ExecutiveDashboard is the single consolidated payload for spec §11.
type ExecutiveDashboard struct {
	GeneratedAt time.Time `json:"generated_at"`
	Currency    string    `json:"currency"`
	XAFPerUSD   float64   `json:"xaf_per_usd"`

	CyberScore       CyberScore           `json:"cyber_score"`
	Financial        FinancialHeadline    `json:"financial"`
	KRIs             []KRI                `json:"kris"`
	TopRisks         []ExecRisk           `json:"top_risks"`
	RiskTrend        []MonthlyRiskPoint   `json:"risk_trend"`
	RiskDistribution []DistributionSlice  `json:"risk_distribution"`
	Compliance       []ComplianceCoverage `json:"compliance"`
	IncidentTrend    []IncidentTrendPoint `json:"incident_trend"`
}

// ---------------------------------------------------------------------------
// Use case
// ---------------------------------------------------------------------------

const (
	trendMonths  = 6  // window for the risk & incident trend charts
	topRiskLimit = 10 // "top 10 risks" widget
)

// GetExecutiveDashboardUseCase consolidates the tenant's posture. Every source is
// optional: pass nil to omit that slice. Build with New… then attach sources via
// the With… options.
type GetExecutiveDashboardUseCase struct {
	financial  FinancialSource
	risks      RiskSource
	compliance ComplianceSource
	vulns      VulnSource
	incidents  IncidentSource
	quantifier *crq.Quantifier
}

// NewGetExecutiveDashboardUseCase builds the use case with no sources attached.
func NewGetExecutiveDashboardUseCase() *GetExecutiveDashboardUseCase {
	return &GetExecutiveDashboardUseCase{}
}

func (uc *GetExecutiveDashboardUseCase) WithFinancial(s FinancialSource) *GetExecutiveDashboardUseCase {
	uc.financial = s
	return uc
}
func (uc *GetExecutiveDashboardUseCase) WithRisks(s RiskSource) *GetExecutiveDashboardUseCase {
	uc.risks = s
	return uc
}
func (uc *GetExecutiveDashboardUseCase) WithCompliance(s ComplianceSource) *GetExecutiveDashboardUseCase {
	uc.compliance = s
	return uc
}
func (uc *GetExecutiveDashboardUseCase) WithVulnerabilities(s VulnSource) *GetExecutiveDashboardUseCase {
	uc.vulns = s
	return uc
}
func (uc *GetExecutiveDashboardUseCase) WithIncidents(s IncidentSource) *GetExecutiveDashboardUseCase {
	uc.incidents = s
	return uc
}
func (uc *GetExecutiveDashboardUseCase) WithQuantifier(q *crq.Quantifier) *GetExecutiveDashboardUseCase {
	uc.quantifier = q
	return uc
}

// Execute assembles the dashboard for a tenant. A failure in any single source is
// swallowed (that slice degrades to empty) so the board always renders — an
// executive view is best-effort by nature.
func (uc *GetExecutiveDashboardUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*ExecutiveDashboard, error) {
	out := &ExecutiveDashboard{
		GeneratedAt:      time.Now().UTC(),
		Currency:         "XAF",
		XAFPerUSD:        crq.DefaultXAFPerUSD,
		KRIs:             []KRI{},
		TopRisks:         []ExecRisk{},
		RiskTrend:        []MonthlyRiskPoint{},
		RiskDistribution: []DistributionSlice{},
		Compliance:       []ComplianceCoverage{},
		IncidentTrend:    []IncidentTrendPoint{},
	}
	if uc.quantifier != nil {
		out.XAFPerUSD = uc.quantifier.XAFPerUSD
	}

	// Axes fed into the composite cyber score.
	var complAxisImpl, complAxisTotal int
	var haveVuln bool
	var vulnKEV, vulnCritical int64
	var incAnalytics *IncidentAnalytics

	// --- Financial exposure (widget 1) --------------------------------------
	if uc.financial != nil {
		if fin, err := uc.financial.Execute(ctx, tenantID); err == nil && fin != nil {
			out.Financial = FinancialHeadline{
				TotalALE:        fin.TotalALE,
				TotalALEWorst:   fin.TotalALEWorst,
				TotalRisks:      fin.TotalRisks,
				QuantifiedRisks: fin.QuantifiedRisks,
			}
			out.Currency = fin.Currency
			if fin.XAFPerUSD > 0 {
				out.XAFPerUSD = fin.XAFPerUSD
			}
		}
	}

	// --- Risk distribution, trend, top-10 (widgets 2, 3) --------------------
	var critCount, highCount int
	if uc.risks != nil {
		if counts, err := uc.risks.CountRisksByCriticality(ctx, tenantID); err == nil {
			for _, band := range []string{"critical", "high", "medium", "low"} {
				out.RiskDistribution = append(out.RiskDistribution, DistributionSlice{
					Criticality: band,
					Count:       counts[band],
				})
			}
			critCount = counts["critical"]
			highCount = counts["high"]
		}
		if trend, err := uc.risks.MonthlyRiskTrend(ctx, tenantID, trendMonths); err == nil && trend != nil {
			out.RiskTrend = trend
		}
		if tops, err := uc.risks.TopRisksByScore(ctx, tenantID, topRiskLimit); err == nil {
			out.TopRisks = uc.buildTopRisks(tops)
		}
	}

	// --- Compliance coverage (widgets 4, 8) ---------------------------------
	if uc.compliance != nil {
		if gap, err := uc.compliance.Execute(ctx, tenantID, uuid.Nil); err == nil && gap != nil {
			for _, fw := range gap.Frameworks {
				out.Compliance = append(out.Compliance, ComplianceCoverage{
					FrameworkID: fw.FrameworkID.String(),
					Name:        fw.FrameworkName,
					Percent:     round1(fw.PercentComplete),
					Implemented: fw.Implemented,
					Total:       fw.Total,
				})
				complAxisImpl += fw.Implemented
				complAxisTotal += fw.Total
			}
		}
	}

	// --- Vulnerability KRIs (widget 5) --------------------------------------
	if uc.vulns != nil {
		if vs, err := uc.vulns.Stats(ctx, tenantID); err == nil && vs != nil {
			haveVuln = true
			vulnKEV = vs.KEVCount
			vulnCritical = vs.BySeverity["critical"]
			out.KRIs = append(out.KRIs,
				KRI{Key: "open_vulns", Label: "Vulnérabilités ouvertes", Value: float64(vs.Open), Unit: "", Severity: sevForCount(int(vs.Open), 5, 20)},
				KRI{Key: "kev_exploited", Label: "Exploitées (KEV)", Value: float64(vs.KEVCount), Unit: "", Severity: sevForCount(int(vs.KEVCount), 1, 5)},
				KRI{Key: "critical_vulns", Label: "Vulnérabilités critiques", Value: float64(vulnCritical), Unit: "", Severity: sevForCount(int(vulnCritical), 1, 5)},
			)
		}
	}

	// Always-available register KRI (critical risks).
	out.KRIs = append(out.KRIs,
		KRI{Key: "critical_risks", Label: "Risques critiques", Value: float64(critCount), Unit: "", Severity: sevForCount(critCount, 1, 5)},
	)

	// --- Incident trend + MTTR (widgets 5, 7) -------------------------------
	if uc.incidents != nil {
		if ia, err := uc.incidents.IncidentAnalytics(ctx, tenantID.String(), trendMonths); err == nil && ia != nil {
			incAnalytics = ia
			out.IncidentTrend = ia.Trend
			out.KRIs = append(out.KRIs,
				KRI{Key: "open_incidents", Label: "Incidents ouverts", Value: float64(ia.OpenCount), Unit: "", Severity: sevForCount(ia.OpenCount, 1, 5)},
				KRI{Key: "avg_mttr_days", Label: "Délai moyen de remédiation", Value: round1(ia.AvgMTTRDays), Unit: "days", Severity: sevForMTTR(ia.AvgMTTRDays)},
			)
		}
	}

	// Compliance coverage KRI (if we have any frameworks).
	if complAxisTotal > 0 {
		pct := 100 * float64(complAxisImpl) / float64(complAxisTotal)
		out.KRIs = append(out.KRIs,
			KRI{Key: "compliance_coverage", Label: "Couverture conformité", Value: round1(pct), Unit: "%", Severity: sevForCoverage(pct)},
		)
	}

	// --- Cyber score (widget 6) ---------------------------------------------
	complV, complOK := complianceAxisValue(complAxisImpl, complAxisTotal)
	riskV, riskOK := riskAxisValue(critCount, highCount, critCount+highCount+distTotal(out.RiskDistribution))
	vulnV, vulnOK := vulnAxisValue(vulnKEV, vulnCritical, haveVuln)
	var incV float64
	var incOK bool
	if incAnalytics != nil {
		incV, incOK = incidentAxisValue(incAnalytics.ResolutionRate, incAnalytics.CriticalOpen, incAnalytics.Total)
	}
	out.CyberScore = computeCyberScore([]scoreAxis{
		{key: "compliance", label: "Conformité", weight: weightCompliance, value: complV, present: complOK},
		{key: "risk", label: "Risques", weight: weightRisk, value: riskV, present: riskOK},
		{key: "vulnerabilities", label: "Vulnérabilités", weight: weightVuln, value: vulnV, present: vulnOK},
		{key: "incidents", label: "Incidents", weight: weightIncident, value: incV, present: incOK},
	})

	return out, nil
}

// buildTopRisks maps register risks onto the exec DTO, quantifying each one's ALE
// when the quantifier is attached, and deriving a 1..5 probability/impact band for
// the heatmap from the stored score (P×I×AC ≤ 30 → mapped onto a 5×5 grid).
func (uc *GetExecutiveDashboardUseCase) buildTopRisks(risks []domain.Risk) []ExecRisk {
	rows := make([]ExecRisk, 0, len(risks))
	for i := range risks {
		r := &risks[i]
		var ale crq.Money
		if uc.quantifier != nil {
			a := uc.quantifier.Assess(crq.FinancialInputs{
				SLEXAF:                  r.SLEXAF,
				ARO:                     r.ARO,
				DowntimeHours:           r.DowntimeHours,
				HourlyDowntimeCostXAF:   r.HourlyDowntimeCostXAF,
				DataLossCostXAF:         r.DataLossCostXAF,
				FinesXAF:                r.FinesXAF,
				OtherDirectCostXAF:      r.OtherDirectCostXAF,
				RemediationCostXAF:      r.RemediationCostXAF,
				MitigationEffectiveness: r.MitigationEffectiveness,
			}, string(r.Criticality))
			ale = a.ALE
		}
		p, im := bands(r.Score)
		rows = append(rows, ExecRisk{
			ID:          r.ID.String(),
			Title:       title(r),
			Score:       r.Score,
			Probability: p,
			Impact:      im,
			Criticality: strings.ToLower(string(r.Criticality)),
			Status:      string(r.Status),
			Phase:       string(r.LifecyclePhase),
			ALE:         ale,
		})
	}
	return rows
}

// --- small helpers ----------------------------------------------------------

func title(r *domain.Risk) string {
	if strings.TrimSpace(r.Title) != "" {
		return r.Title
	}
	return r.Name
}

// bands derives a coarse 1..5 probability and impact from the stored score so the
// executive heatmap can place a risk without needing the raw P/I (the register
// keeps a single P×I×AC score). Even split of sqrt(score) across both axes.
func bands(score float64) (p, i int) {
	// score ranges ~0..30 (P 0..1 × I 0..10 × AC 0.1..3). Normalise to 0..25.
	n := score
	if n > 25 {
		n = 25
	}
	// distribute onto a 5×5 grid: pick the largest factor pair near sqrt(n).
	band := 1
	switch {
	case n >= 20:
		band = 5
	case n >= 12:
		band = 4
	case n >= 6:
		band = 3
	case n >= 2:
		band = 2
	}
	return band, band
}

func distTotal(d []DistributionSlice) int {
	// medium + low only (critical/high are passed separately to riskAxisValue).
	var t int
	for _, s := range d {
		if s.Criticality == "medium" || s.Criticality == "low" {
			t += s.Count
		}
	}
	return t
}

func sevForCount(v, warn, crit int) string {
	switch {
	case v >= crit:
		return "critical"
	case v >= warn:
		return "warn"
	default:
		return "ok"
	}
}

func sevForMTTR(days float64) string {
	switch {
	case days >= 14:
		return "critical"
	case days >= 5:
		return "warn"
	default:
		return "ok"
	}
}

func sevForCoverage(pct float64) string {
	switch {
	case pct < 50:
		return "critical"
	case pct < 80:
		return "warn"
	default:
		return "ok"
	}
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
