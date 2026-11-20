// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RiskSummary feeds the register header, the view tabs and the band pills
// (GET /risks/summary, #913).
type RiskSummary struct {
	// Views counts every view over the whole register, ignoring the view and
	// search the caller is looking at: a tab shows its own size.
	Views map[RiskView]int64 `json:"views"`
	// ByCriticality counts inside the selected view and search. The other list
	// filters are ignored so a band pill shows what picking it would add.
	ByCriticality map[CriticalityLevel]int64 `json:"by_criticality"`
	Active        RiskActiveSummary          `json:"active"`
}

// RiskActiveSummary is the not-closed part of the register, whole tenant.
type RiskActiveSummary struct {
	Count int64 `json:"count"`
	// ALEXAF is the summed annual loss expectancy, null when the tenant's plan
	// does not include financial quantification (never 0: 0 is a figure).
	ALEXAF *float64 `json:"ale_xaf"`
}

// RiskExposureInput is what the per-risk ALE is computed from.
type RiskExposureInput struct {
	SLEXAF      *float64
	ARO         *float64
	Criticality CriticalityLevel
}

// RiskSummaryRepository reads the aggregates behind RiskSummary. Every method
// is scoped to the tenant it is given.
type RiskSummaryRepository interface {
	// ViewCounts counts every view over the tenant's register. userID resolves
	// the "mine" view.
	ViewCounts(ctx context.Context, tenantID, userID uuid.UUID) (map[RiskView]int64, error)
	// CriticalityCounts counts by criticality level inside a view and a search.
	CriticalityCounts(ctx context.Context, tenantID, userID uuid.UUID, view RiskView, search string) (map[CriticalityLevel]int64, error)
	// CountActive counts the tenant's risks that are not closed.
	CountActive(ctx context.Context, tenantID uuid.UUID) (int64, error)
	// ActiveExposureInputs returns the ALE inputs of every not-closed risk.
	ActiveExposureInputs(ctx context.Context, tenantID uuid.UUID) ([]RiskExposureInput, error)
}

// RiskTrendReader reads, for a page of risks in one batch, the snapshots the
// trend baseline is chosen from. It is tenant-scoped through the parent risk:
// risk_histories has no tenant column.
type RiskTrendReader interface {
	TrendSnapshots(ctx context.Context, tenantID uuid.UUID, riskIDs []uuid.UUID, cutoff time.Time) (map[uuid.UUID]TrendSnapshots, error)
}
