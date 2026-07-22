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

// MarkRiskReviewedUseCase records that a risk was reviewed now and schedules the
// next review from the risk's cadence (ReviewIntervalDays). It reuses the
// existing repository (GetByID + Update) so it stays tenant-scoped.
type MarkRiskReviewedUseCase struct {
	riskRepo domain.RiskRepository
}

func NewMarkRiskReviewedUseCase(riskRepo domain.RiskRepository) *MarkRiskReviewedUseCase {
	return &MarkRiskReviewedUseCase{riskRepo: riskRepo}
}

func (uc *MarkRiskReviewedUseCase) Execute(ctx context.Context, orgID, riskID uuid.UUID) (*domain.Risk, error) {
	r, err := uc.riskRepo.GetByID(ctx, riskID, orgID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}
	now := time.Now()
	r.LastReviewedAt = &now
	if r.ReviewIntervalDays > 0 {
		next := now.Add(time.Duration(r.ReviewIntervalDays) * 24 * time.Hour)
		r.NextReviewAt = &next
	} else {
		r.NextReviewAt = nil
	}
	if err := uc.riskRepo.Update(ctx, r); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return r, nil
}
