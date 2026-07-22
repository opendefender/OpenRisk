// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package dashboard

import "math"

// CyberScore is the executive posture grade: a single 0–100 number plus an A–F
// letter, backed by the weighted components that produced it. Higher is safer.
type CyberScore struct {
	Score      int              `json:"score"` // 0..100 (higher = safer)
	Grade      string           `json:"grade"` // A..F
	Label      string           `json:"label"` // human posture label
	Components []ScoreComponent `json:"components"`
}

// ScoreComponent is one axis of the cyber score with the (already normalised)
// weight it carried. Weights sum to 1 across the axes that had data.
type ScoreComponent struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Value  int     `json:"value"`  // 0..100 for this axis (higher = safer)
	Weight float64 `json:"weight"` // effective weight after renormalisation
}

// scoreAxis is an internal accumulator: an axis contributes only when Present.
// Absent axes are dropped and the remaining weights are renormalised to 1, so a
// tenant with no compliance data (say) is not penalised for the missing signal —
// exactly the graceful-degradation contract the rest of the dashboard follows.
type scoreAxis struct {
	key     string
	label   string
	weight  float64
	value   float64 // 0..100
	present bool
}

// Default relative weights for the four posture axes. Documented, deterministic,
// heuristic — the intent is a "board-grade" single number, not an actuarial one.
const (
	weightCompliance = 0.35
	weightRisk       = 0.30
	weightVuln       = 0.20
	weightIncident   = 0.15
)

// computeCyberScore folds the four posture axes into a single grade. Each axis is
// a 0..100 "safety" value; pass present=false to omit an axis whose source was
// unavailable. If every axis is absent the score is a neutral 50/E.
func computeCyberScore(axes []scoreAxis) CyberScore {
	var totalWeight float64
	for _, a := range axes {
		if a.present {
			totalWeight += a.weight
		}
	}

	out := CyberScore{Components: make([]ScoreComponent, 0, len(axes))}
	if totalWeight <= 0 {
		out.Score = 50
		out.Grade, out.Label = gradeFor(50)
		return out
	}

	var acc float64
	for _, a := range axes {
		if !a.present {
			continue
		}
		w := a.weight / totalWeight
		v := clamp100(a.value)
		acc += w * v
		out.Components = append(out.Components, ScoreComponent{
			Key:    a.key,
			Label:  a.label,
			Value:  int(math.Round(v)),
			Weight: round4(w),
		})
	}

	out.Score = int(math.Round(clamp100(acc)))
	out.Grade, out.Label = gradeFor(out.Score)
	return out
}

// gradeFor maps a 0..100 score onto an A–F letter and a posture label.
func gradeFor(score int) (grade, label string) {
	switch {
	case score >= 90:
		return "A", "Excellente"
	case score >= 80:
		return "B", "Solide"
	case score >= 70:
		return "C", "Correcte"
	case score >= 60:
		return "D", "Fragile"
	case score >= 50:
		return "E", "À risque"
	default:
		return "F", "Critique"
	}
}

// --- axis builders (each returns a 0..100 safety value) ---------------------

// complianceAxis: overall control coverage is already a safety percentage.
func complianceAxisValue(implemented, total int) (float64, bool) {
	if total <= 0 {
		return 0, false
	}
	return 100 * float64(implemented) / float64(total), true
}

// riskAxis: penalise the register for its critical/high concentration. A tenant
// whose register is all-critical scores 0; an all-low register scores ~100.
func riskAxisValue(critical, high, total int) (float64, bool) {
	if total <= 0 {
		return 0, false
	}
	critRatio := float64(critical) / float64(total)
	highRatio := float64(high) / float64(total)
	penalty := 100 * (2*critRatio + highRatio) // criticals hurt twice as much
	return 100 - math.Min(100, penalty), true
}

// vulnAxis: start from a clean 100 and dock points for KEV (known-exploited) and
// raw critical-severity volume — the two signals a board cares about.
func vulnAxisValue(kev, critical int64, haveData bool) (float64, bool) {
	if !haveData {
		return 0, false
	}
	penalty := float64(kev)*12 + float64(critical)*4
	return 100 - math.Min(100, penalty), true
}

// incidentAxis: resolution rate, docked for any still-open critical incidents.
func incidentAxisValue(resolutionRate float64, criticalOpen, total int) (float64, bool) {
	if total <= 0 {
		return 0, false
	}
	v := resolutionRate - math.Min(40, float64(criticalOpen)*10)
	return clamp100(v), true
}

func clamp100(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
