// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgscoring "github.com/opendefender/openrisk/pkg/scoring"
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

	// 2. Calculate asset criticality — average domain.AssetCriticality.ScoreFactor()
	// across every linked asset (not just the first one), consistent with how
	// GormRiskRepository.GetRisksByAssetID/RiskHandler now derive it. Defaults
	// to MEDIUM's factor (1.5) if no asset is linked.
	assetCriticality := domain.CriticalityMedium.ScoreFactor()
	if len(risk.Assets) > 0 {
		var sum float64
		for _, a := range risk.Assets {
			sum += a.Criticality.ScoreFactor()
		}
		assetCriticality = sum / float64(len(risk.Assets))
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
