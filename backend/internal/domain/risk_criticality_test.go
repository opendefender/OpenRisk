// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import "testing"

// TestCriticalityFromScore pins the synchronous band computation to the Score
// Engine thresholds (>=7 critical, >=4 high, >=2 medium, <2 low), lowercase to
// match what the ScoreWorker persists (audit-2026 #246).
func TestCriticalityFromScore(t *testing.T) {
	cases := []struct {
		score float64
		want  CriticalityLevel
	}{
		{10.0, CriticalityCriticalNew},
		{7.0, CriticalityCriticalNew},
		{6.999, CriticalityHighNew},
		{4.0, CriticalityHighNew},
		{3.999, CriticalityMediumNew},
		{2.0, CriticalityMediumNew},
		{1.999, CriticalityLowNew},
		{0.0, CriticalityLowNew},
	}
	for _, c := range cases {
		if got := CriticalityFromScore(c.score); got != c.want {
			t.Errorf("CriticalityFromScore(%v) = %q, want %q", c.score, got, c.want)
		}
	}
}
