// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseDuration_SameGrammarAsTheValidator(t *testing.T) {
	for _, s := range []string{"P30D", "P3M", "P1Y", "P2W", "PT36H", "P1DT12H", "P1Y2M3W4DT5H6M7S", "P0DT1S",
		"", "P", "PT", "P0D", "30D", "P1.5D", "P1D2M", "P1M1M", "PT1D", "p1d", "P-1D", "P1DT", "P99999999D"} {
		_, ok := ParseDuration(s)
		want := isISO8601Duration(s) && s != "P99999999D"
		assert.Equal(t, want, ok, s)
	}
	assert.True(t, isISO8601Duration("P99999999D"), "the validator keeps its grammar; only the parser bounds the size")
}

func TestDurationAddTo_CalendarArithmetic(t *testing.T) {
	jan31 := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		cadence string
		from    time.Time
		want    time.Time
	}{
		{"P30D", jan31, time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)},
		{"P1M", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{"P1Y", time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)},
		{"P1W", jan31, time.Date(2026, 2, 7, 10, 0, 0, 0, time.UTC)},
		{"PT36H", jan31, time.Date(2026, 2, 1, 22, 0, 0, 0, time.UTC)},
		{"P1DT1H30M15S", jan31, time.Date(2026, 2, 1, 11, 30, 15, 0, time.UTC)},
	}
	for _, c := range cases {
		d, ok := ParseDuration(c.cadence)
		assert.True(t, ok, c.cadence)
		assert.True(t, c.want.Equal(d.AddTo(c.from)), "%s from %s: got %s", c.cadence, c.from, d.AddTo(c.from))
	}
}
