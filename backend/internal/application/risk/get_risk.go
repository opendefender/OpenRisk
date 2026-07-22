// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// GetRiskUseCase handles retrieving a single risk.
type GetRiskUseCase struct {
	riskRepo domain.RiskRepository
}

func NewGetRiskUseCase(riskRepo domain.RiskRepository) *GetRiskUseCase {
	return &GetRiskUseCase{riskRepo: riskRepo}
}

// Execute retrieves a risk by ID, scoped to the organization.
func (uc *GetRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, riskID uuid.UUID) (*domain.Risk, error) {
	risk, err := uc.riskRepo.GetByID(ctx, riskID, orgID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}
	return risk, nil
}
