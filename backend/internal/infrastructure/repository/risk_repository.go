package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

// CreateRiskIfNotExists implémente la logique d'upsert (mettre à jour si existe, créer si non).
// Ceci est crucial pour le moteur de synchronisation.
// IMPORTANT: organization_id MUST be set in risk - this function will not accept risks without organization_id.
func CreateRiskIfNotExists(ctx context.Context, risk *domain.Risk) error {
	// Validate that organization_id is set (Rule 1: tenant scoping on every DB query)
	if risk.OrganizationID == uuid.Nil {
		return fmt.Errorf("organization_id must be set before creating risk (tenant scoping violation)")
	}

	// 1. Tenter de trouver un risque existant par ExternalID, Source ET OrganizationID (scoped to org)
	var existingRisk domain.Risk
	result := database.DB.WithContext(ctx).
		Where("external_id = ? AND source = ? AND organization_id = ?", risk.ExternalID, risk.Source, risk.OrganizationID).
		First(&existingRisk)

	if result.Error == nil {
		// Risque trouvé: Mettre à jour l'enregistrement existant
		// Pour l'instant, on se contente de mettre à jour le score et le statut
		risk.ID = existingRisk.ID
		return database.DB.WithContext(ctx).Model(&existingRisk).Updates(risk).Error
	}

	if result.Error != gorm.ErrRecordNotFound {
		// Unexpected error
		return fmt.Errorf("failed to check existing risk: %w", result.Error)
	}

	// 2. Risque non trouvé: Créer un nouveau risque
	return database.DB.WithContext(ctx).Create(risk).Error
}
