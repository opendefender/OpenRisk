// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package board holds the monthly board-of-directors report use cases: it
// aggregates a tenant's risk and compliance posture, asks an ai.Advisor to write
// a non-technical narrative, and manages the human-in-the-loop draft → approved
// lifecycle. PDF rendering lives in pkg/report; the LLM client lives in pkg/ai.
package board

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// RiskPostureSource counts a tenant's active risks by criticality level.
// *repository.GormRiskRepository satisfies it (concrete method, kept off the
// domain.RiskRepository port so existing mocks stay valid).
type RiskPostureSource interface {
	CountRisksByCriticality(ctx context.Context, tenantID uuid.UUID) (map[string]int, error)
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
