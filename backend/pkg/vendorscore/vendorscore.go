// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package vendorscore computes the vendor score of a submitted questionnaire
// (ADR 0004 D5, #671).
//
// It is pure and deterministic: no database, no clock, no HTTP, and no import of
// anything under internal/. The same answers always give the same score, and the
// result carries its own arithmetic so a reviewer can check it.
//
// THE BOUNDARY (D-044). The vendor score is SEPARATE. It is never an input to
// Risk.Score, to SmartScore, or to asset criticality: nothing in the risk
// scoring path imports this package, and this package imports nothing from it.
// imports_test.go enforces both directions.
package vendorscore

import (
	"math"

	"github.com/google/uuid"
)

// Version names this formula. A future formula gets a new version, and scores
// stored under this one are never silently recomputed.
const Version = "vendorscore/1"

// Tier is the band a score falls in.
type Tier string

const (
	TierCritical Tier = "critical"
	TierHigh     Tier = "high"
	TierMedium   Tier = "medium"
	TierLow      Tier = "low"
)

// The thresholds are the Score Engine's own band proportions — 7.0 / 4.0 / 2.0
// on its 0–10 scale — multiplied by ten, so the two scales read alike (ADR 0004
// D5). Higher is riskier, like every other score in the product.
const (
	CriticalFrom = 70.0
	HighFrom     = 40.0
	MediumFrom   = 20.0
)

// Item is one question as the formula sees it.
type Item struct {
	ID uuid.UUID
	// Scorable is false for a free-text question: it never scores, whatever
	// weight it carries.
	Scorable bool
	// Weight is the question's weight, 0–10. A weight of 0 does not score.
	Weight float64
	// Points is the chosen option's points, in [0, 1] where 1 is the fully
	// favourable answer. Nil when the question was not answered.
	Points *float64
	// NA marks "not applicable": the question leaves both sums.
	NA bool
}

// Contribution is one question's share of the score. The contributions of a
// result sum to its score, before the score's rounding to two decimals.
type Contribution struct {
	ItemID       uuid.UUID
	Weight       float64
	Points       float64
	NA           bool
	Contribution float64
}

// Result is a computed vendor score.
type Result struct {
	// Score is nil when nothing is scorable. It is never 0 in that case: 0 would
	// read as "no risk", which a questionnaire with no scorable answer has not
	// shown.
	Score *float64
	// Tier is nil exactly when Score is.
	Tier *Tier
	// Breakdown lists every weighted choice question, N/A ones included with a
	// zero contribution, in input order. Always an array, never nil.
	Breakdown []Contribution
	Version   string
}

// Compute is ADR 0004 D5:
//
//	for each scorable item with weight w > 0 and not N/A:
//	    p = the chosen option's points   (an unanswered item scores p = 0)
//	score = 100 × Σ w·(1 − p) / Σ w
//
// Unanswered counts as the least favourable answer: silence is not assurance.
func Compute(items []Item) Result {
	res := Result{Version: Version, Breakdown: []Contribution{}}

	var totalWeight float64
	for _, it := range items {
		if counts(it) {
			totalWeight += it.Weight
		}
	}

	var sum float64
	for _, it := range items {
		if !it.Scorable || !(it.Weight > 0) {
			continue
		}
		c := Contribution{ItemID: it.ID, Weight: it.Weight, NA: it.NA}
		if !it.NA {
			p := 0.0
			if it.Points != nil {
				p = clamp01(*it.Points)
			}
			c.Points = p
			if totalWeight > 0 {
				c.Contribution = 100 * it.Weight * (1 - p) / totalWeight
			}
			sum += c.Contribution
		}
		res.Breakdown = append(res.Breakdown, c)
	}

	if totalWeight <= 0 {
		return res
	}
	score := math.Round(sum*100) / 100
	tier := TierFor(score)
	res.Score = &score
	res.Tier = &tier
	return res
}

// TierFor bands a score.
func TierFor(score float64) Tier {
	switch {
	case score >= CriticalFrom:
		return TierCritical
	case score >= HighFrom:
		return TierHigh
	case score >= MediumFrom:
		return TierMedium
	default:
		return TierLow
	}
}

func counts(it Item) bool {
	return it.Scorable && it.Weight > 0 && !it.NA
}

// clamp01 keeps points in [0, 1]. Options are validated on the way in; this is
// the formula refusing to produce a score outside 0–100 whatever it is handed.
func clamp01(v float64) float64 {
	switch {
	case math.IsNaN(v) || v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
