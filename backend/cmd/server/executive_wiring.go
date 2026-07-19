// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package main

import (
	"context"

	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/application/dashboard"
	"github.com/opendefender/openrisk/internal/application/risk"
	handlers "github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/crq"
)

// incidentSourceAdapter maps the legacy incident service onto the executive
// dashboard's IncidentSource port, keeping the service decoupled from the
// application layer.
type incidentSourceAdapter struct{ svc *service.IncidentService }

func (a incidentSourceAdapter) IncidentAnalytics(_ context.Context, tenantID string, months int) (*dashboard.IncidentAnalytics, error) {
	d, err := a.svc.GetIncidentAnalytics(tenantID, months)
	if err != nil {
		return nil, err
	}
	trend := make([]dashboard.IncidentTrendPoint, 0, len(d.Trend))
	for _, t := range d.Trend {
		trend = append(trend, dashboard.IncidentTrendPoint{Month: t.Month, Total: t.Total, Critical: t.Critical, High: t.High})
	}
	return &dashboard.IncidentAnalytics{
		Total:          d.Total,
		OpenCount:      d.OpenCount,
		CriticalOpen:   d.CriticalOpen,
		AvgMTTRDays:    d.AvgMTTRDays,
		ResolutionRate: d.ResolutionRate,
		Trend:          trend,
	}, nil
}

// newExecutiveDashboardHandler assembles the executive dashboard aggregation from
// the already-constructed, tenant-scoped sources (spec §11). All sources are
// optional in the use case, so wiring order and nil-safety are handled there.
func newExecutiveDashboardHandler(
	financialUC *risk.FinancialSummaryUseCase,
	riskRepo *repository.GormRiskRepository,
	gapUC *compliance.GetGapAnalysisUseCase,
	vulnRepo *repository.GormVulnerabilityRepository,
	incidentSvc *service.IncidentService,
	quantifier *crq.Quantifier,
) *handlers.ExecutiveDashboardHandler {
	uc := dashboard.NewGetExecutiveDashboardUseCase().
		WithFinancial(financialUC).
		WithRisks(riskRepo).
		WithCompliance(gapUC).
		WithVulnerabilities(vulnRepo).
		WithIncidents(incidentSourceAdapter{svc: incidentSvc}).
		WithQuantifier(quantifier)
	return handlers.NewExecutiveDashboardHandler(uc)
}
