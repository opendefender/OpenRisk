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

// mockFinancialLister is a tiny stand-in for the narrow FinancialRiskLister port.
type mockFinancialLister struct {
	risks []domain.Risk
	err   error
}

func (m *mockFinancialLister) ListRisksForFinancial(ctx context.Context, tenantID uuid.UUID) ([]domain.Risk, error) {
	return m.risks, m.err
}

func fp(v float64) *float64 { return &v }

func TestFinancialSummary_Success(t *testing.T) {
	tenantID := uuid.New()
	risks := []domain.Risk{
		{ // explicit SLE×ARO = 10M, remediation 2M @ 80% → ALEAfter 2M, reduction 8M
			ID: uuid.New(), Title: "Ransomware", Criticality: domain.RiskCriticalityCritical,
			SLEXAF: fp(20_000_000), ARO: fp(0.5),
			RemediationCostXAF: fp(2_000_000), MitigationEffectiveness: fp(0.8),
		},
		{ // composed SLE from components = 3M, ARO 1 → ALE 3M, no control
			ID: uuid.New(), Title: "Data leak", Criticality: domain.RiskCriticalityHigh,
			FinesXAF: fp(2_000_000), DataLossCostXAF: fp(1_000_000), ARO: fp(1),
		},
	}
	uc := NewFinancialSummaryUseCase(&mockFinancialLister{risks: risks}, crq.NewQuantifier(600, crq.DefaultReference()))

	sum, err := uc.Execute(context.Background(), tenantID)
	require.NoError(t, err)
	require.NotNil(t, sum)

	assert.Equal(t, 2, sum.TotalRisks)
	assert.Equal(t, 2, sum.QuantifiedRisks) // both have explicit or composed SLE
	// Portfolio ALE = 10M + 3M = 13M.
	assert.Equal(t, 13_000_000.0, sum.TotalALE.XAF)
	// Residual = 2M (control on #1) + 3M (uncontrolled #2) = 5M.
	assert.Equal(t, 5_000_000.0, sum.TotalALEAfter.XAF)
	// Reduction = 8M.
	assert.Equal(t, 8_000_000.0, sum.TotalRiskReduction.XAF)
	// Remediation budget = 2M.
	assert.Equal(t, 2_000_000.0, sum.TotalRemediation.XAF)
	// Portfolio ROSI = (13M − 5M − 2M) / 2M = 3.0.
	assert.True(t, sum.PortfolioROSIOK)
	assert.InDelta(t, 3.0, sum.PortfolioROSI, 0.01)
	// USD derived at 600.
	assert.InDelta(t, 21_666.67, sum.TotalALE.USD, 0.1)

	// Top exposures sorted by ALE desc: Ransomware (10M) before Data leak (3M).
	require.Len(t, sum.TopRisks, 2)
	assert.Equal(t, "Ransomware", sum.TopRisks[0].Title)
	assert.Equal(t, 10_000_000.0, sum.TopRisks[0].ALE.XAF)

	// Criticality buckets always emitted in the 4-band order.
	require.Len(t, sum.ByCriticality, 4)
	assert.Equal(t, "critical", sum.ByCriticality[0].Criticality)
	assert.Equal(t, 10_000_000.0, sum.ByCriticality[0].ALE.XAF)
	assert.Equal(t, 3_000_000.0, sum.ByCriticality[1].ALE.XAF) // high
}

func TestFinancialSummary_Empty(t *testing.T) {
	uc := NewFinancialSummaryUseCase(&mockFinancialLister{risks: nil}, crq.NewQuantifier(600, crq.DefaultReference()))
	sum, err := uc.Execute(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 0, sum.TotalRisks)
	assert.Equal(t, 0.0, sum.TotalALE.XAF)
	assert.False(t, sum.PortfolioROSIOK) // no remediation budget → undefined
	assert.Len(t, sum.ByCriticality, 4)  // buckets still present, all zero
	assert.Empty(t, sum.TopRisks)
}

func TestFinancialSummary_ListerError(t *testing.T) {
	uc := NewFinancialSummaryUseCase(&mockFinancialLister{err: errors.New("db down")}, crq.NewQuantifier(600, crq.DefaultReference()))
	sum, err := uc.Execute(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Nil(t, sum)
}
