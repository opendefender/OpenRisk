// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scoring

import (
	"math"
	"time"
)

// ---------------------------------------------------------------------------
// The model.
//
// Every scope uses the SAME shape: a weighted sum of factors each measured on
// 0–100 where 100 means "worst", producing a value on 0–100, banded by BandFor.
//
// Weighted-additive rather than multiplicative, deliberately and with a cost
// stated openly: in a product model a single zero factor zeroes the score, which
// reads well for "probability = 0" but means one unmeasured dimension can wipe
// out a real exposure. Additive keeps every dimension visible, makes the
// breakdown arithmetic checkable by eye (contribution = weight × raw, and the
// contributions sum to the value), and guarantees monotonicity: no increase in
// probability or impact can ever lower the score. That last property is a test,
// not a hope — see TestMonotonic_* in compute_test.go.
//
// The classic Score Engine (pkg/scoring, P × I × AssetCriticality on 0–30) is NOT
// replaced: it remains the invariant behind Risk.Score. This package is what the
// UI displays, on one scale, with one band, and it consumes the same inputs.
// ---------------------------------------------------------------------------

// Weights per scope. Each set sums to 1.0 before renormalisation; the sums are
// asserted by TestWeightsSumToOne so a future edit cannot quietly unbalance them.
var (
	tenantWeights = map[FactorKey]float64{
		FactorRiskExposure:          0.40,
		FactorControlGaps:           0.25,
		FactorVulnerabilityPressure: 0.20,
		FactorIncidentPressure:      0.15,
	}
	riskWeights = map[FactorKey]float64{
		FactorProbability:      0.40,
		FactorImpact:           0.40,
		FactorAssetCriticality: 0.20,
	}
	assetWeights = map[FactorKey]float64{
		FactorCriticality:           0.35,
		FactorLinkedRiskExposure:    0.35,
		FactorVulnerabilityPressure: 0.20,
		FactorInternetExposure:      0.10,
	}
)

// Domain bounds of the raw inputs. These are the ONLY place the product's input
// ranges are written down in code; docs/scoring/SCORE_MODEL.md documents them for
// humans and the two are kept in step by TestDocumentedBounds.
const (
	MinProbability      = 0.0
	MaxProbability      = 1.0
	MinImpact           = 0.0
	MaxImpact           = 10.0
	MinAssetCriticality = 0.1
	MaxAssetCriticality = 3.0
)

// ---------------------------------------------------------------------------
// Risk scope
// ---------------------------------------------------------------------------

// RiskInput is everything the risk score needs. Every field is optional in the
// sense that a zero value is legal; Available flags say whether a source was
// actually consulted.
type RiskInput struct {
	// Probability ∈ [0,1], Impact ∈ [0,10] — the domain's canonical scales.
	Probability float64
	Impact      float64

	// AssetCriticality ∈ [0.1,3.0], the Score Engine's multiplier. HasAsset is
	// false when the risk is not linked to an asset: the factor is then excluded
	// and its weight redistributed, rather than scored as 0.1 (which would read
	// as "this touches nothing important").
	AssetCriticality float64
	HasAsset         bool

	// MitigationEffectiveness ∈ [0,1] drives the residual score.
	MitigationEffectiveness float64
}

// ComputeRisk scores one risk.
func ComputeRisk(in RiskInput, at time.Time) Result {
	factors := []weighted{
		{
			key:       FactorProbability,
			weight:    riskWeights[FactorProbability],
			raw:       normalise(in.Probability, MinProbability, MaxProbability),
			available: true,
		},
		{
			key:       FactorImpact,
			weight:    riskWeights[FactorImpact],
			raw:       normalise(in.Impact, MinImpact, MaxImpact),
			available: true,
		},
		{
			key:       FactorAssetCriticality,
			weight:    riskWeights[FactorAssetCriticality],
			raw:       normalise(in.AssetCriticality, MinAssetCriticality, MaxAssetCriticality),
			available: in.HasAsset,
		},
	}

	inherent, breakdown := combine(factors)

	inputs := map[string]any{
		"probability":       in.Probability,
		"impact":            in.Impact,
		"has_linked_asset":  in.HasAsset,
		"scale_probability": []float64{MinProbability, MaxProbability},
		"scale_impact":      []float64{MinImpact, MaxImpact},
	}
	if in.HasAsset {
		inputs["asset_criticality"] = in.AssetCriticality
		inputs["scale_asset_criticality"] = []float64{MinAssetCriticality, MaxAssetCriticality}
	}

	return finish(ScopeRisk, inherent, in.MitigationEffectiveness, at, inputs, breakdown)
}

