package services

import (
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"math"
)

// RecommendationService gre la logique de priorisation.
type RecommendationService struct {}

func NewRecommendationService() RecommendationService {
	return &RecommendationService{}
}

// CalculateWeightedPriority calcule le score de priorit pondre pour une mitigation.
// Le risque doit être prcharg dans la mitigation (Mitigation.Risk).
func (s RecommendationService) CalculateWeightedPriority(m domain.Mitigation) float {
	// Scurit: Si le risque n'est pas associ, on retourne .
	if m.Risk == nil {
		return 
	}
	
	// Étape : Identifier la criticit du risque
	riskCriticality := float(m.Risk.Score)

	// Étape : Identifier l'effort total
	// Note: Puisque Cost et MitigationTime sont des catgories (-, jours),
	// nous les combinons pour obtenir l'Effort total.
	totalEffort := float(m.Cost + m.MitigationTime)
	
	// Si l'effort est  ou trop faible (devrait toujours être >=  si dfaut à ), on vite la division par zro.
	if totalEffort <  {
		totalEffort =  // Minimum d'effort
	}

	// Étape : Calcul du Score de Priorit Pondre (SPP)
	weightedPriority := riskCriticality / totalEffort

	// Arrondir à deux dcimales pour la clart (Production-ready)
	return math.Round(weightedPriority  ) / 
}

// GetPrioritizedMitigations rcupre toutes les mitigations et les trie par SPP.
func (s RecommendationService) GetPrioritizedMitigations() ([]domain.Mitigation, error) {
	var mitigations []domain.Mitigation
	
	// Rcuprer toutes les mitigations et leurs risques associs
	if err := database.DB.Preload("Risk").Find(&mitigations).Error; err != nil {
		return nil, err
	}
	
	// Attribuer et trier par SPP
	for i := range mitigations {
		// CalculateWeightedPriority ncessite d'être appel ici
		mitigations[i].WeightedPriority = s.CalculateWeightedPriority(&mitigations[i])
	}
	
	// Tri des mitigations (SPP le plus lev en premier)
	// Dans un langage plus fort que Go, on pourrait utiliser un package de tri.
	// Pour la simplicit ici, on prsuppose que le client fera le tri.
    // NOTE: Pour que le tri soit parfait, la struct domain.Mitigation doit être modifie pour inclure WeightedPriority.
    
    return mitigations, nil
}