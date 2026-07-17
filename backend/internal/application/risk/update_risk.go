// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package risk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// UpdateRiskInput represents the input for updating a risk.
// Pointer fields allow partial updates (nil = don't update).
type UpdateRiskInput struct {
	Title       *string
	Description *string
	Impact      *float64 // ERD numeric(5,1) — bounds [0,10]
	Probability *float64 // ERD numeric(5,3) — bounds [0,1]
	Status      *domain.RiskStatus
	Tags        []string
	Frameworks  []string
	Owner       *string
	// CRQ monetary inputs (XAF). Pointers so a partial update can set or clear them.
	SLEXAF *float64
	ARO    *float64
	// Full financial-quantification drivers (spec §9). Pointers → nil leaves the
	// stored value unchanged; a supplied value (incl. 0) overwrites it.
	DowntimeHours           *float64
	HourlyDowntimeCostXAF   *float64
	DataLossCostXAF         *float64
	FinesXAF                *float64
	OtherDirectCostXAF      *float64
	RemediationCostXAF      *float64
	MitigationEffectiveness *float64 // [0,1]
	// Review cadence (days). 0 disables; >0 (re)initialises NextReviewAt when unset.
	ReviewIntervalDays *int
}

// UpdateRiskUseCase handles updating an existing risk.
type UpdateRiskUseCase struct {
	riskRepo domain.RiskRepository
}

func NewUpdateRiskUseCase(riskRepo domain.RiskRepository) *UpdateRiskUseCase {
	return &UpdateRiskUseCase{riskRepo: riskRepo}
}

// Execute updates a risk by ID, scoped to the organization.
func (uc *UpdateRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, riskID uuid.UUID, input UpdateRiskInput) (*domain.Risk, error) {
	// 1. Fetch existing risk (tenant-scoped)
	risk, err := uc.riskRepo.GetByID(ctx, riskID, orgID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// 2. Apply partial updates
	if input.Title != nil {
		if *input.Title == "" {
			return nil, domain.NewValidationError("title cannot be empty")
		}
		risk.Title = *input.Title
	}
	if input.Description != nil {
		risk.Description = *input.Description
	}
	if input.Impact != nil {
		if *input.Impact < 0 || *input.Impact > 10 {
			return nil, domain.NewValidationError("impact must be between 0 and 10")
		}
		risk.Impact = *input.Impact
	}
	if input.Probability != nil {
		if *input.Probability < 0 || *input.Probability > 1 {
			return nil, domain.NewValidationError("probability must be between 0 and 1")
		}
		risk.Probability = *input.Probability
	}
	if input.Status != nil {
		risk.Status = *input.Status
	}
	if input.Tags != nil {
		risk.Tags = input.Tags
	}
	if input.Frameworks != nil {
		risk.Frameworks = input.Frameworks
	}
	if input.Owner != nil {
		risk.Owner = *input.Owner
	}
	if input.SLEXAF != nil {
		if *input.SLEXAF < 0 {
			return nil, domain.NewValidationError("single loss expectancy (sle_xaf) cannot be negative")
		}
		risk.SLEXAF = input.SLEXAF
	}
	if input.ARO != nil {
		if *input.ARO < 0 {
			return nil, domain.NewValidationError("annualized rate of occurrence (aro) cannot be negative")
		}
		risk.ARO = input.ARO
	}
	// Financial-quantification drivers. Reject negatives; effectiveness ∈ [0,1].
	for label, p := range map[string]*float64{
		"downtime_hours":           input.DowntimeHours,
		"hourly_downtime_cost_xaf": input.HourlyDowntimeCostXAF,
		"data_loss_cost_xaf":       input.DataLossCostXAF,
		"fines_xaf":                input.FinesXAF,
		"other_direct_cost_xaf":    input.OtherDirectCostXAF,
		"remediation_cost_xaf":     input.RemediationCostXAF,
	} {
		if p != nil && *p < 0 {
			return nil, domain.NewValidationError(label + " cannot be negative")
		}
	}
	if input.MitigationEffectiveness != nil && (*input.MitigationEffectiveness < 0 || *input.MitigationEffectiveness > 1) {
		return nil, domain.NewValidationError("mitigation_effectiveness must be between 0 and 1")
	}
	if input.DowntimeHours != nil {
		risk.DowntimeHours = input.DowntimeHours
	}
	if input.HourlyDowntimeCostXAF != nil {
		risk.HourlyDowntimeCostXAF = input.HourlyDowntimeCostXAF
	}
	if input.DataLossCostXAF != nil {
		risk.DataLossCostXAF = input.DataLossCostXAF
	}
	if input.FinesXAF != nil {
		risk.FinesXAF = input.FinesXAF
	}
	if input.OtherDirectCostXAF != nil {
		risk.OtherDirectCostXAF = input.OtherDirectCostXAF
	}
	if input.RemediationCostXAF != nil {
		risk.RemediationCostXAF = input.RemediationCostXAF
	}
	if input.MitigationEffectiveness != nil {
		risk.MitigationEffectiveness = input.MitigationEffectiveness
	}
	if input.ReviewIntervalDays != nil {
		if *input.ReviewIntervalDays < 0 {
			return nil, domain.NewValidationError("review_interval_days cannot be negative")
		}
		risk.ReviewIntervalDays = *input.ReviewIntervalDays
		// Enabling a cadence with no scheduled review yet → schedule the first one.
		if risk.ReviewIntervalDays > 0 && risk.NextReviewAt == nil {
			next := time.Now().Add(time.Duration(risk.ReviewIntervalDays) * 24 * time.Hour)
			risk.NextReviewAt = &next
		}
		if risk.ReviewIntervalDays == 0 {
			risk.NextReviewAt = nil
		}
	}

	// 3. Recompute score
	risk.Score = risk.Impact * risk.Probability

	// 4. Persist
	if err := uc.riskRepo.Update(ctx, risk); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to update risk: %v", err))
	}

	return risk, nil
}
