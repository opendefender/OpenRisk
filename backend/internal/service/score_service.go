// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package service

import (
	"math"

	"github.com/opendefender/openrisk/internal/domain"
)

// criticalityFactor maps AssetCriticality to a multiplicative factor
var criticalityFactor = map[domain.AssetCriticality]float64{
	domain.CriticalityLow:      0.8,
	domain.CriticalityMedium:   1.0,
	domain.CriticalityHigh:     1.25,
	domain.CriticalityCritical: 1.5,
}

// ComputeRiskScore computes a final score using impact, probability and asset criticality.
// Formula: base = impact * probability; final = base * avg(asset_factors)
// If there are no assets, avg factor defaults to 1.0
func ComputeRiskScore(impact, probability float64, assets []*domain.Asset) float64 {
	base := impact * probability
	if len(assets) == 0 {
		return math.Round(base*100) / 100 // 2 decimals
	}

	var sum float64
	for _, a := range assets {
		if f, ok := criticalityFactor[a.Criticality]; ok {
			sum += f
		} else {
			sum += 1.0
		}
	}
	avg := sum / float64(len(assets))
	final := base * avg
	// round to 2 decimals
	return math.Round(final*100) / 100
}
