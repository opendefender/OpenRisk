// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package timeframe

import (
	"testing"
	"time"
)

// A fixed "now" in the middle of a day, so every assertion about day boundaries
// is about the boundary logic and not about when the suite happened to run.
var now = time.Date(2026, 8, 21, 13, 45, 12, 0, time.UTC)

func TestParse_DefaultIsUnbounded(t *testing.T) {
	w, err := Parse("", "", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !w.IsAll() {
		t.Fatalf("default window should be unbounded, got preset %q", w.Preset)
	}
	// The headline counters are stock quantities; defaulting them to a window
	// would make the dashboard disagree with the register on first paint.
	if !w.From.IsZero() {
		t.Fatalf("unbounded window must have no lower bound, got %s", w.From)
	}
	if got, want := w.To, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("to = %s, want end of the current UTC day %s", got, want)
	}
}

func TestParse_PresetsCountTodayIn(t *testing.T) {
	cases := []struct {
		preset Preset
		from   time.Time
	}{
		{Preset7d, time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)},
		{Preset30d, time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)},
		{Preset90d, time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		w, err := Parse(string(c.preset), "", "", now)
		if err != nil {
			t.Fatalf("%s: %v", c.preset, err)
		}
		if !w.From.Equal(c.from) {
			t.Errorf("%s: from = %s, want %s", c.preset, w.From, c.from)
		}
		if got := w.Days(); got != mustDays(c.preset) {
			t.Errorf("%s: days = %d, want %d", c.preset, got, mustDays(c.preset))
		}
		// "Last 7 days" must include today, or the chart's right edge is
		// yesterday and every user reads it as data loss.
		if !w.Contains(now) {
			t.Errorf("%s: window excludes now", c.preset)
		}
	}
}

func mustDays(p Preset) int {
	switch p {
	case Preset7d:
		return 7
	case Preset30d:
		return 30
	case Preset90d:
		return 90
	}
	return 0
}

func TestParse_RejectsUnknownPresetRatherThanDefaulting(t *testing.T) {
	// The dangerous behaviour is answering ?period=6m with the all-time numbers
	// and no indication that the filter was ignored.
	for _, bad := range []string{"6m", "yesterday", "1", "custom", "30days"} {
		if _, err := Parse(bad, "", "", now); err == nil {
			t.Errorf("period=%q was accepted; it must be rejected", bad)
		}
	}
}

func TestParse_CustomBoundsAreHalfOpen(t *testing.T) {
	w, err := Parse("", "2026-08-01", "2026-09-01", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Preset != PresetCustom {
		t.Fatalf("preset = %q, want custom", w.Preset)
	}
	aug1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	sep1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if !w.Contains(aug1) {
		t.Error("from must be inclusive")
	}
	if w.Contains(sep1) {
		t.Error("to must be exclusive")
	}
	// Consecutive windows must tile: the instant that ends August starts
	// September and belongs to exactly one of them.
	sep, _ := Parse("", "2026-09-01", "2026-10-01", now)
	if !sep.Contains(sep1) {
		t.Error("the next window must own the shared boundary")
	}
}

func TestParse_RejectsMalformedRanges(t *testing.T) {
	cases := []struct{ name, from, to string }{
		{"unparseable from", "not-a-date", "2026-09-01"},
		{"unparseable to", "2026-08-01", "31/12/2026"},
		{"inverted", "2026-09-01", "2026-08-01"},
		{"empty range", "2026-08-01", "2026-08-01"},
		{"to without from", "", "2026-09-01"},
	}
	for _, c := range cases {
		if _, err := Parse("", c.from, c.to, now); err == nil {
			t.Errorf("%s: accepted from=%q to=%q", c.name, c.from, c.to)
		}
	}
}

func TestParse_RejectsExcessiveRange(t *testing.T) {
	// One bucket per day means an unbounded range is an unbounded response.
	if _, err := Parse("", "2020-01-01", "2026-01-01", now); err == nil {
		t.Fatal("a six-year range was accepted; it must be rejected")
	}
	// Exactly at the cap is allowed.
	if _, err := Parse("", "2025-08-21", "2026-08-22", now); err != nil {
		t.Fatalf("a %d-day range must be allowed: %v", MaxCustomDays, err)
	}
}

func TestParse_RejectsPresetAndBoundsTogether(t *testing.T) {
	if _, err := Parse("30d", "2026-08-01", "2026-09-01", now); err == nil {
		t.Fatal("period + from/to was accepted; the two disagree and guessing is the bug")
	}
}

func TestParse_FutureToIsAcceptedAsAsked(t *testing.T) {
	w, err := Parse("", "2026-08-01", "2026-08-31", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Clamping would rewrite the period the response echoes back, which is worse
	// than a range reaching a few days past today over rows that do not exist.
	if got, want := w.To, time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("to = %s, want %s (unclamped)", got, want)
	}
}

func TestGranularityAndTrendBounds(t *testing.T) {
	d7, _ := Parse("7d", "", "", now)
	if d7.Granularity() != "day" {
		t.Errorf("7d granularity = %q, want day", d7.Granularity())
	}
	d90, _ := Parse("90d", "", "", now)
	if d90.Granularity() != "day" {
		t.Errorf("90d granularity = %q, want day", d90.Granularity())
	}
	all, _ := Parse("all", "", "", now)
	if all.Granularity() != "week" {
		t.Errorf("all granularity = %q, want week", all.Granularity())
	}

	from, to, gran := all.TrendBounds()
	if gran != "week" {
		t.Errorf("trend granularity = %q, want week", gran)
	}
	// `all` counts everything but the SERIES is capped, and the caller must be
	// able to report the bounds actually used.
	if want := to.AddDate(0, 0, -TrendCap); !from.Equal(want) {
		t.Errorf("trend from = %s, want %s (capped)", from, want)
	}
}

func TestResolved_OmitsFromWhenUnbounded(t *testing.T) {
	all, _ := Parse("all", "", "", now)
	r := all.Resolved()
	if r.From != "" {
		t.Errorf("from = %q, want empty — an unbounded window has no start date to invent", r.From)
	}
	if r.To == "" {
		t.Error("to must always be reported")
	}
	if r.Preset != "all" {
		t.Errorf("preset = %q, want all", r.Preset)
	}

	d30, _ := Parse("30d", "", "", now)
	if d30.Resolved().From == "" {
		t.Error("a bounded window must report its start")
	}
}
