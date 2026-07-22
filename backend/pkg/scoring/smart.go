// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scoring

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Smart Risk Calculation (spec §8 "Calcul de risque intelligent").
//
// The classic Score Engine (see engine.go) is the intrinsic Probability × Impact ×
// AssetCriticality model and stays untouched — it is the invariant the rest of the
// platform relies on. This file adds a SEPARATE, richer model that blends eight
// factors into a single 0–100 "smart score", with CONFIGURABLE per-factor weights.
//
// The engine is pure and deterministic: no I/O, no state, stdlib only — just like
// pkg/vulnprio. All inputs are optional and degrade gracefully (a missing signal
// contributes nothing / stays neutral), so a freshly created risk with no linked
// asset, no scans and no compliance data still produces a sensible score.

// FactorKey identifies one of the eight factors of the smart-risk model. The
// string values are stable API contract keys — the frontend localises by them.
type FactorKey string

const (
	FactorBusinessCriticality FactorKey = "business_criticality" // 1. importance of the asset to the business
	FactorInternetExposure    FactorKey = "internet_exposure"    // 2. publicly reachable vs. isolated
	FactorVulnerabilities     FactorKey = "vulnerabilities"       // 3. number + severity (CVSS) of findings
	FactorControlMaturity     FactorKey = "control_maturity"      // 4. effectiveness of controls already in place
	FactorIncidentHistory     FactorKey = "incident_history"      // 5. frequency of past compromises on the asset
	FactorExploitability      FactorKey = "exploitability"        // 6. public exploit code / attack complexity
	FactorFinancialValue      FactorKey = "financial_value"       // 7. estimated loss on C/I/A compromise
	FactorThreatIntel         FactorKey = "threat_intel"          // 8. correlation with live threat intelligence (CTI)
)

// FactorKeys is the canonical ordering of the eight factors — used for stable
// iteration, breakdown ordering and default-weight construction.
var FactorKeys = []FactorKey{
	FactorBusinessCriticality,
	FactorInternetExposure,
	FactorVulnerabilities,
	FactorControlMaturity,
	FactorIncidentHistory,
	FactorExploitability,
	FactorFinancialValue,
	FactorThreatIntel,
}

// factorLabels are the human-readable (English) default labels. The frontend may
// override via i18n keyed on FactorKey, but the API always ships a label so a raw
// consumer (curl, a report) is self-describing.
var factorLabels = map[FactorKey]string{
	FactorBusinessCriticality: "Business criticality",
	FactorInternetExposure:    "Internet exposure",
	FactorVulnerabilities:     "Vulnerabilities",
	FactorControlMaturity:     "Control maturity",
	FactorIncidentHistory:     "Incident history",
	FactorExploitability:      "Exploitability",
	FactorFinancialValue:      "Financial value",
	FactorThreatIntel:         "Active threats (CTI)",
}

// Label returns the default English label for a factor.
func (k FactorKey) Label() string {
	if l, ok := factorLabels[k]; ok {
		return l
	}
	return string(k)
}

// FactorWeights maps each factor to its relative importance. Values are relative
// (not required to sum to 1): the engine normalises them internally, so an admin
// can bump one factor without re-tuning the others. A missing key falls back to
// the default weight; a negative weight is treated as 0.
type FactorWeights map[FactorKey]float64

// Default per-factor weights (sum = 1.00). Tuned to be defensible, not magic:
// vulnerabilities + exploitability + threat-intel (the "is it being attacked"
// axis) dominate, business criticality and financial value carry the "what does
// it cost us" axis, exposure / maturity / history refine.
var defaultWeights = FactorWeights{
	FactorBusinessCriticality: 0.15,
	FactorInternetExposure:    0.10,
	FactorVulnerabilities:     0.20,
	FactorControlMaturity:     0.10,
	FactorIncidentHistory:     0.10,
	FactorExploitability:      0.15,
	FactorFinancialValue:      0.10,
	FactorThreatIntel:         0.10,
}

// DefaultFactorWeights returns a fresh copy of the default weights so callers can
// mutate the result without corrupting the package-level defaults.
func DefaultFactorWeights() FactorWeights {
	w := make(FactorWeights, len(defaultWeights))
	for k, v := range defaultWeights {
		w[k] = v
	}
	return w
}

// DefaultALEReferenceXAF is the annual-loss-expectancy (XAF) at which the
// financial-value factor saturates to 1.0. It is an order-of-magnitude anchor
// (≈ 50M FCFA); callers may override per tenant via SmartInput.ALEReferenceXAF.
const DefaultALEReferenceXAF = 50_000_000.0

