// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

// Trend series status (#756). A day with no risk in the register has no
// average score, and a series with fewer than MinTrendPoints such days is not
// a trend. The trend endpoints say so with TrendStatusInsufficientData instead
// of drawing a flat line of zeros that reads as "no risk".
const (
	TrendStatusOK               = "ok"
	TrendStatusInsufficientData = "insufficient_data"

	// MinTrendPoints is the fewest dated points carrying at least one risk
	// before a series is reported as a trend.
	MinTrendPoints = 2
)

// RiskTrendStatus reports whether an analytics risk trend holds enough history.
func RiskTrendStatus(points []RiskTrendPoint) string {
	counts := make([]int64, len(points))
	for i, p := range points {
		counts[i] = p.Count
	}
	return trendStatus(counts)
}

// DashboardTrendStatus reports whether a dashboard risk trend holds enough history.
func DashboardTrendStatus(points []RiskTrendDataPoint) string {
	counts := make([]int64, len(points))
	for i, p := range points {
		counts[i] = p.Count
	}
	return trendStatus(counts)
}

func trendStatus(counts []int64) string {
	withData := 0
	for _, c := range counts {
		if c > 0 {
			withData++
		}
	}
	if withData < MinTrendPoints {
		return TrendStatusInsufficientData
	}
	return TrendStatusOK
}
