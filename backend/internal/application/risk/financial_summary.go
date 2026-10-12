// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
)

// FinancialRiskLister is the narrow port the financial summary needs: the
// tenant's risks with their monetary drivers. GormRiskRepository satisfies it via
// its concrete ListRisksForFinancial method (kept off the domain port so mocks of
// RiskRepository stay valid).
type FinancialRiskLister interface {
	ListRisksForFinancial(ctx context.Context, tenantID uuid.UUID) ([]domain.Risk, error)
}

// FinancialCoverageCounter returns the tenant's total + quantified risk counts
// from a single SQL aggregate, so the "N/M quantified" figure is a server fact
// rather than a client-side filter (spec §6). Optional/nil-safe.
type FinancialCoverageCounter interface {
	CountFinancialCoverage(ctx context.Context, tenantID uuid.UUID) (total, quantified int, err error)
}

// RiskAppetiteReader returns the tenant's risk appetite in XAF, or nil when the
// tenant has not set one (#904). Optional/nil-safe.
type RiskAppetiteReader interface {
	OrgRiskAppetite(ctx context.Context, tenantID uuid.UUID) (*float64, error)
}

// LossExceedance is the loss-exceedance curve of the whole register in compact
// form: total annual loss (XAF) at every percentile 0..100, before and after the
// treatment plans (#904). P(loss > x) is the share of percentiles above x.
type LossExceedance struct {
	Inherent []float64 `json:"inherent"`
	Residual []float64 `json:"residual"`
}

// TreatmentPayback is one risk's treatment plan seen as an investment: what it
// costs, how much annual loss it removes, and how many months of avoided loss
// pay for it (#904). Money lives on the risk's plan (remediation cost and
// expected effectiveness), not on individual mitigations.
type TreatmentPayback struct {
	RiskID        uuid.UUID `json:"risk_id"`
	Title         string    `json:"title"`
	Cost          crq.Money `json:"cost"`
	Reduction     crq.Money `json:"reduction"` // annual loss avoided
	PaybackMonths float64   `json:"payback_months"`
}

// CriticalityBucket is the aggregated annual loss for one criticality band.
type CriticalityBucket struct {
	Criticality string    `json:"criticality"`
	Count       int       `json:"count"`
	ALE         crq.Money `json:"ale"`
}

// TopRiskFinancial is one row of the "biggest financial exposures" table.
type TopRiskFinancial struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Criticality string    `json:"criticality"`
	ALE         crq.Money `json:"ale"`
	ALEWorst    crq.Money `json:"ale_worst"`
	// ALEAfter is the exposure targeted once the mitigations are in place
	// (#903: the executive view draws it beside the inherent ALE).
	ALEAfter crq.Money `json:"ale_after"`
	ROSI     float64   `json:"rosi"`
	ROSIOK   bool      `json:"rosi_computable"`
}

// FinancialSummary is the tenant-wide financial posture rendered by the CFO/CISO
// dashboard: portfolio ALE (current, worst-case, residual), remediation budget,
// portfolio ROSI, a breakdown by criticality and the top exposures.
type FinancialSummary struct {
	Currency        string    `json:"currency"`    // tenant display currency
	FXRateXAF       float64   `json:"fx_rate_xaf"` // XAF per 1 unit of Currency
	FXAsOf          time.Time `json:"fx_as_of"`    // reference date of the rate
	XAFPerUSD       float64   `json:"xaf_per_usd"`
	ComputedAt      time.Time `json:"computed_at"` // when this snapshot was computed
	FormulaVersion  string    `json:"formula_version"`
	Iterations      int       `json:"iterations"`
	TotalRisks      int       `json:"total_risks"`
	QuantifiedRisks int       `json:"quantified_risks"` // from SQL aggregate, not a client filter
	// PortfolioLoss is the FAIR-lite P10/P50/P90 band of TOTAL annual exposure —
	// the headline figure. A single number is a false certainty (spec §2).
	PortfolioLoss      crq.DistributionAmounts `json:"portfolio_loss"`
	TotalALE           crq.Money               `json:"total_ale"`
	TotalALEWorst      crq.Money               `json:"total_ale_worst"`
	TotalALEAfter      crq.Money               `json:"total_ale_after"`      // residual after modeled controls
	TotalRiskReduction crq.Money               `json:"total_risk_reduction"` // benefit of modeled controls
	TotalRemediation   crq.Money               `json:"total_remediation"`
	PortfolioROSI      float64                 `json:"portfolio_rosi"`
	PortfolioROSIOK    bool                    `json:"portfolio_rosi_computable"`
	ByCriticality      []CriticalityBucket     `json:"by_criticality"`
	TopRisks           []TopRiskFinancial      `json:"top_risks"`

	// #904 — the financial page of the October 2026 redesign.
	// PortfolioROSI3Y is (3 × annual reduction − plan cost) / plan cost.
	PortfolioROSI3Y   float64            `json:"portfolio_rosi_3y"`
	PortfolioROSI3YOK bool               `json:"portfolio_rosi_3y_computable"`
	TreatedRisks      int                `json:"treated_risks"` // risks whose plan has a cost
	LossExceedance    *LossExceedance    `json:"loss_exceedance"`
	Treatments        []TreatmentPayback `json:"treatments"` // shortest payback first
	RiskAppetiteXAF   *float64           `json:"risk_appetite_xaf"`
}

