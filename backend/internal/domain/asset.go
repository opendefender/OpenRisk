// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AssetCriticality string

const (
	CriticalityLow      AssetCriticality = "LOW"
	CriticalityMedium   AssetCriticality = "MEDIUM"
	CriticalityHigh     AssetCriticality = "HIGH"
	CriticalityCritical AssetCriticality = "CRITICAL"
)

// ScoreFactor maps an AssetCriticality to the numeric multiplier consumed by
// pkg/scoring.Engine (declared range [0.1, 3.0] — see CLAUDE.md's Score Engine
// formula). This is the single source of truth for that mapping: previously
// three different call sites (get_score_breakdown.go, score_service.go,
// score_engine_service.go) each hardcoded their own, inconsistent values.
// Unknown/empty criticality defaults to MEDIUM's factor.
func (c AssetCriticality) ScoreFactor() float64 {
	switch c {
	case CriticalityLow:
		return 0.5
	case CriticalityHigh:
		return 2.5
	case CriticalityCritical:
		return 3.0
	default:
		return 1.5 // MEDIUM and unknown values
	}
}

type Asset struct {
	ID             uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID       uuid.UUID        `gorm:"type:uuid;index" json:"tenant_id"`
	OrganizationID uuid.UUID        `gorm:"index" json:"organization_id"`
	Name           string           `gorm:"not null" json:"name"`
	Type           string           `json:"type"` // Server, Laptop, Database, SaaS
	Criticality    AssetCriticality `gorm:"default:'MEDIUM'" json:"criticality"`
	Owner          string           `json:"owner"`

	// Relation Many-to-Many avec Risk
	Risks []*Risk `gorm:"many2many:risk_assets;" json:"risks,omitempty"`

	Source     string `gorm:"default:'MANUAL'" json:"source"` // MANUAL ou OPENASSET
	ExternalID string `json:"external_id"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeSave keeps TenantID and OrganizationID in sync — OrganizationID is
// the historical field name (still used by the legacy handler query paths),
// TenantID is the canonical name used by the new repository/use-case layer
// (mirrors domain.Risk's TenantID/OrganizationID alias pattern).
func (a *Asset) BeforeSave(tx *gorm.DB) error {
	if a.TenantID == uuid.Nil && a.OrganizationID != uuid.Nil {
		a.TenantID = a.OrganizationID
	}
	if a.OrganizationID == uuid.Nil && a.TenantID != uuid.Nil {
		a.OrganizationID = a.TenantID
	}
	return nil
}

// AssetSnapshot captures an asset's state at a point in time, taken
// immediately before an update or deletion is applied. This is what powers
// the asset inventory's history view (ROADMAP.md M3): "what did this asset
// look like, and when did its criticality change".
type AssetSnapshot struct {
	ID          uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID        `gorm:"type:uuid;not null;index" json:"tenant_id"`
	AssetID     uuid.UUID        `gorm:"type:uuid;not null;index" json:"asset_id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Criticality AssetCriticality `json:"criticality"`
	Owner       string           `json:"owner"`
	// Reason describes why the snapshot was taken: "update" or "delete".
	Reason string `gorm:"size:20;not null;default:'update'" json:"reason"`

	CreatedAt time.Time `json:"created_at"`
}

// TableName overrides the default GORM table name.
func (AssetSnapshot) TableName() string {
	return "asset_snapshots"
}
