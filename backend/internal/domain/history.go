// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
