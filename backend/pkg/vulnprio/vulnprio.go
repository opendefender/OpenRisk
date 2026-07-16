// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package vulnprio is a pure, deterministic vulnerability-prioritisation engine.
//
// It scores a vulnerability on four axes the user asked for — CVSS, exploitability,
// business criticality (of the affected asset), and blast radius (number of
// affected assets) — into a single 0–100 priority with a P1..P4 tier and a
// human-readable explanation. No I/O, no dependencies: trivially testable.
package vulnprio

import (
	"fmt"
	"math"
	"strings"
)

// Input is everything the engine needs. All fields are optional; zero values
// degrade gracefully (an unknown CVSS just contributes nothing).
type Input struct {
	CVSS             float64 // base score 0–10
	EPSS             float64 // FIRST EPSS exploit probability 0–1
	KEV              bool    // CISA Known-Exploited Vulnerability
	ExploitAvailable bool    // a public exploit / weaponised PoC exists
	ExploitMaturity  string  // none|poc|functional|high (optional, refines exploit weight)

	// AssetCriticalityFactor is the Score-Engine factor of the affected asset
	// (0.1–3.0; 3.0 = CRITICAL). 0 → treated as unknown/medium.
	AssetCriticalityFactor float64
	AffectedAssets         int // distinct assets carrying this vuln (blast radius)
}

// Result is the computed priority.
type Result struct {
	Score       float64 // 0–100
	Tier        string  // P1|P2|P3|P4
	Explanation string
}

// Axis weights (sum = 1.0). CVSS and exploitability dominate; business
// criticality and blast radius refine. Tuned to be defensible, not magic.
const (
	wSeverity = 0.40
	wExploit  = 0.30
	wBusiness = 0.20
	wExposure = 0.10

	// KEV is a hard signal: CISA-known-exploited means "patch now" regardless of
	// the other axes, so we floor the score into P1 (>= 80).
	kevFloor = 80.0
)

// clamp01 bounds v to [0,1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// exploitWeight combines the exploitability signals into 0–1. KEV dominates
// (known-exploited in the wild), then a weaponised exploit, then EPSS, then a
// bare maturity hint.
func exploitWeight(in Input) float64 {
	w := clamp01(in.EPSS)
	if in.KEV {
		w = math.Max(w, 0.95)
	}
	if in.ExploitAvailable {
		w = math.Max(w, 0.70)
	}
	switch strings.ToLower(in.ExploitMaturity) {
	case "high":
		w = math.Max(w, 0.90)
	case "functional":
		w = math.Max(w, 0.75)
	case "poc":
		w = math.Max(w, 0.50)
	}
	return clamp01(w)
}

// businessWeight maps the asset criticality factor (0.1–3.0) to 0–1. Unknown
// (0) is treated as MEDIUM (factor 1.5).
func businessWeight(factor float64) float64 {
	if factor <= 0 {
		factor = 1.5
	}
	return clamp01(factor / 3.0)
}

// exposureWeight turns the number of affected assets into 0–1 on a log scale:
// 1 asset → 0.30, 3 → 0.60, 9 → 1.0, capped. A single affected asset still
// contributes; a fleet-wide vuln saturates.
func exposureWeight(affected int) float64 {
	if affected <= 1 {
		return 0.30
	}
	return clamp01(math.Log10(float64(affected)) + 0.30)
}

// tierFor buckets a 0–100 score into P1..P4.
func tierFor(score float64) string {
	switch {
	case score >= 80:
		return "P1"
	case score >= 60:
		return "P2"
	case score >= 40:
		return "P3"
	default:
		return "P4"
	}
}

// Compute scores a vulnerability. Deterministic and side-effect free.
func Compute(in Input) Result {
	sev := clamp01(in.CVSS / 10.0)
	exp := exploitWeight(in)
	biz := businessWeight(in.AssetCriticalityFactor)
	exo := exposureWeight(in.AffectedAssets)

	score := 100.0 * (wSeverity*sev + wExploit*exp + wBusiness*biz + wExposure*exo)

	floored := false
	if in.KEV && score < kevFloor {
		score = kevFloor
		floored = true
	}
	score = math.Round(score*100) / 100
	if score > 100 {
		score = 100
	}

	// Build a short, honest explanation of the main drivers.
	var parts []string
	parts = append(parts, fmt.Sprintf("CVSS %.1f", in.CVSS))
	if in.KEV {
		parts = append(parts, "CISA-KEV (exploited in the wild)")
	} else if in.ExploitAvailable {
		parts = append(parts, "public exploit available")
	} else if in.EPSS > 0 {
		parts = append(parts, fmt.Sprintf("EPSS %.0f%%", in.EPSS*100))
	}
	if in.AssetCriticalityFactor >= 2.5 {
		parts = append(parts, "critical asset")
	} else if in.AssetCriticalityFactor >= 2.0 {
		parts = append(parts, "high-value asset")
	}
	if in.AffectedAssets > 1 {
		parts = append(parts, fmt.Sprintf("%d affected assets", in.AffectedAssets))
	}
	explanation := strings.Join(parts, " · ")
	if floored {
		explanation += " — floored to P1 by CISA-KEV"
	}

	return Result{Score: score, Tier: tierFor(score), Explanation: explanation}
}
