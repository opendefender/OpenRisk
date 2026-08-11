// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scoring

// Self-description of the model, served by GET /score/model.
//
// It exists so the explainer can state the model's assumptions — the scale, the
// band boundaries, the weights — WITHOUT the client carrying its own copy of
// them. A second copy of the thresholds on the client is precisely how this
// codebase ended up with four incompatible band mappings; the fix is not "keep
// them in sync", it is "have one".

// BandRange describes one band's span on the canonical scale.
type BandRange struct {
	Band     Band    `json:"band"`
	I18nKey  string  `json:"label_i18n_key"`
	MinValue float64 `json:"min"`
	// MaxValue is inclusive for the top band and exclusive below it — the field
	// says which, so a consumer never has to guess at a boundary.
	MaxValue     float64 `json:"max"`
	MaxInclusive bool    `json:"max_inclusive"`
}

// ScopeModel describes one scope's factors and their weights.
type ScopeModel struct {
	Scope   Scope              `json:"scope"`
	Factors []ScopeModelFactor `json:"factors"`
}

// ScopeModelFactor is one factor's declared weight.
type ScopeModelFactor struct {
	Key          FactorKey `json:"factor"`
	Weight       float64   `json:"weight"`
	LabelI18nKey string    `json:"label_i18n_key"`
}

// ModelDescription is the payload of GET /score/model.
type ModelDescription struct {
	FormulaVersion string       `json:"formula_version"`
	MinValue       float64      `json:"min_value"`
	MaxValue       float64      `json:"max_value"`
	Bands          []BandRange  `json:"bands"`
	Scopes         []ScopeModel `json:"scopes"`
	// InputBounds documents the accepted range of every raw input, so a form can
	// validate against the model rather than against a hard-coded guess.
	InputBounds map[string][]float64 `json:"input_bounds"`
}

// Describe returns the model's self-description.
func Describe() ModelDescription {
	return ModelDescription{
		FormulaVersion: FormulaVersion,
		MinValue:       MinScore,
		MaxValue:       MaxScore,
		Bands: []BandRange{
			{Band: BandLow, I18nKey: BandLow.I18nKey(), MinValue: MinScore, MaxValue: BandMediumFloor},
			{Band: BandMedium, I18nKey: BandMedium.I18nKey(), MinValue: BandMediumFloor, MaxValue: BandHighFloor},
			{Band: BandHigh, I18nKey: BandHigh.I18nKey(), MinValue: BandHighFloor, MaxValue: BandCriticalFloor},
			{Band: BandCritical, I18nKey: BandCritical.I18nKey(), MinValue: BandCriticalFloor, MaxValue: MaxScore, MaxInclusive: true},
		},
		Scopes: []ScopeModel{
			describeScope(ScopeTenant, tenantWeights, []FactorKey{
				FactorRiskExposure, FactorControlGaps, FactorVulnerabilityPressure, FactorIncidentPressure,
			}),
			describeScope(ScopeRisk, riskWeights, []FactorKey{
				FactorProbability, FactorImpact, FactorAssetCriticality,
			}),
			describeScope(ScopeAsset, assetWeights, []FactorKey{
				FactorCriticality, FactorLinkedRiskExposure, FactorVulnerabilityPressure, FactorInternetExposure,
			}),
		},
		InputBounds: map[string][]float64{
			"probability":       {MinProbability, MaxProbability},
			"impact":            {MinImpact, MaxImpact},
			"asset_criticality": {MinAssetCriticality, MaxAssetCriticality},
			"score":             {MinScore, MaxScore},
		},
	}
}

// describeScope keeps the declared order stable (it is the order the explainer
// lists factors in when contributions tie).
func describeScope(scope Scope, weights map[FactorKey]float64, order []FactorKey) ScopeModel {
	factors := make([]ScopeModelFactor, 0, len(order))
	for _, key := range order {
		factors = append(factors, ScopeModelFactor{
			Key:          key,
			Weight:       weights[key],
			LabelI18nKey: key.I18nKey(),
		})
	}
	return ScopeModel{Scope: scope, Factors: factors}
}
