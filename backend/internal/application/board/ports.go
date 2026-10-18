// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

// Package board holds the monthly board-of-directors report use cases: it
// aggregates a tenant's risk and compliance posture, asks an ai.Advisor to write
// a non-technical narrative, and manages the human-in-the-loop draft → approved
// lifecycle. PDF rendering lives in pkg/report; the LLM client lives in pkg/ai.
package board

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// RiskPostureSource counts a tenant's active risks by criticality level.
// *repository.GormRiskRepository satisfies it (concrete method, kept off the
// domain.RiskRepository port so existing mocks stay valid).
type RiskPostureSource interface {
	CountRisksByCriticality(ctx context.Context, tenantID uuid.UUID) (map[string]int, error)
}

// AcceptanceSource lists a tenant's residual-risk acceptances that end on or
// before `by` (#811). Optional; *repository.GormRiskRepository satisfies it.
type AcceptanceSource interface {
	ListAcceptancesExpiring(ctx context.Context, tenantID uuid.UUID, by time.Time) ([]domain.Risk, error)
}

// AcceptanceHorizon is how far ahead the board report looks: a quarter, the
// usual gap between two board meetings.
const AcceptanceHorizon = 90 * 24 * time.Hour

// AcceptanceSnapshot is one acceptance frozen into a report.
type AcceptanceSnapshot struct {
	RiskID    string    `json:"risk_id"`
	Title     string    `json:"title"`
	ExpiresAt time.Time `json:"expires_at"`
	// ApprovedBy is the approver's user id, resolved to a name on render.
	ApprovedBy string `json:"approved_by,omitempty"`
}

// OrganizationLookup resolves a tenant's display name for the cover page.
// *repository.GormOrganizationRepository satisfies it.
type OrganizationLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// UserLookup resolves who generated/approved a report.
// *repository.GormUserRepository satisfies it.
type UserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// FrameworkSnapshot is the per-framework advancement frozen into a report at
// generation time. It is what gets JSON-encoded into BoardReport.FrameworksSnapshot
// and later decoded by the PDF renderer and the frontend.
type FrameworkSnapshot struct {
	Name            string  `json:"name"`
	Version         string  `json:"version"`
	Total           int     `json:"total"`
	Applicable      int     `json:"applicable"`
	Implemented     int     `json:"implemented"`
	PercentComplete float64 `json:"percent_complete"`
}