// SmartInput is everything the engine needs. Every field is optional; the zero
// value degrades gracefully. Callers (the use case) assemble it from the risk,
// its asset, the vulnerability register, compliance posture, incidents, CRQ and
// the CTI feed — but the engine itself never reaches for any of that.
type SmartInput struct {
	// 1. Business criticality — the asset's Score-Engine factor (0.1–3.0, 3.0 =
	//    CRITICAL). 0 → treated as unknown/MEDIUM.
	BusinessCriticalityFactor float64

	// 2. Internet exposure — 0 (isolated / internal only) … 1 (public-facing).
	//    Use ExposureFromLabel to map a coarse label.
	InternetExposure float64

	// 3. Vulnerabilities present — count of open findings on the asset and the
	//    worst CVSS among them (0–10).
	VulnerabilityCount int
	MaxCVSS            float64

	// 4. Control maturity — implemented-control coverage 0 (none) … 1 (fully
	//    mature). Mature controls REDUCE risk, so the engine inverts this. When no
	//    compliance data is available set ControlsAssessed=false → neutral factor.
	ControlMaturity  float64
	ControlsAssessed bool

	// 5. Incident history — number of past incidents recorded against the asset.
	IncidentCount int

	// 6. Exploitability — worst-case exploit signals among the asset's CVEs.
	EPSS             float64 // FIRST EPSS probability 0–1
	KEV              bool    // CISA Known-Exploited
	ExploitAvailable bool    // public exploit / weaponised PoC
	ExploitMaturity  string  // none|poc|functional|high

	// 7. Financial value — annual loss expectancy (XAF) and the reference at which
	//    the factor saturates. ALEReferenceXAF <= 0 → DefaultALEReferenceXAF.
	ALEXAF          float64
	ALEReferenceXAF float64

	// 8. Active threats (CTI) — live threat-intelligence correlation 0 … 1
	//    (e.g. asset CVE present in CISA KEV / an active campaign).
	ActiveThreatSignal float64
}

// FactorScore is one row of the breakdown: the normalised risk contribution of a
// factor, the (normalised) weight applied to it, and the resulting points.
type FactorScore struct {
	Key          FactorKey `json:"key"`
	Label        string    `json:"label"`
	Weight       float64   `json:"weight"`       // normalised weight actually applied, [0,1]
	Value        float64   `json:"value"`        // factor risk contribution, [0,1] (1 = worst)
	Contribution float64   `json:"contribution"` // Value × Weight × 100 — points this factor adds
	Detail       string    `json:"detail"`       // short human explanation of the raw inputs
}

// SmartResult is the computed smart score plus its full, radar-ready breakdown.
type SmartResult struct {
	Score       float64          `json:"score"`       // 0–100
	Criticality CriticalityLevel `json:"criticality"` // low|medium|high|critical
	Factors     []FactorScore    `json:"factors"`     // one per factor, in FactorKeys order
	Explanation string           `json:"explanation"` // top drivers, human readable
}

// clampUnit bounds v to [0,1].
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ExposureFromLabel maps a coarse exposure label to a 0–1 signal. Unknown → 0.5
// (assume some exposure rather than none). Helps callers that only have a tag.
func ExposureFromLabel(label string) float64 {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "public", "internet-facing", "internet_facing", "external", "public-facing":
		return 1.0
	case "dmz", "limited", "partially-exposed", "restricted":
		return 0.6
	case "internal", "private", "isolated", "air-gapped", "airgapped":
		return 0.1
	default:
		return 0.5
	}
}

// logSaturate maps a non-negative count to 0–1 on a log10 scale that reaches 1.0
// at `full` (e.g. full=10 → count 0=0, 1≈0.29, 3≈0.58, 10=1.0). Saturates above.
func logSaturate(count int, full int) float64 {
	if count <= 0 {
		return 0
	}
	if full < 2 {
		full = 2
	}
	return clampUnit(math.Log10(float64(count)+1) / math.Log10(float64(full)+1))
}

// exploitSignal combines exploitability inputs into 0–1. Mirrors pkg/vulnprio's
// logic: KEV dominates, then a weaponised exploit, then EPSS, then a maturity hint.
func exploitSignal(in SmartInput) float64 {
	w := clampUnit(in.EPSS)
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
	return clampUnit(w)
}

