// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// GetRiskWeightsUseCase returns a tenant's smart-risk factor weights, falling back
// to the engine defaults when the tenant has never customised them.
type GetRiskWeightsUseCase struct {
	repo domain.RiskScoringWeightsRepository
}

func NewGetRiskWeightsUseCase(repo domain.RiskScoringWeightsRepository) *GetRiskWeightsUseCase {
	return &GetRiskWeightsUseCase{repo: repo}
}

// Execute returns the effective weights for a tenant (custom or default).
func (uc *GetRiskWeightsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*domain.RiskScoringWeights, error) {
	w, err := uc.repo.GetByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return domain.DefaultRiskScoringWeights(tenantID), nil
	}
	return w, nil
}

// UpdateRiskWeightsInput carries the eight configurable factor weights. Weights
// are relative — the engine normalises them — but each must be within [0,1].
type UpdateRiskWeightsInput struct {
	BusinessCriticality float64
	InternetExposure    float64
	Vulnerabilities     float64
	ControlMaturity     float64
	IncidentHistory     float64
	Exploitability      float64
	FinancialValue      float64
	ThreatIntel         float64
}

// UpdateRiskWeightsUseCase persists a tenant's custom factor weights. Admin-guarded
// at the route; tenant-scoped; validates the configuration is usable.
type UpdateRiskWeightsUseCase struct {
	repo domain.RiskScoringWeightsRepository
}

func NewUpdateRiskWeightsUseCase(repo domain.RiskScoringWeightsRepository) *UpdateRiskWeightsUseCase {
	return &UpdateRiskWeightsUseCase{repo: repo}
}

// Execute validates and upserts the tenant's weights row.
func (uc *UpdateRiskWeightsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, actorID uuid.UUID, in UpdateRiskWeightsInput) (*domain.RiskScoringWeights, error) {
	// Start from the current effective weights so partial concerns are explicit:
	// the API always sends the full set, but this keeps tenant_id/created_at intact.
	existing, err := uc.repo.GetByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	w := existing
	if w == nil {
		w = domain.DefaultRiskScoringWeights(tenantID)
	}

	w.BusinessCriticality = in.BusinessCriticality
	w.InternetExposure = in.InternetExposure
	w.Vulnerabilities = in.Vulnerabilities
	w.ControlMaturity = in.ControlMaturity
	w.IncidentHistory = in.IncidentHistory
	w.Exploitability = in.Exploitability
	w.FinancialValue = in.FinancialValue
	w.ThreatIntel = in.ThreatIntel
	w.UpdatedBy = actorID
	w.UpdatedAt = time.Now()

	if err := w.Validate(); err != nil {
		return nil, err
	}
	if err := uc.repo.Upsert(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}