// FinancialSummaryUseCase aggregates the CRQ model across a tenant's register.
type FinancialSummaryUseCase struct {
	lister     FinancialRiskLister
	quantifier *crq.Quantifier
	presenters *FinancialPresenterFactory // optional; nil → XAF/static
	coverage   FinancialCoverageCounter   // optional; nil → derived from the list
	appetite   RiskAppetiteReader         // optional; nil → no appetite
	now        func() time.Time
}

// NewFinancialSummaryUseCase builds the use case.
func NewFinancialSummaryUseCase(lister FinancialRiskLister, quantifier *crq.Quantifier) *FinancialSummaryUseCase {
	return &FinancialSummaryUseCase{lister: lister, quantifier: quantifier, now: time.Now}
}

// WithPresenters attaches the currency/FX presenter factory.
func (uc *FinancialSummaryUseCase) WithPresenters(f *FinancialPresenterFactory) *FinancialSummaryUseCase {
	uc.presenters = f
	return uc
}

// WithCoverageCounter attaches the SQL coverage aggregate.
func (uc *FinancialSummaryUseCase) WithCoverageCounter(c FinancialCoverageCounter) *FinancialSummaryUseCase {
	uc.coverage = c
	return uc
}

// WithAppetiteReader attaches the tenant risk-appetite reader.
func (uc *FinancialSummaryUseCase) WithAppetiteReader(r RiskAppetiteReader) *FinancialSummaryUseCase {
	uc.appetite = r
	return uc
}

// WithClock overrides the clock (tests).
func (uc *FinancialSummaryUseCase) WithClock(now func() time.Time) *FinancialSummaryUseCase {
	uc.now = now
	return uc
}

// topRiskLimit caps the "biggest exposures" table.
const topRiskLimit = 10

