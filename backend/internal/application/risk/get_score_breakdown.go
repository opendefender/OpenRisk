// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pkgscoring "github.com/opendefender/openrisk/pkg/scoring"
	"github.com/opendefender/openrisk/internal/domain"
)

// GetScoreBreakdownUseCase retrieves detailed score calculation for a risk
// Shows the components and how the score was calculated
type GetScoreBreakdownUseCase struct {
	riskRepo      domain.RiskRepository
	scoringEngine pkgscoring.Engine
}

// NewGetScoreBreakdownUseCase creates a new GetScoreBreakdownUseCase
func NewGetScoreBreakdownUseCase(
	riskRepo domain.RiskRepository,
	scoringEngine pkgscoring.Engine,
) *GetScoreBreakdownUseCase {
	return &GetScoreBreakdownUseCase{
		riskRepo:      riskRepo,
		scoringEngine: scoringEngine,
	}
}

// Execute retrieves the score breakdown for a risk
// Shows probability, impact, asset criticality, resulting score, and criticality level
func (uc *GetScoreBreakdownUseCase) Execute(ctx context.Context, tenantID uuid.UUID, riskID uuid.UUID) (*pkgscoring.ScoreBreakdown, error) {
	// 1. Fetch risk (tenant-scoped)
	risk, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// 2. Calculate asset criticality (default to MEDIUM = 1.5 if no asset linked)
	assetCriticality := 1.5 // Default MEDIUM criticality
	if risk.AssetID != nil && len(risk.Assets) > 0 {
		// Use the first linked asset's criticality
		asset := risk.Assets[0]
		switch asset.Criticality {
		case domain.CriticalityLow:
			assetCriticality = 0.5
		case domain.CriticalityMedium:
			assetCriticality = 1.5
		case domain.CriticalityHigh:
			assetCriticality = 2.5
		case domain.CriticalityCritical:
			assetCriticality = 3.0
		}
	}

	// 3. Use Score Engine to compute breakdown
	// IMPORTANT: All score calculations go through the Score Engine
	// This ensures consistency with the official formula
	oldScore := risk.Score
	breakdown, err := uc.scoringEngine.Breakdown(
		risk.Probability,
		risk.Impact,
		assetCriticality,
		&oldScore,
	)
	if err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to calculate score breakdown: %v", err))
	}

	return &breakdown, nil
}
