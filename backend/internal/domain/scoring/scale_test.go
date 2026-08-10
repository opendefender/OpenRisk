// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package scoring

import (
	"math"
	"testing"
)

// The boundary table required by the spec. Every one of these nine values is an
// edge of a band, and each was a place the old client-side mappings disagreed.
func TestBandFor_Boundaries(t *testing.T) {
	cases := []struct {
		value float64
		want  Band
	}{
		{0, BandLow},
		{24, BandLow},
		{25, BandMedium}, // floor is INCLUSIVE
		{49, BandMedium},
		{50, BandHigh},
		{74, BandHigh},
		{75, BandCritical},
		{99, BandCritical},
		{100, BandCritical},
	}
	for _, tc := range cases {
		if got := BandFor(tc.value); got != tc.want {
			t.Errorf("BandFor(%v) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// Just below each floor must fall into the band below. Floating point makes this
// worth pinning explicitly rather than trusting the table above.
func TestBandFor_JustBelowFloors(t *testing.T) {
	cases := []struct {
		value float64
		want  Band
	}{
		{24.999, BandLow},
		{49.999, BandMedium},
		{74.999, BandHigh},
	}
	for _, tc := range cases {
		if got := BandFor(tc.value); got != tc.want {
			t.Errorf("BandFor(%v) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// THE RANGE DEFECT, as a non-regression test.
//
// Scores used to be produced on the 0–30 P×I×AC scale and rendered as "x / 100",
// while bands were cut at 7 — so 77 % of the possible range collapsed into one
// label and out-of-range inputs were never clamped at all. Nothing may leave
// [0,100], and nothing outside it may produce a band outside the ladder.
func TestClampAndBand_NeverEscapeTheScale(t *testing.T) {
	rogue := []float64{
		-1, -0.0001, -100, -1e9,
		100.0001, 101, 30, 300, 1e9,
		math.NaN(), math.Inf(1), math.Inf(-1),
	}
	for _, v := range rogue {
		got := Clamp(v)
		if got < MinScore || got > MaxScore {
			t.Errorf("Clamp(%v) = %v, outside [%v,%v]", v, got, MinScore, MaxScore)
		}
		if math.IsNaN(got) {
			t.Errorf("Clamp(%v) produced NaN", v)
		}
		if r := BandFor(v).Rank(); r < 0 || r > 3 {
			t.Errorf("BandFor(%v) produced an off-ladder band %q", v, BandFor(v))
		}
	}

	// A value that cannot be computed is the FLOOR, never the ceiling: an
	// unmeasurable score must not present as maximum risk.
	if Clamp(math.NaN()) != MinScore {
		t.Errorf("NaN must clamp to the floor, got %v", Clamp(math.NaN()))
	}
	if Clamp(math.Inf(-1)) != MinScore || Clamp(math.Inf(1)) != MaxScore {
		t.Error("infinities must clamp to the ends of the scale")
	}
}

// The band ladder must be monotonic in the value: a higher score can never be
// less severe. Swept across the whole scale at fine resolution.
func TestBandFor_MonotonicInValue(t *testing.T) {
	prev := BandFor(MinScore).Rank()
	for v := MinScore; v <= MaxScore; v += 0.1 {
		rank := BandFor(v).Rank()
		if rank < prev {
			t.Fatalf("band went DOWN at %v (%d → %d)", v, prev, rank)
		}
		prev = rank
	}
	if BandFor(MaxScore) != BandCritical || BandFor(MinScore) != BandLow {
		t.Error("the ladder must span low..critical across the full scale")
	}
}

// Every band ships an i18n key, and no two bands share one — the label the client
// renders is the server's, and it must be unambiguous.
func TestBand_I18nKeysAreDistinct(t *testing.T) {
	seen := map[string]Band{}
	for _, b := range []Band{BandLow, BandMedium, BandHigh, BandCritical} {
		key := b.I18nKey()
		if key == "score.band." {
			t.Errorf("band %q has an empty i18n key", b)
		}
		if other, dup := seen[key]; dup {
			t.Errorf("bands %q and %q share the i18n key %q", other, b, key)
		}
		seen[key] = b
	}
}

func TestNormalise(t *testing.T) {
	cases := []struct {
		value, min, max, want float64
	}{
		{0, 0, 1, 0},
		{1, 0, 1, 100},
		{0.5, 0, 1, 50},
		{5, 0, 10, 50},
		{0.1, 0.1, 3.0, 0},   // the floor of asset criticality is 0, not 3.3
		{3.0, 0.1, 3.0, 100}, // and its ceiling is a full 100
		{-5, 0, 10, 0},       // below range clamps
		{50, 0, 10, 100},     // above range clamps
	}
	for _, tc := range cases {
		if got := normalise(tc.value, tc.min, tc.max); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("normalise(%v, %v, %v) = %v, want %v", tc.value, tc.min, tc.max, got, tc.want)
		}
	}

	// A degenerate range must not produce ±Inf.
	if got := normalise(5, 3, 3); got != MinScore {
		t.Errorf("a zero-width range must yield the floor, got %v", got)
	}
	if got := normalise(math.NaN(), 0, 10); got != MinScore {
		t.Errorf("NaN must yield the floor, got %v", got)
	}
}

func TestRound1(t *testing.T) {
	cases := [][2]float64{{63.44, 63.4}, {63.45, 63.5}, {0, 0}, {99.99, 100}}
	for _, tc := range cases {
		if got := Round1(tc[0]); math.Abs(got-tc[1]) > 1e-9 {
			t.Errorf("Round1(%v) = %v, want %v", tc[0], got, tc[1])
		}
	}
}

func TestParseScope(t *testing.T) {
	for _, ok := range []string{"tenant", "risk", "asset"} {
		if _, valid := ParseScope(ok); !valid {
			t.Errorf("%q should be a valid scope", ok)
		}
	}
	for _, bad := range []string{"", "TENANT", "org", "user"} {
		if _, valid := ParseScope(bad); valid {
			t.Errorf("%q should not be a valid scope", bad)
		}
	}
}
