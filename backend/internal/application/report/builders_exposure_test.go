// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crq"
)

// fakeRisks serves each tenant only its own risks, as the GORM repository does
// with its tenant_id filter.
type fakeRisks struct {
	byTenant map[uuid.UUID][]domain.Risk
	asked    []uuid.UUID
}

func (f *fakeRisks) ListRisksForFinancial(_ context.Context, tenantID uuid.UUID) ([]domain.Risk, error) {
	f.asked = append(f.asked, tenantID)
	return f.byTenant[tenantID], nil
}

func ptr(v float64) *float64 { return &v }

// The issue's case: one risk with its own loss figures (SLE 20 M × ARO 0.5 =
// 10 M) and one valued by the reference model (medium = 3 M).
func issueRisks(tenantID uuid.UUID) []domain.Risk {
	return []domain.Risk{
		{ID: uuid.New(), TenantID: tenantID, Name: "Ransomware ERP", Criticality: "medium",
			Score: 5, SLEXAF: ptr(20_000_000), ARO: ptr(0.5)},
		{ID: uuid.New(), TenantID: tenantID, Name: "Phishing", Criticality: "medium", Score: 3},
	}
}

func referenceOnlyRisks(tenantID uuid.UUID) []domain.Risk {
	return []domain.Risk{
		{ID: uuid.New(), TenantID: tenantID, Name: "Phishing", Criticality: "medium", Score: 3},
		{ID: uuid.New(), TenantID: tenantID, Name: "Insider", Criticality: "high", Score: 6},
	}
}

func execRequest(tenantID uuid.UUID, t domain.ReportType) Request {
	tpl, _ := TemplateFor(t)
	return Request{TenantID: tenantID, Type: t, Locale: domain.ReportLocaleFR, Template: tpl}
}

func summaryValue(c Content, label string) string {
	for _, kv := range c.Summary {
		if kv.Label == label {
			return kv.Value
		}
	}
	return ""
}

func registerALE(t *testing.T, c Content) map[string]string {
	t.Helper()
	require.Len(t, c.Sections, 1)
	require.NotNil(t, c.Sections[0].Table)
	out := map[string]string{}
	for _, row := range c.Sections[0].Table.Rows {
		out[row[0]] = row[4]
	}
	return out
}

// screenTotalALE is total_ale as /analytics/financial computes it for the same
// risks: the figure the report must equal.
func screenTotalALE(t *testing.T, q *crq.Quantifier, risks []domain.Risk) float64 {
	t.Helper()
	tenantID := risks[0].TenantID
	uc := risk.NewFinancialSummaryUseCase(&fakeRisks{byTenant: map[uuid.UUID][]domain.Risk{tenantID: risks}}, q)
	sum, err := uc.Execute(context.Background(), tenantID)
	require.NoError(t, err)
	return sum.TotalALE.XAF
}

func TestBuildExecutive_Success(t *testing.T) {
	tenantID := uuid.New()
	risks := issueRisks(tenantID)
	q := crq.NewQuantifier(crq.DefaultXAFPerUSD, crq.DefaultReference())
	src := Sources{Risks: &fakeRisks{byTenant: map[uuid.UUID][]domain.Risk{tenantID: risks}}, Quantifier: q}

	c, err := Build(context.Background(), src, execRequest(tenantID, domain.ReportTypeExecutiveSummary), nil)
	require.NoError(t, err)

	// 10 M + 3 M, and the same total the financial screen shows.
	assert.InDelta(t, 13_000_000, screenTotalALE(t, q, risks), 0.01)
	assert.Equal(t, "13.0 M FCFA", summaryValue(c, "Exposition annuelle"))
	body := strings.Join(c.Sections[0].Body, "\n")
	assert.Contains(t, body, "exposition annuelle attendue 13.0 M FCFA")
	assert.Contains(t, body, "dont 3.0 M FCFA estimés par le modèle de référence pour 1 risque(s)")
	assert.NotContains(t, body, "0 FCFA.")
}

