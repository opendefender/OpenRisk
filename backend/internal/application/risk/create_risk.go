// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// CreateRiskInput represents the input for creating a risk.
type CreateRiskInput struct {
	Title       string
	Description string
	Impact      float64 // ERD numeric(5,1) — bounds [0,10]
	Probability float64 // ERD numeric(5,3) — bounds [0,1]
	Status      domain.RiskStatus
	Tags        []string
	Frameworks  []string
	Owner       string
	Source      string // parsed into domain.RiskSource in Execute()
	ExternalID  string
	CreatedBy   uuid.UUID // the authenticated user creating the risk
	SLEXAF      *float64  // CRQ: single loss expectancy (XAF), optional
	ARO         *float64  // CRQ: annualized rate of occurrence, optional
}

// CreateRiskUseCase handles the creation of a new risk.
type CreateRiskUseCase struct {
	riskRepo domain.RiskRepository
}

// NewCreateRiskUseCase creates a new CreateRiskUseCase.
func NewCreateRiskUseCase(riskRepo domain.RiskRepository) *CreateRiskUseCase {
	return &CreateRiskUseCase{riskRepo: riskRepo}
}

// Execute creates a new risk within the specified organization.
func (uc *CreateRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, input CreateRiskInput) (*domain.Risk, error) {
	// 1. Validate input
	if err := uc.validate(input); err != nil {
		return nil, err
	}

	// Convert the raw source string into the typed domain.RiskSource
	// (empty defaults to SourceManual; anything else must be a known value).
	source, err := domain.ParseRiskSource(input.Source)
	if err != nil {
		return nil, err
	}

	// 2. Build domain entity
	// TenantID is the canonical (not-null) field; OrganizationID is kept as
	// its legacy alias. Risk.BeforeSave() also syncs the two on real GORM
	// writes, but setting both explicitly here avoids depending solely on
	// that hook (e.g. against mocked repositories in tests).
	risk := &domain.Risk{
		ID:             uuid.New(),
		Title:          input.Title,
		Description:    input.Description,
		Impact:         input.Impact,
		Probability:    input.Probability,
		Tags:           input.Tags,
		Frameworks:     input.Frameworks,
		Owner:          input.Owner,
		Source:         source,
		ExternalID:     input.ExternalID,
		TenantID:       orgID,
		OrganizationID: orgID,
		CreatedBy:      input.CreatedBy,
		SLEXAF:         input.SLEXAF,
		ARO:            input.ARO,
	}

	// Set status (default to DRAFT)
	if input.Status != "" {
		risk.Status = input.Status
	} else {
		risk.Status = domain.StatusDraft
	}

	// A newly created risk enters the lifecycle at "Identifier" (ISO 31000).
	risk.LifecyclePhase = domain.PhaseIdentified

	// 3. Compute score (Claude.md formula: P × I, score engine can override later)
	risk.Score = risk.Impact * risk.Probability

	// 4. Persist
	if err := uc.riskRepo.Create(ctx, risk); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to create risk: %v", err))
	}

	return risk, nil
}

func (uc *CreateRiskUseCase) validate(input CreateRiskInput) error {
	if input.Title == "" {
		return domain.NewValidationError("title is required")
	}
	if len(input.Title) > 255 {
		return domain.NewValidationError("title must be 255 characters or less")
	}
	if input.Impact < 0 || input.Impact > 10 {
		return domain.NewValidationError("impact must be between 0 and 10")
	}
	if input.Probability < 0 || input.Probability > 1 {
		return domain.NewValidationError("probability must be between 0 and 1")
	}
	return nil
}
