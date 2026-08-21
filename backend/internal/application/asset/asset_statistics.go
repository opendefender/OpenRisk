// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/timeframe"
)

// StatisticsReader is the narrow port this use case needs.
//
// Declared here rather than on domain.AssetRepository so the concrete
// GormAssetRepository satisfies it structurally and no existing mock has to grow
// a method it never calls — the same arrangement ListRisksForFinancial uses.
type StatisticsReader interface {
	Statistics(ctx context.Context, tenantID uuid.UUID, from, to *time.Time) (*domain.AssetStatistics, error)
}

// AssetStatisticsUseCase answers "what is in the inventory" without loading it.
type AssetStatisticsUseCase struct {
	repo StatisticsReader
}

func NewAssetStatisticsUseCase(repo StatisticsReader) *AssetStatisticsUseCase {
	return &AssetStatisticsUseCase{repo: repo}
}

// Execute counts the tenant's inventory over the given window.
//
// The window narrows exactly ONE field — AddedInPeriod. Everything else is a
// point-in-time stock and is counted in full, on purpose:
//
//	"How many critical assets do we have?" does not become a different question
//	when someone picks a date range. Filtering the stock by created_at would
//	answer "how many critical assets did we ADD last month", print it under a
//	label that says otherwise, and put the dashboard permanently at odds with the
//	inventory screen it links to.
//
// The response therefore reports which fields the period touched, and the client
// renders the period control as applying to those and no others.
func (uc *AssetStatisticsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, w timeframe.Window) (*domain.AssetStatistics, error) {
	if tenantID == uuid.Nil {
		// Fail closed. Counting every tenant's assets because the context was
		// not resolved is the one outcome worse than an error.
		return nil, domain.NewValidationError("tenant is required")
	}
	if uc.repo == nil {
		return nil, domain.NewInternalError("asset statistics source not configured")
	}

	var from *time.Time
	to := w.To
	if !w.IsAll() {
		f := w.From
		from = &f
	}

	stats, err := uc.repo.Statistics(ctx, tenantID, from, &to)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return stats, nil
}
