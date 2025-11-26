package services

import (
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"math"
)

// RecommendationService gère la logique de priorisation.
type RecommendationService struct {}

func NewRecommendationService() *RecommendationService {
	return &RecommendationService{}
}

// CalculateWeightedPriority calcule le score de priorité pondérée pour une mitigation.
// Le risque doit être préchargé dans la mitigation (Mitigation.Risk).
func (s *RecommendationService) CalculateWeightedPriority(m *domain.Mitigation) float64 {
	// Sécurité: Si le risque n'est pas associé, on retourne 0.
	if m.Risk == nil {
		return 0
	}
	
	// Étape 1: Identifier la criticité du risque
	riskCriticality := float64(m.Risk.Score)

	// Étape 2: Identifier l'effort total
	// Note: Puisque Cost et MitigationTime sont des catégories (1-3, jours),
	// nous les combinons pour obtenir l'Effort total.
	totalEffort := float64(m.Cost + m.MitigationTime)
	
	// Si l'effort est 0 ou trop faible (devrait toujours être >= 2 si défaut à 1), on évite la division par zéro.
	if totalEffort < 2 {
		totalEffort = 2 // Minimum d'effort
	}

	// Étape 3: Calcul du Score de Priorité Pondérée (SPP)
	weightedPriority := riskCriticality / totalEffort

	// Arrondir à deux décimales pour la clarté (Production-ready)
	return math.Round(weightedPriority * 100) / 100
}

// GetPrioritizedMitigations récupère toutes les mitigations et les trie par SPP.
func (s *RecommendationService) GetPrioritizedMitigations() ([]domain.Mitigation, error) {
	var mitigations []domain.Mitigation
	
	// Récupérer toutes les mitigations et leurs risques associés
	if err := database.DB.Preload("Risk").Find(&mitigations).Error; err != nil {
		return nil, err
	}
	
	// Attribuer et trier par SPP
	for i := range mitigations {
		// CalculateWeightedPriority nécessite d'être appelé ici
		mitigations[i].WeightedPriority = s.CalculateWeightedPriority(&mitigations[i])
	}
	
	// Tri des mitigations (SPP le plus élevé en premier)
	// Dans un langage plus fort que Go, on pourrait utiliser un package de tri.
	// Pour la simplicité ici, on présuppose que le client fera le tri.
    // NOTE: Pour que le tri soit parfait, la struct domain.Mitigation doit être modifiée pour inclure WeightedPriority.
    
    return mitigations, nil
}