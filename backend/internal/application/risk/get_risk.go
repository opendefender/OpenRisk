// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
