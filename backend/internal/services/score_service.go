package services

import (
	"math"

	"github.com/opendefender/openrisk/internal/core/domain"
)

// criticalityFactor maps AssetCriticality to a multiplicative factor
var criticalityFactor = map[domain.AssetCriticality]float{
	domain.CriticalityLow:      .,
	domain.CriticalityMedium:   .,
	domain.CriticalityHigh:     .,
	domain.CriticalityCritical: .,
}

// ComputeRiskScore computes a final score using impact, probability and asset criticality.
// Formula: base = impact  probability; final = base  avg(asset_factors)
// If there are no assets, avg factor defaults to .
func ComputeRiskScore(impact, probability int, assets []domain.Asset) float {
	base := float(impact  probability)
	if len(assets) ==  {
		return math.Round(base) /  //  decimals
	}

	var sum float
	for _, a := range assets {
		if f, ok := criticalityFactor[a.Criticality]; ok {
			sum += f
		} else {
			sum += .
		}
	}
	avg := sum / float(len(assets))
	final := base  avg
	// round to  decimals
	return math.Round(final) / 
}