// factorValue returns the 0–1 risk contribution of a single factor plus a short
// human detail string. 1 = maximal risk from that factor.
func factorValue(key FactorKey, in SmartInput) (float64, string) {
	switch key {
	case FactorBusinessCriticality:
		f := in.BusinessCriticalityFactor
		if f <= 0 {
			f = 1.5 // unknown → MEDIUM
		}
		return clampUnit(f / 3.0), fmt.Sprintf("asset criticality factor %.1f/3.0", f)

	case FactorInternetExposure:
		v := clampUnit(in.InternetExposure)
		return v, fmt.Sprintf("%.0f%% exposed", v*100)

	case FactorVulnerabilities:
		if in.VulnerabilityCount <= 0 {
			return 0, "no open vulnerabilities"
		}
		sev := clampUnit(in.MaxCVSS / 10.0)
		vol := logSaturate(in.VulnerabilityCount, 10)
		v := clampUnit(0.7*sev + 0.3*vol)
		return v, fmt.Sprintf("%d open, worst CVSS %.1f", in.VulnerabilityCount, in.MaxCVSS)

	case FactorControlMaturity:
		if !in.ControlsAssessed {
			return 0.5, "no compliance data (neutral)"
		}
		// Mature controls reduce risk: risk contribution = 1 − maturity.
		v := clampUnit(1.0 - clampUnit(in.ControlMaturity))
		return v, fmt.Sprintf("%.0f%% controls implemented", clampUnit(in.ControlMaturity)*100)

	case FactorIncidentHistory:
		if in.IncidentCount <= 0 {
			return 0, "no past incidents"
		}
		v := logSaturate(in.IncidentCount, 8)
		return v, fmt.Sprintf("%d past incident(s)", in.IncidentCount)

	case FactorExploitability:
		v := exploitSignal(in)
		switch {
		case in.KEV:
			return v, "actively exploited (CISA-KEV)"
		case in.ExploitAvailable:
			return v, "public exploit available"
		case in.EPSS > 0:
			return v, fmt.Sprintf("EPSS %.0f%%", in.EPSS*100)
		default:
			return v, "no known exploit"
		}
	case FactorFinancialValue:
		ref := in.ALEReferenceXAF
		if ref <= 0 {
			ref = DefaultALEReferenceXAF
		}
		if in.ALEXAF <= 0 {
			return 0, "no financial exposure modelled"
		}
		v := clampUnit(in.ALEXAF / ref)
		return v, fmt.Sprintf("ALE %.0f XAF", in.ALEXAF)

	case FactorThreatIntel:
		v := clampUnit(in.ActiveThreatSignal)
		if v >= 0.75 {
			return v, "active threat correlated"
		}
		if v > 0 {
			return v, "some threat activity"
		}
		return 0, "no active threat correlated"

	default:
		return 0, ""
	}
}

// normaliseWeights returns weights that sum to 1.0 across the eight factors,
// filling missing keys from the defaults and clamping negatives to 0. If every
// resulting weight is 0, falls back to equal weighting so a score is still produced.
func normaliseWeights(w FactorWeights) map[FactorKey]float64 {
	raw := make(map[FactorKey]float64, len(FactorKeys))
	var sum float64
	for _, k := range FactorKeys {
		v, ok := w[k]
		if !ok {
			v = defaultWeights[k]
		}
		if v < 0 {
			v = 0
		}
		raw[k] = v
		sum += v
	}
	out := make(map[FactorKey]float64, len(FactorKeys))
	if sum <= 0 {
		eq := 1.0 / float64(len(FactorKeys))
		for _, k := range FactorKeys {
			out[k] = eq
		}
		return out
	}
	for _, k := range FactorKeys {
		out[k] = raw[k] / sum
	}
	return out
}

// smartCriticality buckets a 0–100 smart score into a criticality band.
//
//	score >= 75 → Critical
//	score >= 50 → High
//	score >= 25 → Medium
//	score <  25 → Low
func smartCriticality(score float64) CriticalityLevel {
	switch {
	case score >= 75:
		return CriticalityCritical
	case score >= 50:
		return CriticalityHigh
	case score >= 25:
		return CriticalityMedium
	default:
		return CriticalityLow
	}
}

// ComputeSmart blends the eight factors into a 0–100 smart score using the
// supplied weights (nil → defaults). Deterministic and side-effect free.
//
// SmartScore = 100 × Σ (normalisedWeight_i × factorValue_i)
func ComputeSmart(in SmartInput, weights FactorWeights) SmartResult {
	if weights == nil {
		weights = defaultWeights
	}
	norm := normaliseWeights(weights)

	factors := make([]FactorScore, 0, len(FactorKeys))
	var total float64
	for _, k := range FactorKeys {
		val, detail := factorValue(k, in)
		wt := norm[k]
		contribution := val * wt * 100.0
		total += contribution
		factors = append(factors, FactorScore{
			Key:          k,
			Label:        k.Label(),
			Weight:       math.Round(wt*10000) / 10000,
			Value:        math.Round(val*10000) / 10000,
			Contribution: math.Round(contribution*100) / 100,
			Detail:       detail,
		})
	}

	score := math.Round(total*100) / 100
	if score > 100 {
		score = 100
	}

	return SmartResult{
		Score:       score,
		Criticality: smartCriticality(score),
		Factors:     factors,
		Explanation: explainSmart(score, factors),
	}
}

// explainSmart builds a short, honest sentence naming the top contributing
// factors so a reader understands what drove the score.
func explainSmart(score float64, factors []FactorScore) string {
	ranked := make([]FactorScore, len(factors))
	copy(ranked, factors)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].Contribution > ranked[j].Contribution
	})

	var drivers []string
	for _, f := range ranked {
		if f.Contribution <= 0 {
			continue
		}
		drivers = append(drivers, fmt.Sprintf("%s (%.0f%% weight, %s)", f.Label, f.Weight*100, f.Detail))
		if len(drivers) == 3 {
			break
		}
	}

	lvl := string(smartCriticality(score))
	lvl = strings.ToUpper(lvl[:1]) + lvl[1:]
	if len(drivers) == 0 {
		return fmt.Sprintf("Smart score %.1f/100 → %s. No aggravating factors present.", score, lvl)
	}
	return fmt.Sprintf("Smart score %.1f/100 → %s. Top drivers: %s.", score, lvl, strings.Join(drivers, "; "))
}