// Execute computes the financial summary for a tenant.
func (uc *FinancialSummaryUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*FinancialSummary, error) {
	risks, err := uc.lister.ListRisksForFinancial(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("failed to list risks for financial summary: " + err.Error())
	}

	q := uc.quantifier
	pres := uc.presenter(ctx, tenantID)
	rates := pres.Rates

	sum := &FinancialSummary{
		Currency:       string(pres.Currency),
		FXRateXAF:      rates.RateFor(pres.Currency),
		FXAsOf:         rates.AsOf,
		XAFPerUSD:      q.XAFPerUSD,
		ComputedAt:     uc.clock()(),
		FormulaVersion: crq.FormulaVersion,
		Iterations:     crq.DefaultIterations,
		TotalRisks:     len(risks),
	}

	// Running XAF totals; USD is derived once at the end for consistent rounding.
	var totalALE, totalWorst, totalAfter, totalReduction, totalRemediation float64
	var derivedQuantified int
	bands := map[string]*CriticalityBucket{}
	bandOrder := []string{"critical", "high", "medium", "low"}
	for _, b := range bandOrder {
		bands[b] = &CriticalityBucket{Criticality: b}
	}

	tops := make([]TopRiskFinancial, 0, len(risks))
	sims := make([]crq.SimulationInput, 0, len(risks))
	residualSims := make([]crq.SimulationInput, 0, len(risks))
	treatments := make([]TreatmentPayback, 0, len(risks))

	for i := range risks {
		r := &risks[i]
		in := financialInputs(r)
		// Deterministic per-risk figures (no per-risk Monte Carlo); the band comes
		// from ONE shared portfolio simulation below.
		a := q.AssessDeterministic(in, string(r.Criticality))
		sim := q.SimulationInputFor(in, string(r.Criticality))
		sims = append(sims, sim)
		residualSims = append(residualSims, crq.ScaleLoss(sim, 1-a.Effectiveness))

		if a.RemediationCost.XAF > 0 {
			sum.TreatedRisks++
			if a.RiskReduction.XAF > 0 {
				treatments = append(treatments, TreatmentPayback{
					RiskID:        r.ID,
					Title:         riskTitle(r),
					Cost:          a.RemediationCost,
					Reduction:     a.RiskReduction,
					PaybackMonths: math.Round(a.RemediationCost.XAF/a.RiskReduction.XAF*12*10) / 10,
				})
			}
		}

		totalALE += a.ALE.XAF
		totalWorst += a.ALEWorst.XAF
		totalAfter += a.ALEAfter.XAF
		totalReduction += a.RiskReduction.XAF
		totalRemediation += a.RemediationCost.XAF

		if a.SLEBasis == crq.BasisExplicit || a.SLEBasis == crq.BasisComposed {
			derivedQuantified++
		}

		band := strings.ToLower(strings.TrimSpace(string(r.Criticality)))
		if b, ok := bands[band]; ok {
			b.Count++
			b.ALE.XAF += a.ALE.XAF
		}

		tops = append(tops, TopRiskFinancial{
			ID:          r.ID,
			Title:       riskTitle(r),
			Criticality: string(r.Criticality),
			ALE:         a.ALE,
			ALEWorst:    a.ALEWorst,
			ALEAfter:    a.ALEAfter,
			ROSI:        a.ROSI,
			ROSIOK:      a.ROSIComputable,
		})
	}

	// Quantified count: prefer the SQL aggregate (server fact); fall back to the
	// per-risk derivation only if no counter is wired.
	sum.TotalRisks, sum.QuantifiedRisks = len(risks), derivedQuantified
	if uc.coverage != nil {
		if total, quantified, cErr := uc.coverage.CountFinancialCoverage(ctx, tenantID); cErr == nil {
			sum.TotalRisks, sum.QuantifiedRisks = total, quantified
		}
	}

	// Headline: one shared portfolio Monte Carlo → P10/P50/P90 of total exposure.
	sum.PortfolioLoss = pres.Present(crq.SimulatePortfolio(sims, crq.DefaultIterations, crq.DefaultSeed))

	sum.TotalALE = q.Money(totalALE)
	sum.TotalALEWorst = q.Money(totalWorst)
	sum.TotalALEAfter = q.Money(totalAfter)
	sum.TotalRiskReduction = q.Money(totalReduction)
	sum.TotalRemediation = q.Money(totalRemediation)

	// Portfolio ROSI over all modeled controls.
	sum.PortfolioROSI, sum.PortfolioROSIOK = crq.ROSI(totalALE, totalAfter, totalRemediation)
	if totalRemediation > 0 {
		sum.PortfolioROSI3Y = math.Round((3*totalReduction-totalRemediation)/totalRemediation*100) / 100
		sum.PortfolioROSI3YOK = true
	}

	// Loss-exceedance curve, before and after the plans, from the same draws.
	if len(sims) > 0 {
		sum.LossExceedance = &LossExceedance{
			Inherent: crq.PortfolioQuantiles(sims, crq.DefaultIterations, crq.DefaultSeed),
			Residual: crq.PortfolioQuantiles(residualSims, crq.DefaultIterations, crq.DefaultSeed),
		}
	}

	sort.SliceStable(treatments, func(i, j int) bool {
		return treatments[i].PaybackMonths < treatments[j].PaybackMonths
	})
	sum.Treatments = treatments

	if uc.appetite != nil {
		// A failing read leaves the appetite unset rather than failing the page.
		if v, aErr := uc.appetite.OrgRiskAppetite(ctx, tenantID); aErr == nil {
			sum.RiskAppetiteXAF = v
		}
	}

	// Emit criticality buckets in a stable order with USD derived.
	for _, b := range bandOrder {
		bucket := bands[b]
		bucket.ALE = q.Money(bucket.ALE.XAF)
		sum.ByCriticality = append(sum.ByCriticality, *bucket)
	}

	// Top exposures by current ALE, descending.
	sort.SliceStable(tops, func(i, j int) bool { return tops[i].ALE.XAF > tops[j].ALE.XAF })
	if len(tops) > topRiskLimit {
		tops = tops[:topRiskLimit]
	}
	sum.TopRisks = tops

	return sum, nil
}

// presenter resolves the tenant currency + rate table (XAF/static fallback).
func (uc *FinancialSummaryUseCase) presenter(ctx context.Context, tenantID uuid.UUID) crq.Presenter {
	if uc.presenters != nil {
		return uc.presenters.For(ctx, tenantID)
	}
	return crq.NewPresenter(crq.CurrencyXAF, crq.DefaultRateTable(), uc.quantifier.XAFPerUSD)
}

// clock returns the injected clock (defaulting to time.Now).
func (uc *FinancialSummaryUseCase) clock() func() time.Time {
	if uc.now != nil {
		return uc.now
	}
	return time.Now
}

// financialInputs maps a risk's stored drivers onto the CRQ input struct. Kept
// here (not in the handler helper) so the use case has no handler dependency.
func financialInputs(r *domain.Risk) crq.FinancialInputs {
	return crq.FinancialInputs{
		SLEXAF:                  r.SLEXAF,
		ARO:                     r.ARO,
		DowntimeHours:           r.DowntimeHours,
		HourlyDowntimeCostXAF:   r.HourlyDowntimeCostXAF,
		DataLossCostXAF:         r.DataLossCostXAF,
		FinesXAF:                r.FinesXAF,
		OtherDirectCostXAF:      r.OtherDirectCostXAF,
		RemediationCostXAF:      r.RemediationCostXAF,
		MitigationEffectiveness: r.MitigationEffectiveness,
	}
}

// riskTitle prefers Title, falling back to Name (the two are kept in sync but a
// narrow SELECT may populate only one).
func riskTitle(r *domain.Risk) string {
	if strings.TrimSpace(r.Title) != "" {
		return r.Title
	}
	return r.Name
}
