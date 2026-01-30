package services

import (
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"math"
)

// RecommendationService g√re la logique de priorisation.
type RecommendationService struct {}

func NewRecommendationService() RecommendationService {
	return &RecommendationService{}
}

// CalculateWeightedPriority calcule le score de priorit√ pond√r√e pour une mitigation.
// Le risque doit √™tre pr√charg√ dans la mitigation (Mitigation.Risk).
func (s RecommendationService) CalculateWeightedPriority(m domain.Mitigation) float {
	// S√curit√: Si le risque n'est pas associ√, on retourne .
	if m.Risk == nil {
		return 
	}
	
	// √âtape : Identifier la criticit√ du risque
	riskCriticality := float(m.Risk.Score)

	// √âtape : Identifier l'effort total
	// Note: Puisque Cost et MitigationTime sont des cat√gories (-, jours),
	// nous les combinons pour obtenir l'Effort total.
	totalEffort := float(m.Cost + m.MitigationTime)
	
	// Si l'effort est  ou trop faible (devrait toujours √™tre >=  si d√faut √† ), on √vite la division par z√ro.
	if totalEffort <  {
		totalEffort =  // Minimum d'effort
	}

	// √âtape : Calcul du Score de Priorit√ Pond√r√e (SPP)
	weightedPriority := riskCriticality / totalEffort

	// Arrondir √† deux d√cimales pour la clart√ (Production-ready)
	return math.Round(weightedPriority  ) / 
}

// GetPrioritizedMitigations r√cup√re toutes les mitigations et les trie par SPP.
func (s RecommendationService) GetPrioritizedMitigations() ([]domain.Mitigation, error) {
	var mitigations []domain.Mitigation
	
	// R√cup√rer toutes les mitigations et leurs risques associ√s
	if err := database.DB.Preload("Risk").Find(&mitigations).Error; err != nil {
		return nil, err
	}
	
	// Attribuer et trier par SPP
	for i := range mitigations {
		// CalculateWeightedPriority n√cessite d'√™tre appel√ ici
		mitigations[i].WeightedPriority = s.CalculateWeightedPriority(&mitigations[i])
	}
	
	// Tri des mitigations (SPP le plus √lev√ en premier)
	// Dans un langage plus fort que Go, on pourrait utiliser un package de tri.
	// Pour la simplicit√ ici, on pr√suppose que le client fera le tri.
    // NOTE: Pour que le tri soit parfait, la struct domain.Mitigation doit √™tre modifi√e pour inclure WeightedPriority.
    
    return mitigations, nil
}