package repositories

import (
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

func CreateRisk(risk *domain.Risk) error {
	return database.DB.Create(risk).Error
}

func GetAllRisks() ([]domain.Risk, error) {
	var risks []domain.Risk
	result := database.DB.Order("score desc").Find(&risks)
	return risks, result.Error
}
