// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package risk

import (
	"context"
	"sort"
	"strings"

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
	ROSI        float64   `json:"rosi"`
	ROSIOK      bool      `json:"rosi_computable"`
}

// FinancialSummary is the tenant-wide financial posture rendered by the CFO/CISO
// dashboard: portfolio ALE (current, worst-case, residual), remediation budget,
// portfolio ROSI, a breakdown by criticality and the top exposures.
type FinancialSummary struct {
	Currency           string              `json:"currency"`
	XAFPerUSD          float64             `json:"xaf_per_usd"`
	TotalRisks         int                 `json:"total_risks"`
	QuantifiedRisks    int                 `json:"quantified_risks"` // risks with explicit SLE or components
	TotalALE           crq.Money           `json:"total_ale"`
	TotalALEWorst      crq.Money           `json:"total_ale_worst"`
	TotalALEAfter      crq.Money           `json:"total_ale_after"`      // residual after modeled controls
	TotalRiskReduction crq.Money           `json:"total_risk_reduction"` // benefit of modeled controls
	TotalRemediation   crq.Money           `json:"total_remediation"`
	PortfolioROSI      float64             `json:"portfolio_rosi"`
	PortfolioROSIOK    bool                `json:"portfolio_rosi_computable"`
	ByCriticality      []CriticalityBucket `json:"by_criticality"`
	TopRisks           []TopRiskFinancial  `json:"top_risks"`
}

// FinancialSummaryUseCase aggregates the CRQ model across a tenant's register.
type FinancialSummaryUseCase struct {
	lister     FinancialRiskLister
	quantifier *crq.Quantifier
}

// NewFinancialSummaryUseCase builds the use case.
func NewFinancialSummaryUseCase(lister FinancialRiskLister, quantifier *crq.Quantifier) *FinancialSummaryUseCase {
	return &FinancialSummaryUseCase{lister: lister, quantifier: quantifier}
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
	sum := &FinancialSummary{
		Currency:   "XAF",
		XAFPerUSD:  q.XAFPerUSD,
		TotalRisks: len(risks),
	}

	// Running XAF totals; USD is derived once at the end for consistent rounding.
	var totalALE, totalWorst, totalAfter, totalReduction, totalRemediation float64
	bands := map[string]*CriticalityBucket{}
	bandOrder := []string{"critical", "high", "medium", "low"}
	for _, b := range bandOrder {
		bands[b] = &CriticalityBucket{Criticality: b}
	}

	tops := make([]TopRiskFinancial, 0, len(risks))

	for i := range risks {
		r := &risks[i]
		in := financialInputs(r)
		a := q.Assess(in, string(r.Criticality))

		totalALE += a.ALE.XAF
		totalWorst += a.ALEWorst.XAF
		totalAfter += a.ALEAfter.XAF
		totalReduction += a.RiskReduction.XAF
		totalRemediation += a.RemediationCost.XAF

		if a.SLEBasis == crq.BasisExplicit || a.SLEBasis == crq.BasisComposed {
			sum.QuantifiedRisks++
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
			ROSI:        a.ROSI,
			ROSIOK:      a.ROSIComputable,
		})
	}

	sum.TotalALE = q.Money(totalALE)
	sum.TotalALEWorst = q.Money(totalWorst)
	sum.TotalALEAfter = q.Money(totalAfter)
	sum.TotalRiskReduction = q.Money(totalReduction)
	sum.TotalRemediation = q.Money(totalRemediation)

	// Portfolio ROSI over all modeled controls.
	sum.PortfolioROSI, sum.PortfolioROSIOK = crq.ROSI(totalALE, totalAfter, totalRemediation)

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
