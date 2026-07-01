// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package events

// Noms des channels Redis — constantes, jamais de strings hardcodées
// ailleurs dans le projet.
const (
	// Publié par les handlers après création/modification d'un risque.
	// Payload: RiskUpdatedEvent
	// Consumer: ScoreWorker (calcule le score et publie RiskScoreUpdated)
	RiskUpdated = "risk.updated"

	// Publié par le score_worker après recalcul.
	// Payload: RiskScoreUpdatedEvent
	// Consumers: SSE hub (Module 7), Notification service (Module 9),
	//           Dashboard cache invalidation (Module 8)
	RiskScoreUpdated = "risk.score_updated"

	// Publié quand la criticité d'un asset change.
	// Payload: AssetCriticalityChangedEvent
	// Consumer: ScoreWorker (republish risk.updated pour tous les risques liés)
	AssetCriticalityChanged = "asset.criticality_changed"
)

// RiskUpdatedEvent est le payload publié sur risk.updated.
// Format: JSON serializable
type RiskUpdatedEvent struct {
	RiskID           string  `json:"risk_id"`
	TenantID         string  `json:"tenant_id"`
	Probability      float64 `json:"probability"`
	Impact           float64 `json:"impact"`
	AssetCriticality float64 `json:"asset_criticality"`
	TriggeredBy      string  `json:"triggered_by"` // user_id ou "system"
}

// RiskScoreUpdatedEvent est le payload publié sur risk.score_updated.
// Consommé par: SSE hub (Module 7), Notification service (Module 9),
//
//	Dashboard analytics cache invalidation (Module 8).
type RiskScoreUpdatedEvent struct {
	RiskID       string  `json:"risk_id"`
	TenantID     string  `json:"tenant_id"`
	NewScore     float64 `json:"new_score"`
	OldScore     float64 `json:"old_score"`
	Delta        float64 `json:"delta"` // new - old
	Criticality  string  `json:"criticality"`
	CalculatedAt string  `json:"calculated_at"` // RFC3339
}

// AssetCriticalityChangedEvent est le payload publié sur asset.criticality_changed.
type AssetCriticalityChangedEvent struct {
	AssetID        string `json:"asset_id"`
	TenantID       string `json:"tenant_id"`
	OldCriticality string `json:"old_criticality"`
	NewCriticality string `json:"new_criticality"`
	ChangedBy      string `json:"changed_by"` // user_id
	ChangedAt      string `json:"changed_at"` // RFC3339
}
