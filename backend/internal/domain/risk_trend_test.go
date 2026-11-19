// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScoreTrend(t *testing.T) {
	cases := []struct {
		name              string
		baseline, current float64
		want              RiskTrend
	}{
		{"clear rise", 3, 4, RiskTrendUp},
		{"clear fall", 4, 3, RiskTrendDown},
		{"no change", 3, 3, RiskTrendFlat},
		{"just over the threshold up", 3, 3.51, RiskTrendUp},
		{"just over the threshold down", 3, 2.49, RiskTrendDown},
		{"exactly +0.5 is flat", 3, 3.5, RiskTrendFlat},
		{"exactly -0.5 is flat", 3, 2.5, RiskTrendFlat},
		{"from zero", 0, 0.4, RiskTrendFlat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ScoreTrend(tc.baseline, tc.current))
		})
	}
	assert.Equal(t, 0.5, ScoreTrendThreshold)
}

func TestTrendSnapshots_Baseline(t *testing.T) {
	atCutoff := 6.0
	assert.Equal(t, 6.0, TrendSnapshots{AtCutoff: &atCutoff, Oldest: 2}.Baseline(), "the snapshot at the cutoff beats the oldest")
	assert.Equal(t, 2.0, TrendSnapshots{Oldest: 2}.Baseline(), "no snapshot that old: the oldest one")

	zero := 0.0
	assert.Equal(t, 0.0, TrendSnapshots{AtCutoff: &zero, Oldest: 5}.Baseline(), "a recorded score of 0 is a baseline, not an absence")
}

func TestTrendBaselineCutoff(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), TrendBaselineCutoff(now))
}
