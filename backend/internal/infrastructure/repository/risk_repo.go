// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

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
