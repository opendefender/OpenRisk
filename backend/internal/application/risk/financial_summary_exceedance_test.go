// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The financial page of the redesign (#904): exceedance curve before and after
// the plans, treatment payback, three-year ROSI and the tenant's appetite.

type mockAppetite struct {
	v   *float64
	err error
}

func (m mockAppetite) OrgRiskAppetite(context.Context, uuid.UUID) (*float64, error) {
	return m.v, m.err
}

func pageRisks() []domain.Risk {
	return []domain.Risk{
		{ // ALE 10M, plan 2M at 80 % → reduction 8M/yr, payback 3 months
			ID: uuid.New(), Title: "Ransomware", Criticality: domain.RiskCriticalityCritical,
			SLEXAF: fp(20_000_000), ARO: fp(0.5),
			RemediationCostXAF: fp(2_000_000), MitigationEffectiveness: fp(0.8),
		},
		{ // ALE 6M, plan 6M at 50 % → reduction 3M/yr, payback 24 months
			ID: uuid.New(), Title: "Fraud", Criticality: domain.RiskCriticalityHigh,
			SLEXAF: fp(6_000_000), ARO: fp(1),
			RemediationCostXAF: fp(6_000_000), MitigationEffectiveness: fp(0.5),
		},
		{ // a plan with a cost but no effect: treated, never in the payback list
			ID: uuid.New(), Title: "Unproven", Criticality: domain.RiskCriticalityMedium,
			SLEXAF: fp(1_000_000), ARO: fp(1), RemediationCostXAF: fp(500_000),
		},
		{ // no plan
			ID: uuid.New(), Title: "Untreated", Criticality: domain.RiskCriticalityLow,
			SLEXAF: fp(2_000_000), ARO: fp(1),
		},
	}
}

func newPageUC(risks []domain.Risk) *FinancialSummaryUseCase {
	return NewFinancialSummaryUseCase(&mockFinancialLister{risks: risks}, crq.NewQuantifier(600, crq.DefaultReference()))
}

func TestFinancialSummaryPage_Success(t *testing.T) {
	appetite := 12_000_000.0
	sum, err := newPageUC(pageRisks()).WithAppetiteReader(mockAppetite{v: &appetite}).
		Execute(context.Background(), uuid.New())
	require.NoError(t, err)

	assert.Equal(t, 3, sum.TreatedRisks)
	require.Len(t, sum.Treatments, 2, "a plan without effect has no payback")
	assert.Equal(t, "Ransomware", sum.Treatments[0].Title)
	assert.Equal(t, 3.0, sum.Treatments[0].PaybackMonths)
	assert.Equal(t, 8_000_000.0, sum.Treatments[0].Reduction.XAF)
	assert.Equal(t, "Fraud", sum.Treatments[1].Title)
	assert.Equal(t, 24.0, sum.Treatments[1].PaybackMonths)

	// Plan cost 8.5M, annual reduction 11M → (33 − 8.5) / 8.5.
	assert.True(t, sum.PortfolioROSI3YOK)
	assert.InDelta(t, 2.88, sum.PortfolioROSI3Y, 0.005)

	require.NotNil(t, sum.LossExceedance)
	inh, res := sum.LossExceedance.Inherent, sum.LossExceedance.Residual
	require.Len(t, inh, 101)
	require.Len(t, res, 101)
	for p := range inh {
		assert.LessOrEqual(t, res[p], inh[p], "after the plans a year never costs more (p%d)", p)
	}
	// The median of the curve is the headline band's P50.
	assert.Equal(t, sum.PortfolioLoss.P50.XAF, inh[50])

	require.NotNil(t, sum.RiskAppetiteXAF)
	assert.Equal(t, appetite, *sum.RiskAppetiteXAF)
}

// An empty register has no curve, no payback and no ROSI, and says so with
// nulls rather than zeros that read like measurements.
func TestFinancialSummaryPage_NotFound(t *testing.T) {
	sum, err := newPageUC(nil).Execute(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, sum.LossExceedance)
	assert.Empty(t, sum.Treatments)
	assert.False(t, sum.PortfolioROSI3YOK)
	assert.Nil(t, sum.RiskAppetiteXAF)
}

// A failing appetite read leaves the appetite unset; the page still loads.
func TestFinancialSummaryPage_AppetiteReadFails(t *testing.T) {
	sum, err := newPageUC(pageRisks()).WithAppetiteReader(mockAppetite{err: errors.New("db down")}).
		Execute(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, sum.RiskAppetiteXAF)
	assert.NotNil(t, sum.LossExceedance)
}
