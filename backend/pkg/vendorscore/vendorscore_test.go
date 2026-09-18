// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package vendorscore

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pts(v float64) *float64 { return &v }

func choice(weight float64, points *float64) Item {
	return Item{ID: uuid.New(), Scorable: true, Weight: weight, Points: points}
}

func TestCompute(t *testing.T) {
	tests := []struct {
		name      string
		items     []Item
		wantScore *float64
		wantTier  Tier
	}{
		{"every weighted answer favourable", []Item{choice(5, pts(1)), choice(3, pts(1))}, pts(0), TierLow},
		{"every weighted answer unfavourable", []Item{choice(5, pts(0)), choice(3, pts(0))}, pts(100), TierCritical},
		{"equal weights, one of two favourable", []Item{choice(5, pts(1)), choice(5, pts(0))}, pts(50), TierHigh},
		{"weights matter: the heavy question failed", []Item{choice(8, pts(0)), choice(2, pts(1))}, pts(80), TierCritical},
		{"weights matter: the light question failed", []Item{choice(8, pts(1)), choice(2, pts(0))}, pts(20), TierMedium},
		{"partial points", []Item{choice(3, pts(0.5)), choice(7, pts(0.2))}, pts(71), TierCritical},
		{"an unanswered question counts as unfavourable", []Item{choice(5, pts(1)), choice(5, nil)}, pts(50), TierHigh},
		{"a fully unanswered questionnaire is 100, not 'not scorable'", []Item{choice(5, nil), choice(5, nil)}, pts(100), TierCritical},
		{"N/A leaves both sums", []Item{choice(5, pts(1)), {ID: uuid.New(), Scorable: true, Weight: 5, NA: true}}, pts(0), TierLow},
		{"a text question never scores", []Item{choice(5, pts(1)), {ID: uuid.New(), Scorable: false, Weight: 10, Points: pts(0)}}, pts(0), TierLow},
		{"a zero weight never scores", []Item{choice(5, pts(0)), choice(0, pts(1))}, pts(100), TierCritical},
		{"points above 1 are clamped", []Item{choice(5, pts(1.5))}, pts(0), TierLow},
		{"negative points are clamped", []Item{choice(5, pts(-2))}, pts(100), TierCritical},
		{"NaN points count as unfavourable", []Item{choice(5, pts(math.NaN()))}, pts(100), TierCritical},
		{"no item", nil, nil, ""},
		{"only text questions", []Item{{ID: uuid.New(), Scorable: false, Weight: 5}}, nil, ""},
		{"only N/A answers", []Item{{ID: uuid.New(), Scorable: true, Weight: 5, NA: true}}, nil, ""},
		{"only zero weights", []Item{choice(0, pts(0))}, nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.items)

			assert.Equal(t, Version, got.Version)
			assert.NotNil(t, got.Breakdown, "the breakdown is an array, never nil")
			if tt.wantScore == nil {
				assert.Nil(t, got.Score, "nothing scorable is null, never 0")
				assert.Nil(t, got.Tier)
				return
			}
			require.NotNil(t, got.Score)
			require.NotNil(t, got.Tier)
			assert.InDelta(t, *tt.wantScore, *got.Score, 0.0001)
			assert.Equal(t, tt.wantTier, *got.Tier)
		})
	}
}

func TestTierFor_Thresholds(t *testing.T) {
	tests := []struct {
		score float64
		want  Tier
	}{
		{100, TierCritical},
		{70, TierCritical},
		{69.99, TierHigh},
		{40, TierHigh},
		{39.99, TierMedium},
		{20, TierMedium},
		{19.99, TierLow},
		{0, TierLow},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, TierFor(tt.score), "score %.2f", tt.score)
	}
}

// ADR 0004 D5: the contributions sum to the score, so the UI shows the
// arithmetic instead of recomputing it.
func TestCompute_BreakdownSumsToTheScore(t *testing.T) {
	na := Item{ID: uuid.New(), Scorable: true, Weight: 4, NA: true}
	text := Item{ID: uuid.New(), Scorable: false, Weight: 0}
	items := []Item{choice(3, pts(0.25)), choice(7, nil), na, choice(1.5, pts(0.9)), text}

	got := Compute(items)

	require.NotNil(t, got.Score)
	require.Len(t, got.Breakdown, 4, "every weighted choice question, N/A included; the text question is not listed")
	var sum float64
	for _, c := range got.Breakdown {
		sum += c.Contribution
	}
	assert.InDelta(t, *got.Score, sum, 0.005)

	assert.Equal(t, na.ID, got.Breakdown[2].ItemID, "input order is kept")
	assert.True(t, got.Breakdown[2].NA)
	assert.Zero(t, got.Breakdown[2].Contribution)
	assert.Zero(t, got.Breakdown[1].Points, "an unanswered question is recorded at 0 points")
}

func TestCompute_IsDeterministic(t *testing.T) {
	items := []Item{choice(3, pts(0.25)), choice(7, nil), choice(1.5, pts(0.9))}

	a, b := Compute(items), Compute(items)

	assert.Equal(t, *a.Score, *b.Score)
	assert.Equal(t, a.Breakdown, b.Breakdown)
}
