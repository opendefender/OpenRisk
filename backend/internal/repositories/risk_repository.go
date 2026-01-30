package repositories

import (
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// CreateRiskIfNotExists implmente la logique d'upsert (mettre à jour si existe, crer si non).
// Ceci est crucial pour le moteur de synchronisation.
func CreateRiskIfNotExists(risk domain.Risk) error {
	// . Tenter de trouver un risque existant par ExternalID et Source
	var existingRisk domain.Risk
	result := database.DB.Where("external_id = ? AND source = ?", risk.ExternalID, risk.Source).First(&existingRisk)

	if result.Error == nil {
		// Risque trouv: Mettre à jour l'enregistrement existant
		// Pour l'instant, on se contente de mettre à jour le score et le statut
		risk.ID = existingRisk.ID 
		return database.DB.Model(&existingRisk).Updates(risk).Error
	}

	// . Risque non trouv ou erreur de type 'not found': Crer un nouveau risque
	return database.DB.Create(risk).Error
}