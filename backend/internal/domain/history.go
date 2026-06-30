// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"time"

	"github.com/google/uuid"
)

// RiskHistory : Trace l'évolution d'un risque dans le temps
type RiskHistory struct {
	ID     uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RiskID uuid.UUID `gorm:"type:uuid;index" json:"risk_id"`

	// Snapshot des valeurs clés
	Score       float64    `json:"score"`
	Impact      int        `json:"impact"`
	Probability int        `json:"probability"`
	Status      RiskStatus `json:"status"`

	// Qui et Quand
	ChangedBy  string    `json:"changed_by"`  // User ID ou "System" (SyncEngine)
	ChangeType string    `json:"change_type"` // CREATE, UPDATE, MITIGATE
	CreatedAt  time.Time `json:"created_at"`  // Timestamp du changement
}