// ---------------------------------------------------------------------------
// Asset scope
// ---------------------------------------------------------------------------

// AssetInput is everything the asset score needs.
type AssetInput struct {
	// Criticality ∈ [0.1,3.0].
	Criticality float64

	// MaxLinkedRiskScore is the highest INHERENT risk score (0–100, this
	// package's scale) among the risks linked to the asset. HasLinkedRisks is
	// false when there are none.
	MaxLinkedRiskScore float64
	HasLinkedRisks     bool

	// OpenVulnerabilities / MaxCVSS describe the asset's current findings.
	OpenVulnerabilities int
	MaxCVSS             float64
	HasVulnData         bool

	// InternetFacing raises exposure; HasExposureData is false when nothing is
	// known about reachability.
	InternetFacing  bool
	HasExposureData bool

	MitigationEffectiveness float64
}

// ComputeAsset scores one asset.
func ComputeAsset(in AssetInput, at time.Time) Result {
	factors := []weighted{
		{
			key:       FactorCriticality,
			weight:    assetWeights[FactorCriticality],
			raw:       normalise(in.Criticality, MinAssetCriticality, MaxAssetCriticality),
			available: true,
		},
		{
			key:       FactorLinkedRiskExposure,
			weight:    assetWeights[FactorLinkedRiskExposure],
			raw:       Clamp(in.MaxLinkedRiskScore),
			available: in.HasLinkedRisks,
		},
		{
			key:       FactorVulnerabilityPressure,
			weight:    assetWeights[FactorVulnerabilityPressure],
			raw:       vulnerabilityPressure(in.OpenVulnerabilities, in.MaxCVSS),
			available: in.HasVulnData,
		},
		{
			key:       FactorInternetExposure,
			weight:    assetWeights[FactorInternetExposure],
			raw:       boolRaw(in.InternetFacing),
			available: in.HasExposureData,
		},
	}

	inherent, breakdown := combine(factors)

	inputs := map[string]any{
		"criticality":          in.Criticality,
		"open_vulnerabilities": in.OpenVulnerabilities,
		"max_cvss":             in.MaxCVSS,
		"internet_facing":      in.InternetFacing,
		"linked_risks":         in.HasLinkedRisks,
	}

	return finish(ScopeAsset, inherent, in.MitigationEffectiveness, at, inputs, breakdown)
}

// ---------------------------------------------------------------------------
// Tenant scope
// ---------------------------------------------------------------------------

// TenantInput is everything the organisation-wide score needs.
type TenantInput struct {
	// CriticalRisks / HighRisks / TotalRisks describe the register.
	CriticalRisks int
	HighRisks     int
	TotalRisks    int
	HasRiskData   bool

	// ImplementedControls / ApplicableControls describe compliance coverage.
	// The factor measures the GAP (100 − coverage), because on this scale 100
	// always means "worst".
	ImplementedControls int
	ApplicableControls  int
	HasComplianceData   bool

	// KEV / critical findings currently open.
	KEVVulnerabilities      int
	CriticalVulnerabilities int
	HasVulnData             bool

	// OpenIncidents / CriticalOpenIncidents describe live pressure.
	OpenIncidents         int
	CriticalOpenIncidents int
	HasIncidentData       bool

	MitigationEffectiveness float64
}

