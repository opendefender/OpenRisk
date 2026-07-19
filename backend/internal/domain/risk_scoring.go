// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/pkg/scoring"
)

// RiskScoringWeights is the per-tenant configuration of the smart-risk model's
// eight factor weights (spec §8 "Calcul de risque intelligent"). Exactly one row
// per tenant (unique index on tenant_id); a tenant with no row uses the built-in
// defaults. Weights are stored as their own columns so they are queryable and a
// migration can seed defaults. They are relative — pkg/scoring normalises them.
type RiskScoringWeights struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"tenant_id"`

	BusinessCriticality float64 `gorm:"type:numeric(6,4);not null;default:0.15" json:"business_criticality"`
	InternetExposure    float64 `gorm:"type:numeric(6,4);not null;default:0.10" json:"internet_exposure"`
	Vulnerabilities     float64 `gorm:"type:numeric(6,4);not null;default:0.20" json:"vulnerabilities"`
	ControlMaturity     float64 `gorm:"type:numeric(6,4);not null;default:0.10" json:"control_maturity"`
	IncidentHistory     float64 `gorm:"type:numeric(6,4);not null;default:0.10" json:"incident_history"`
	Exploitability      float64 `gorm:"type:numeric(6,4);not null;default:0.15" json:"exploitability"`
	FinancialValue      float64 `gorm:"type:numeric(6,4);not null;default:0.10" json:"financial_value"`
	ThreatIntel         float64 `gorm:"type:numeric(6,4);not null;default:0.10" json:"threat_intel"`

	UpdatedBy uuid.UUID `gorm:"type:uuid" json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (RiskScoringWeights) TableName() string { return "risk_scoring_weights" }

// DefaultRiskScoringWeights returns a weights row seeded with the engine defaults
// for a tenant. Used when a tenant has never customised its weights.
func DefaultRiskScoringWeights(tenantID uuid.UUID) *RiskScoringWeights {
	d := scoring.DefaultFactorWeights()
	return &RiskScoringWeights{
		TenantID:            tenantID,
		BusinessCriticality: d[scoring.FactorBusinessCriticality],
		InternetExposure:    d[scoring.FactorInternetExposure],
		Vulnerabilities:     d[scoring.FactorVulnerabilities],
		ControlMaturity:     d[scoring.FactorControlMaturity],
		IncidentHistory:     d[scoring.FactorIncidentHistory],
		Exploitability:      d[scoring.FactorExploitability],
		FinancialValue:      d[scoring.FactorFinancialValue],
		ThreatIntel:         d[scoring.FactorThreatIntel],
	}
}

// ToFactorWeights maps the persisted columns onto the engine's weight map.
func (w *RiskScoringWeights) ToFactorWeights() scoring.FactorWeights {
	return scoring.FactorWeights{
		scoring.FactorBusinessCriticality: w.BusinessCriticality,
		scoring.FactorInternetExposure:    w.InternetExposure,
		scoring.FactorVulnerabilities:     w.Vulnerabilities,
		scoring.FactorControlMaturity:     w.ControlMaturity,
		scoring.FactorIncidentHistory:     w.IncidentHistory,
		scoring.FactorExploitability:      w.Exploitability,
		scoring.FactorFinancialValue:      w.FinancialValue,
		scoring.FactorThreatIntel:         w.ThreatIntel,
	}
}

// ApplyFactorWeights overwrites the columns from a weight map (used on update).
// Unknown keys are ignored; missing keys leave the current value untouched.
func (w *RiskScoringWeights) ApplyFactorWeights(fw scoring.FactorWeights) {
	if v, ok := fw[scoring.FactorBusinessCriticality]; ok {
		w.BusinessCriticality = v
	}
	if v, ok := fw[scoring.FactorInternetExposure]; ok {
		w.InternetExposure = v
	}
	if v, ok := fw[scoring.FactorVulnerabilities]; ok {
		w.Vulnerabilities = v
	}
	if v, ok := fw[scoring.FactorControlMaturity]; ok {
		w.ControlMaturity = v
	}
	if v, ok := fw[scoring.FactorIncidentHistory]; ok {
		w.IncidentHistory = v
	}
	if v, ok := fw[scoring.FactorExploitability]; ok {
		w.Exploitability = v
	}
	if v, ok := fw[scoring.FactorFinancialValue]; ok {
		w.FinancialValue = v
	}
	if v, ok := fw[scoring.FactorThreatIntel]; ok {
		w.ThreatIntel = v
	}
}

// Validate rejects a weights configuration that is unusable: every weight must be
// in [0,1] and at least one must be strictly positive (else there is nothing to
// normalise). Returns a typed ErrValidation.
func (w *RiskScoringWeights) Validate() error {
	weights := []struct {
		name string
		val  float64
	}{
		{"business_criticality", w.BusinessCriticality},
		{"internet_exposure", w.InternetExposure},
		{"vulnerabilities", w.Vulnerabilities},
		{"control_maturity", w.ControlMaturity},
		{"incident_history", w.IncidentHistory},
		{"exploitability", w.Exploitability},
		{"financial_value", w.FinancialValue},
		{"threat_intel", w.ThreatIntel},
	}
	var positive bool
	for _, wt := range weights {
		if wt.val < 0 || wt.val > 1 {
			return NewValidationError(wt.name + " weight must be between 0 and 1")
		}
		if wt.val > 0 {
			positive = true
		}
	}
	if !positive {
		return NewValidationError("at least one factor weight must be greater than 0")
	}
	return nil
}

// RiskScoringWeightsRepository is the persistence port for per-tenant weights.
// ABSOLUTE RULE: every method is scoped by tenant_id.
type RiskScoringWeightsRepository interface {
	// GetByTenant returns the tenant's weights, or (nil, nil) if never customised
	// (the use case then falls back to DefaultRiskScoringWeights).
	GetByTenant(ctx context.Context, tenantID uuid.UUID) (*RiskScoringWeights, error)

	// Upsert inserts or updates the tenant's single weights row.
	Upsert(ctx context.Context, w *RiskScoringWeights) error
}
