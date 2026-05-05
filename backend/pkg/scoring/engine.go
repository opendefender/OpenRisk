package scoring

import (
	"fmt"
	"math"
	"strings"
)

// engine implémente l'interface Engine avec la formule officielle d'OpenRisk.
type engine struct{}

// NewEngine retourne une instance du Score Engine.
// Cette fonction est le seul point d'entrée du package.
// Thread-safe: l'engine est stateless, peut être partagé.
func NewEngine() Engine {
	return &engine{}
}

// Calculate implémente Engine.Calculate.
// Formule officielle (invariante): Score = round(P × I × A, 3)
// Plages valides:
//
//	probability ∈ [0.0, 1.0]
//	impact ∈ [0.0, 10.0]
//	assetCriticality ∈ [0.1, 3.0]
//
// Retourne ErrValidation si hors range.
func (e *engine) Calculate(probability, impact, assetCriticality float64) (float64, error) {
	// Validate ranges
	if probability < 0.0 || probability > 1.0 {
		return 0, NewValidationError("probability", probability, 0.0, 1.0)
	}
	if impact < 0.0 || impact > 10.0 {
		return 0, NewValidationError("impact", impact, 0.0, 10.0)
	}
	if assetCriticality < 0.1 || assetCriticality > 3.0 {
		return 0, NewValidationError("asset_criticality", assetCriticality, 0.1, 3.0)
	}

	// Formule: Score = Probability × Impact × AssetCriticality
	raw := probability * impact * assetCriticality

	// Round to 3 decimal places
	score := math.Round(raw*1000) / 1000

	return score, nil
}

// ToCriticality implémente Engine.ToCriticality.
// Seuils exacts (≥ operator, précision 3 décimales):
//
//	score >= 7.000 → Critical
//	score >= 4.000 → High
//	score >= 2.000 → Medium
//	score < 2.000 → Low
func (e *engine) ToCriticality(score float64) CriticalityLevel {
	// Round to 3 decimals for consistent comparison
	score = math.Round(score*1000) / 1000

	if score >= 7.000 {
		return CriticalityCritical
	}
	if score >= 4.000 {
		return CriticalityHigh
	}
	if score >= 2.000 {
		return CriticalityMedium
	}
	return CriticalityLow
}

// Breakdown implémente Engine.Breakdown.
// Retourne le détail complet avec explication et delta (si previousScore != nil).
func (e *engine) Breakdown(
	probability, impact, assetCriticality float64,
	previousScore *float64,
) (ScoreBreakdown, error) {
	// Calculate score (will validate ranges)
	score, err := e.Calculate(probability, impact, assetCriticality)
	if err != nil {
		return ScoreBreakdown{}, err
	}

	criticality := e.ToCriticality(score)

	// Build explanation: "0.700 × 8.000 × 1.500 = 8.400 → Critical"
	explanation := fmt.Sprintf(
		"%.3f × %.3f × %.3f = %.3f → %s",
		probability,
		impact,
		assetCriticality,
		score,
		strings.Title(string(criticality)),
	)

	breakdown := ScoreBreakdown{
		Score:            score,
		Probability:      probability,
		Impact:           impact,
		AssetCriticality: assetCriticality,
		Criticality:      criticality,
		Explanation:      explanation,
	}

	// Calculate delta if previousScore provided
	if previousScore != nil {
		breakdown.PreviousScore = previousScore
		delta := score - *previousScore
		breakdown.Delta = &delta
	}

	return breakdown, nil
}
