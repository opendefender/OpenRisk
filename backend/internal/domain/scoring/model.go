// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scoring

import (
	"sort"
	"time"
)

// Scope names what is being scored. The same scale, the same bands and the same
// explanation shape apply to all three — only the factors differ.
type Scope string

const (
	ScopeTenant Scope = "tenant"
	ScopeRisk   Scope = "risk"
	ScopeAsset  Scope = "asset"
)

// ParseScope validates a raw scope string.
func ParseScope(raw string) (Scope, bool) {
	switch Scope(raw) {
	case ScopeTenant:
		return ScopeTenant, true
	case ScopeRisk:
		return ScopeRisk, true
	case ScopeAsset:
		return ScopeAsset, true
	}
	return "", false
}

// FactorKey identifies one contributing factor. These strings are API contract:
// the client localises by them and never invents its own.
type FactorKey string

const (
	// Tenant scope.
	FactorRiskExposure          FactorKey = "risk_exposure"
	FactorControlGaps           FactorKey = "control_gaps"
	FactorVulnerabilityPressure FactorKey = "vulnerability_pressure"
	FactorIncidentPressure      FactorKey = "incident_pressure"

	// Risk scope.
	FactorProbability      FactorKey = "probability"
	FactorImpact           FactorKey = "impact"
	FactorAssetCriticality FactorKey = "asset_criticality"

	// Asset scope.
	FactorLinkedRiskExposure FactorKey = "linked_risk_exposure"
	FactorInternetExposure   FactorKey = "internet_exposure"
	FactorCriticality        FactorKey = "criticality"
)

// I18nKey is the translation key for a factor's label.
func (k FactorKey) I18nKey() string { return "score.factor." + string(k) }

// Factor is one line of the breakdown — the unit of explanation.
//
// The arithmetic is deliberately trivial and checkable by eye:
// Contribution = Weight × Raw, and the contributions sum to Value. Anyone reading
// the response can verify the score by hand, which is the point: a score nobody
// can reproduce is a score nobody should act on.
type Factor struct {
	Key FactorKey `json:"factor"`
	// Weight is normalised: the weights of a breakdown sum to 1.
	Weight float64 `json:"weight"`
	// Raw is this factor's own measurement on the canonical 0–100 scale, where
	// 100 always means "worst". A factor whose natural direction is the opposite
	// (control coverage, for instance) is inverted at the point of measurement,
	// not at the point of display.
	Raw          float64 `json:"raw"`
	Contribution float64 `json:"contribution"`
	LabelI18nKey string  `json:"label_i18n_key"`
	// Available is false when the underlying source could not be consulted. The
	// factor is then excluded from the score and its weight redistributed — an
	// absent signal must not read as a good one. It is still returned, so the
	// explainer can say "not measured" instead of quietly showing three factors
	// where the model has four.
	Available bool `json:"available"`
}

// Result is one computed score, with everything needed to explain it.
type Result struct {
	Scope Scope `json:"scope"`
	// Value is the residual score when mitigations are known, otherwise the
	// inherent one — it is the number every surface displays.
	Value            float64 `json:"value"`
	Band             Band    `json:"band"`
	BandLabelI18nKey string  `json:"band_label_i18n_key"`

	// Inherent is the exposure before any mitigation credit; Residual is what
	// remains once applied mitigations are taken into account. Auditors ask for
	// both, and a product that shows only one is answering half the question.
	Inherent         float64 `json:"inherent"`
	InherentBand     Band    `json:"inherent_band"`
	Residual         float64 `json:"residual"`
	ResidualBand     Band    `json:"residual_band"`
	// MitigationEffectiveness ∈ [0,1] is the reduction applied to get Residual.
	MitigationEffectiveness float64 `json:"mitigation_effectiveness"`

	ComputedAt     time.Time `json:"computed_at"`
	FormulaVersion string    `json:"formula_version"`

	// Inputs are the measurements the calculation actually used — the assumptions
	// the explainer shows. Echoing them back is what lets a user say "that impact
	// figure is wrong" instead of "the score feels wrong".
	Inputs map[string]any `json:"inputs"`

	Breakdown []Factor `json:"breakdown"`
}

// weighted is the internal accumulator shared by all three scopes.
type weighted struct {
	key       FactorKey
	weight    float64
	raw       float64
	available bool
}

// combine turns weighted factors into a Result body.
//
// Unavailable factors are dropped and the remaining weights renormalised, so a
// missing source degrades the confidence of the score rather than silently
// scoring that dimension as zero (which would read as "excellent" and is the
// single most dangerous failure mode a security score can have).
//
// The returned value is the INHERENT score; residual is applied by the caller.
func combine(factors []weighted) (value float64, breakdown []Factor) {
	var totalWeight float64
	for _, f := range factors {
		if f.available && f.weight > 0 {
			totalWeight += f.weight
		}
	}

	breakdown = make([]Factor, 0, len(factors))
	for _, f := range factors {
		out := Factor{
			Key:          f.key,
			Raw:          Round1(Clamp(f.raw)),
			LabelI18nKey: f.key.I18nKey(),
			Available:    f.available,
		}
		if f.available && f.weight > 0 && totalWeight > 0 {
			out.Weight = Round1(f.weight/totalWeight*100) / 100 // 2 decimals
			contribution := (f.weight / totalWeight) * Clamp(f.raw)
			out.Contribution = Round1(contribution)
			value += contribution
		}
		breakdown = append(breakdown, out)
	}

	// Stable order: heaviest contribution first, then by key, so the explainer's
	// bars do not reshuffle between two identical requests.
	sort.SliceStable(breakdown, func(i, j int) bool {
		if breakdown[i].Contribution != breakdown[j].Contribution {
			return breakdown[i].Contribution > breakdown[j].Contribution
		}
		return breakdown[i].Key < breakdown[j].Key
	})

	return Round1(Clamp(value)), breakdown
}

// finish assembles the public Result, applying mitigation effectiveness to get
// the residual score.
//
// Residual = Inherent × (1 − effectiveness). Effectiveness is clamped to [0,1]:
// no mitigation may ever increase exposure, and none may take it below zero.
func finish(scope Scope, inherent float64, effectiveness float64, at time.Time, inputs map[string]any, breakdown []Factor) Result {
	if effectiveness < 0 {
		effectiveness = 0
	}
	if effectiveness > 1 {
		effectiveness = 1
	}

	inherent = Round1(Clamp(inherent))
	residual := Round1(Clamp(inherent * (1 - effectiveness)))

	if inputs == nil {
		inputs = map[string]any{}
	}

	return Result{
		Scope:                   scope,
		Value:                   residual,
		Band:                    BandFor(residual),
		BandLabelI18nKey:        BandFor(residual).I18nKey(),
		Inherent:                inherent,
		InherentBand:            BandFor(inherent),
		Residual:                residual,
		ResidualBand:            BandFor(residual),
		MitigationEffectiveness: Round1(effectiveness*100) / 100,
		ComputedAt:              at.UTC(),
		FormulaVersion:          FormulaVersion,
		Inputs:                  inputs,
		Breakdown:               breakdown,
	}
}
