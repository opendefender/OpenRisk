// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import "time"

// RiskTrend is the direction a risk's score has moved over the trend window.
type RiskTrend string

const (
	RiskTrendUp   RiskTrend = "up"
	RiskTrendDown RiskTrend = "down"
	RiskTrendFlat RiskTrend = "flat"
)

// ScoreTrendThreshold is the largest score change still called flat. One rule
// for the register arrows and for the per-risk timeline (ComputeRiskTrend).
const ScoreTrendThreshold = 0.5

// TrendWindowDays is how far back the baseline snapshot is looked for.
const TrendWindowDays = 30

// ScoreTrend classifies the move from baseline to current. |delta| <= 0.5 is
// flat; the boundary itself is flat.
func ScoreTrend(baseline, current float64) RiskTrend {
	delta := current - baseline
	switch {
	case delta > ScoreTrendThreshold:
		return RiskTrendUp
	case delta < -ScoreTrendThreshold:
		return RiskTrendDown
	default:
		return RiskTrendFlat
	}
}

// TrendBaselineCutoff is the instant the baseline snapshot is taken at or
// before: now minus the trend window.
func TrendBaselineCutoff(now time.Time) time.Time {
	return now.UTC().AddDate(0, 0, -TrendWindowDays)
}

// TrendSnapshots is what a repository reads for one risk: the score of the
// latest history snapshot taken at or before the cutoff (nil when there is
// none) and the score of the oldest snapshot. A risk with no snapshot at all
// has no entry.
type TrendSnapshots struct {
	AtCutoff *float64
	Oldest   float64
}

// Baseline picks the score to compare with: the latest snapshot at or before
// the cutoff, else the oldest snapshot.
func (s TrendSnapshots) Baseline() float64 {
	if s.AtCutoff != nil {
		return *s.AtCutoff
	}
	return s.Oldest
}
