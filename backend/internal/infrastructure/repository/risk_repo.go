// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
