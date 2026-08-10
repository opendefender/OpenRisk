// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package scoring is the SINGLE place a displayed score is calculated.
//
// It is pure: stdlib only, no Fiber, no GORM, no I/O, no clock. Everything here
// is a deterministic function of its inputs, which is what makes the property
// tests in scale_test.go / compute_test.go meaningful.
//
// WHY THIS PACKAGE EXISTS — the bugs it makes unrepresentable:
//
//	(a) "dashboard ≠ sidebar". The two read different endpoints computing
//	    different quantities on different scales (a 0–100 composite "cyber
//	    score" where higher is better, versus a raw P×I×AC exposure where
//	    higher is worse). Two numbers, both labelled "score". Now there is one
//	    computation, one scale, one meaning.
//
//	(b) "the label is stuck on low while the number moves". The band was derived
//	    on the CLIENT, in four mutually incompatible places, from thresholds
//	    calibrated for a different scale than the number being displayed:
//	      · shared/riskColors.ts       ≥7 / ≥4 / ≥2      (for the 0–30 P×I×AC scale)
//	      · RiskRegisterPage.tsx       ≥15 / ≥8 / ≥4     (same scale, other cuts)
//	      · docs/AUDIT_MANIFEST.md     ≥15 / ≥9 / ≥5     (a third set, shipped)
//	      · MitigationCard.tsx         ≥40               (a 0–100 scale)
//	    Here the band is computed WITH the value, by BandFor, and travels with it
//	    in the same response. Label/value desynchronisation is now structurally
//	    impossible: there is no code path that produces one without the other.
//
//	(c) The range defect. Scores on the 0–30 P×I×AC scale were rendered as
//	    "x / 100" (see RiskCard), so a maximal risk (30) displayed as 30 % of the
//	    scale, and the bands cut that scale at 7 — meaning 77 % of the possible
//	    range collapsed into the single "critical" label, which is precisely why
//	    the label stopped tracking the number. The canonical scale is now 0–100
//	    with evenly spaced quartile bands, and every producer clamps into it.
package scoring

import "math"

// FormulaVersion identifies the calculation. It travels in every response so a
// stored score can always be traced back to the model that produced it, and so a
// stale cached value is recognisable rather than silently wrong.
//
// Bump it whenever weights, factors or band boundaries change.
const FormulaVersion = "2.1"

// The canonical scale. EVERY score this product displays lives here — there is no
// second scale, and no caller may invent one.
const (
	MinScore = 0.0
	MaxScore = 100.0
)

// Band boundaries, evenly spaced across the canonical scale.
//
// Even spacing is a deliberate correction. The previous thresholds cut the range
// at 23 % of its span, so nearly everything real landed in the top band and the
// label stopped carrying information. Quartiles make the label move with the
// number, which is the entire point of having a label.
const (
	BandMediumFloor   = 25.0
	BandHighFloor     = 50.0
	BandCriticalFloor = 75.0
)

// Band is the qualitative reading of a score.
//
// The vocabulary is the product's existing one (low / medium / high / critical) —
// the same words used by domain.AssetCriticality, by the vulnerability tiers, by
// pkg/scoring's smart model, and by the --low/--medium/--high/--critical design
// tokens. Introducing a fifth vocabulary for one more surface is how a codebase
// ends up with four incompatible band mappings in the first place.
type Band string

const (
	BandLow      Band = "low"
	BandMedium   Band = "medium"
	BandHigh     Band = "high"
	BandCritical Band = "critical"
)

// I18nKey is the translation key the client renders. The client never derives a
// label from the number — it prints what the server sent.
func (b Band) I18nKey() string { return "score.band." + string(b) }

// Rank orders bands from least to most severe. Used by the monotonicity property
// test: a higher score may never yield a less severe band.
func (b Band) Rank() int {
	switch b {
	case BandCritical:
		return 3
	case BandHigh:
		return 2
	case BandMedium:
		return 1
	default:
		return 0
	}
}

// Clamp forces a value into the canonical range.
//
// Every producer runs through this, so [0,100] is an invariant of the type rather
// than a promise each call site has to keep. NaN maps to the floor: a score that
// cannot be computed is not "maximum risk", and rendering NaN as a bar width is
// how a dashboard silently draws nothing.
func Clamp(v float64) float64 {
	if math.IsNaN(v) {
		return MinScore
	}
	if v < MinScore {
		return MinScore
	}
	if v > MaxScore {
		return MaxScore
	}
	return v
}

// BandFor maps a value to its band. It clamps first, so an out-of-range input
// cannot produce an out-of-range band.
//
// Boundaries are INCLUSIVE at the floor: exactly 25 is medium, exactly 50 is
// high, exactly 75 is critical. The boundary table test pins all nine edges.
func BandFor(v float64) Band {
	switch v = Clamp(v); {
	case v >= BandCriticalFloor:
		return BandCritical
	case v >= BandHighFloor:
		return BandHigh
	case v >= BandMediumFloor:
		return BandMedium
	default:
		return BandLow
	}
}

// Round1 rounds to one decimal, the display precision of the canonical scale.
// Rounding happens once, here, at the edge of the model — rounding at several
// call sites is how two views of "the same" number come to differ by 0.1.
func Round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// normalise maps a raw measurement onto 0–100 given its own bounds, clamping.
// A degenerate range (max <= min) yields the floor rather than +Inf.
func normalise(value, min, max float64) float64 {
	if max <= min || math.IsNaN(value) {
		return MinScore
	}
	return Clamp((value - min) / (max - min) * MaxScore)
}
