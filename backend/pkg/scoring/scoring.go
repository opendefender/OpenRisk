// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scoring

// RÈGLE ABSOLUE: Ce package n'importe RIEN de GORM, Fiber, Redis,
// ou de tout autre layer d'infrastructure. Zéro dépendance externe.
// Uniquement: math, fmt, errors, et les types du domain local.

// CriticalityLevel représente le niveau de criticité d'un risque.
type CriticalityLevel string

const (
	CriticalityLow      CriticalityLevel = "low"
	CriticalityMedium   CriticalityLevel = "medium"
	CriticalityHigh     CriticalityLevel = "high"
	CriticalityCritical CriticalityLevel = "critical"
)

// ScoreBreakdown contient le détail complet d'un calcul de score.
type ScoreBreakdown struct {
	// Score final (arrondi à 3 décimales)
	Score float64 `json:"score"`

	// Composants du calcul
	Probability      float64 `json:"probability"`      // 0.0 – 1.0
	Impact           float64 `json:"impact"`           // 0.0 – 10.0
	AssetCriticality float64 `json:"asset_criticality"` // 0.1 – 3.0

	// Résultat de criticité
	Criticality CriticalityLevel `json:"criticality"`

	// Explication du calcul (ex: "0.700 × 8.000 × 1.500 = 8.400 → Critical")
	Explanation string `json:"explanation"`

	// Score précédent (nil si premier calcul)
	PreviousScore *float64 `json:"previous_score,omitempty"`

	// Delta (score actuel - précédent, nil si premier calcul)
	Delta *float64 `json:"delta,omitempty"`
}

// Engine est l'interface du moteur de scoring.
// Toute implémentation doit être pure: pas d'I/O, pas d'état mutable.
type Engine interface {
	// Calculate retourne le score final (arrondi à 3 décimales) ou
	// une erreur typée ErrValidation si les paramètres sont hors range.
	Calculate(probability, impact, assetCriticality float64) (float64, error)

	// ToCriticality convertit un score en CriticalityLevel.
	// Seuils exacts (précision 3 décimales):
	//   score >= 7.000 → Critical
	//   score >= 4.000 → High
	//   score >= 2.000 → Medium
	//   score <  2.000 → Low
	ToCriticality(score float64) CriticalityLevel

	// Breakdown retourne le détail complet avec explication et delta.
	Breakdown(
		probability, impact, assetCriticality float64,
		previousScore *float64,
	) (ScoreBreakdown, error)
}
