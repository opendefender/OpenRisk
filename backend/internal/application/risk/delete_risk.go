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

// DeleteRiskUseCase handles deleting a risk.
type DeleteRiskUseCase struct {
	riskRepo domain.RiskRepository
}

func NewDeleteRiskUseCase(riskRepo domain.RiskRepository) *DeleteRiskUseCase {
	return &DeleteRiskUseCase{riskRepo: riskRepo}
}

// Execute soft-deletes a risk by ID, scoped to the organization.
func (uc *DeleteRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, riskID uuid.UUID) error {
	// Verify the risk exists and belongs to this org
	risk, err := uc.riskRepo.GetByID(ctx, riskID, orgID)
	if err != nil {
		return err
	}
	if risk == nil {
		return domain.NewNotFoundError("risk", riskID)
	}

	if err := uc.riskRepo.Delete(ctx, riskID, orgID); err != nil {
		return domain.NewInternalError("failed to delete risk")
	}

	return nil
}