// ComputeTenant scores an organisation's overall exposure.
func ComputeTenant(in TenantInput, at time.Time) Result {
	factors := []weighted{
		{
			key:       FactorRiskExposure,
			weight:    tenantWeights[FactorRiskExposure],
			raw:       riskExposure(in.CriticalRisks, in.HighRisks, in.TotalRisks),
			available: in.HasRiskData,
		},
		{
			key:       FactorControlGaps,
			weight:    tenantWeights[FactorControlGaps],
			raw:       controlGap(in.ImplementedControls, in.ApplicableControls),
			available: in.HasComplianceData && in.ApplicableControls > 0,
		},
		{
			key:       FactorVulnerabilityPressure,
			weight:    tenantWeights[FactorVulnerabilityPressure],
			raw:       tenantVulnPressure(in.KEVVulnerabilities, in.CriticalVulnerabilities),
			available: in.HasVulnData,
		},
		{
			key:       FactorIncidentPressure,
			weight:    tenantWeights[FactorIncidentPressure],
			raw:       incidentPressure(in.OpenIncidents, in.CriticalOpenIncidents),
			available: in.HasIncidentData,
		},
	}

	inherent, breakdown := combine(factors)

	inputs := map[string]any{
		"critical_risks":           in.CriticalRisks,
		"high_risks":               in.HighRisks,
		"total_risks":              in.TotalRisks,
		"implemented_controls":     in.ImplementedControls,
		"applicable_controls":      in.ApplicableControls,
		"kev_vulnerabilities":      in.KEVVulnerabilities,
		"critical_vulnerabilities": in.CriticalVulnerabilities,
		"open_incidents":           in.OpenIncidents,
		"critical_open_incidents":  in.CriticalOpenIncidents,
	}

	return finish(ScopeTenant, inherent, in.MitigationEffectiveness, at, inputs, breakdown)
}

// ---------------------------------------------------------------------------
// Factor measurements. Each returns 0–100 where 100 is worst, and each is
// monotonic non-decreasing in every one of its "more is worse" arguments.
// ---------------------------------------------------------------------------

// riskExposure weights the register by severity: a critical risk counts double a
// high one, and the result is the share of the register that is severe, floored
// by an absolute term so that "3 critical risks out of 3" and "3 out of 300" are
// not scored identically — a small register full of critical risks is not safe.
func riskExposure(critical, high, total int) float64 {
	if total <= 0 {
		return MinScore
	}
	if critical < 0 {
		critical = 0
	}
	if high < 0 {
		high = 0
	}

	severity := float64(2*critical+high) / float64(2*total)
	share := Clamp(severity * MaxScore)

	// Absolute pressure: saturates at 10 critical-equivalents.
	absolute := Clamp(float64(2*critical+high) / 10 * MaxScore)

	return math.Max(share, absolute)
}

// controlGap is the inverse of coverage: 100 means nothing is implemented.
func controlGap(implemented, applicable int) float64 {
	if applicable <= 0 {
		return MinScore
	}
	if implemented < 0 {
		implemented = 0
	}
	if implemented > applicable {
		implemented = applicable
	}
	coverage := float64(implemented) / float64(applicable)
	return Clamp((1 - coverage) * MaxScore)
}

// tenantVulnPressure: a known-exploited vulnerability weighs far more than a
// merely critical one, because one is being used against people today.
func tenantVulnPressure(kev, critical int) float64 {
	if kev < 0 {
		kev = 0
	}
	if critical < 0 {
		critical = 0
	}
	return Clamp(float64(kev)*20 + float64(critical)*5)
}

// vulnerabilityPressure (per asset) blends worst severity with volume: severity
// dominates, volume is logarithmic because the tenth finding on a host matters
// far less than the first.
func vulnerabilityPressure(open int, maxCVSS float64) float64 {
	if open <= 0 {
		return MinScore
	}
	severity := normalise(maxCVSS, 0, 10)
	volume := Clamp(math.Log10(float64(open)+1) / math.Log10(101) * MaxScore)
	return Clamp(0.7*severity + 0.3*volume)
}

// incidentPressure: open incidents, with critical ones weighted heavily.
func incidentPressure(open, criticalOpen int) float64 {
	if open < 0 {
		open = 0
	}
	if criticalOpen < 0 {
		criticalOpen = 0
	}
	return Clamp(float64(open)*8 + float64(criticalOpen)*20)
}

func boolRaw(b bool) float64 {
	if b {
		return MaxScore
	}
	return MinScore
}