func TestBuildExecutive_NotFound(t *testing.T) {
	// No quantified risk: the report says so and prints no amount.
	tenantID := uuid.New()
	src := Sources{Risks: &fakeRisks{byTenant: map[uuid.UUID][]domain.Risk{tenantID: referenceOnlyRisks(tenantID)}}}

	c, err := Build(context.Background(), src, execRequest(tenantID, domain.ReportTypeExecutiveSummary), nil)
	require.NoError(t, err)

	assert.Equal(t, "Non quantifiée", summaryValue(c, "Exposition annuelle"))
	body := strings.Join(c.Sections[0].Body, "\n")
	assert.Contains(t, body, "exposition annuelle non quantifiée")
	assert.NotContains(t, body, "FCFA")

	// An empty register is not quantified either.
	empty, err := Build(context.Background(), Sources{Risks: &fakeRisks{}},
		execRequest(uuid.New(), domain.ReportTypeExecutiveSummary), nil)
	require.NoError(t, err)
	assert.Equal(t, "Non quantifiée", summaryValue(empty, "Exposition annuelle"))
}

func TestBuildExecutive_Unauthorized(t *testing.T) {
	// Another tenant's quantified risks never reach this tenant's total.
	mine, other := uuid.New(), uuid.New()
	src := &fakeRisks{byTenant: map[uuid.UUID][]domain.Risk{
		mine:  referenceOnlyRisks(mine),
		other: issueRisks(other),
	}}

	c, err := Build(context.Background(), Sources{Risks: src}, execRequest(mine, domain.ReportTypeExecutiveSummary), nil)
	require.NoError(t, err)

	assert.Equal(t, []uuid.UUID{mine}, src.asked)
	assert.Equal(t, "Non quantifiée", summaryValue(c, "Exposition annuelle"))
}

func TestBuildRiskRegister_Success(t *testing.T) {
	tenantID := uuid.New()
	src := Sources{Risks: &fakeRisks{byTenant: map[uuid.UUID][]domain.Risk{tenantID: issueRisks(tenantID)}}}

	c, err := Build(context.Background(), src, execRequest(tenantID, domain.ReportTypeRiskRegister), nil)
	require.NoError(t, err)

	ale := registerALE(t, c)
	assert.Equal(t, "10.0 M FCFA", ale["Ransomware ERP"])
	assert.Equal(t, "3.0 M FCFA (modèle de référence)", ale["Phishing"])

	en := execRequest(tenantID, domain.ReportTypeRiskRegister)
	en.Locale = domain.ReportLocaleEN
	c, err = Build(context.Background(), src, en, nil)
	require.NoError(t, err)
	assert.Equal(t, "3.0 M FCFA (reference model)", registerALE(t, c)["Phishing"])
}

func TestBuildRiskRegister_NotFound(t *testing.T) {
	// A register with no quantified risk prints no amount on any row.
	tenantID := uuid.New()
	src := Sources{Risks: &fakeRisks{byTenant: map[uuid.UUID][]domain.Risk{tenantID: referenceOnlyRisks(tenantID)}}}

	c, err := Build(context.Background(), src, execRequest(tenantID, domain.ReportTypeRiskRegister), nil)
	require.NoError(t, err)

	for name, v := range registerALE(t, c) {
		assert.Equal(t, "Non quantifiée", v, name)
		assert.NotContains(t, v, "FCFA", name)
	}

	// Without a risk source the register is refused, not printed empty.
	_, err = Build(context.Background(), Sources{}, execRequest(tenantID, domain.ReportTypeRiskRegister), nil)
	require.Error(t, err)
}

func TestBuildRiskRegister_Unauthorized(t *testing.T) {
	mine, other := uuid.New(), uuid.New()
	src := &fakeRisks{byTenant: map[uuid.UUID][]domain.Risk{
		mine:  issueRisks(mine)[:1],
		other: referenceOnlyRisks(other),
	}}

	c, err := Build(context.Background(), Sources{Risks: src}, execRequest(mine, domain.ReportTypeRiskRegister), nil)
	require.NoError(t, err)

	assert.Equal(t, []uuid.UUID{mine}, src.asked)
	ale := registerALE(t, c)
	assert.Len(t, ale, 1)
	assert.NotContains(t, ale, "Insider")
}
