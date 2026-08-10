// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/application/score"
	handlers "github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/service"
)

// Adapters mapping the existing use cases / legacy services onto the narrow ports
// the score use case declares. Same pattern as executive_wiring.go: the adapter
// lives here, at the composition root, so neither side has to know about the
// other's shape.

// complianceCoverageAdapter turns the gap analysis into the two numbers the
// control-gap factor needs.
type complianceCoverageAdapter struct {
	gaps *compliance.GetGapAnalysisUseCase
}

// ControlTotals returns the tenant's applicable controls and how many are gaps.
func (a complianceCoverageAdapter) ControlTotals(ctx context.Context, tenantID uuid.UUID) (int, int, error) {
	analysis, err := a.gaps.Execute(ctx, tenantID, uuid.Nil)
	if err != nil || analysis == nil {
		return 0, 0, err
	}
	return analysis.TotalControls, analysis.TotalGaps, nil
}

// incidentPressureAdapter exposes open/critical incident counts from the legacy
// incident service, which is not context-aware and keys on a string tenant.
type incidentPressureAdapter struct {
	svc *service.IncidentService
}

// OpenIncidentCounts returns (open, criticalOpen).
func (a incidentPressureAdapter) OpenIncidentCounts(_ context.Context, tenantID uuid.UUID) (int, int, error) {
	analytics, err := a.svc.GetIncidentAnalytics(tenantID.String(), 1)
	if err != nil || analytics == nil {
		return 0, 0, err
	}
	return analytics.OpenCount, analytics.CriticalOpen, nil
}

// newScoreHandler assembles the ONE score endpoint from the already-constructed,
// tenant-scoped sources. Every source is optional in the use case, so a missing
// one degrades its own factor instead of failing the request.
func newScoreHandler(
	riskRepo *repository.GormRiskRepository,
	assetRepo *repository.GormAssetRepository,
	vulnRepo *repository.GormVulnerabilityRepository,
	mitigationRepo *repository.GormMitigationRepository,
	gapUC *compliance.GetGapAnalysisUseCase,
	incidentSvc *service.IncidentService,
) *handlers.ScoreHandler {
	uc := score.New().
		WithRiskCounts(riskRepo).
		WithRisk(riskRepo).
		WithRisks(riskRepo).
		WithAssets(assetRepo).
		WithVulnStats(vulnRepo).
		WithVulns(vulnRepo).
		WithMitigations(mitigationRepo).
		WithCompliance(complianceCoverageAdapter{gaps: gapUC}).
		WithIncidents(incidentPressureAdapter{svc: incidentSvc})
	return handlers.NewScoreHandler(uc)
}
