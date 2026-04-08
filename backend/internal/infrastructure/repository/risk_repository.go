package repositories

import (
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

// CreateRiskIfNotExists implémente la logique d'upsert (mettre à jour si existe, créer si non).
// Ceci est crucial pour le moteur de synchronisation.
func CreateRiskIfNotExists(risk *domain.Risk) error {
	// 1. Tenter de trouver un risque existant par ExternalID et Source
	var existingRisk domain.Risk
	result := database.DB.Where("external_id = ? AND source = ?", risk.ExternalID, risk.Source).First(&existingRisk)

	if result.Error == nil {
		// Risque trouvé: Mettre à jour l'enregistrement existant
		// Pour l'instant, on se contente de mettre à jour le score et le statut
		risk.ID = existingRisk.ID
		return database.DB.Model(&existingRisk).Updates(risk).Error
	}

	// 2. Risque non trouvé ou erreur de type 'not found': Créer un nouveau risque
	return database.DB.Create(risk).Error
}
