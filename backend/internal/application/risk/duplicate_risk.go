// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// DuplicateRiskUseCase handles duplicating an existing risk
// The new risk will have the same properties but different ID
type DuplicateRiskUseCase struct {
	riskRepo domain.RiskRepository
}

// NewDuplicateRiskUseCase creates a new DuplicateRiskUseCase
func NewDuplicateRiskUseCase(riskRepo domain.RiskRepository) *DuplicateRiskUseCase {
	return &DuplicateRiskUseCase{riskRepo: riskRepo}
}

// Execute duplicates a risk and creates a new one
// The new risk will have "(Copy)" appended to the name
func (uc *DuplicateRiskUseCase) Execute(ctx context.Context, tenantID uuid.UUID, riskID uuid.UUID, duplicatedBy uuid.UUID) (*domain.Risk, error) {
	// 1. Fetch source risk (tenant-scoped)
	sourceRisk, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if sourceRisk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// 2. Create copy with new ID and updated metadata
	newRisk := &domain.Risk{
		ID:                uuid.New(),
		TenantID:          sourceRisk.TenantID,
		OrganizationID:    sourceRisk.OrganizationID,
		Name:              sourceRisk.Name + " (Copy)",
		Title:             sourceRisk.Title + " (Copy)",
		Description:       sourceRisk.Description,
		Probability:       sourceRisk.Probability,
		Impact:            sourceRisk.Impact,
		Score:             0, // Will be recalculated by Score Engine
		Criticality:       sourceRisk.Criticality,
		ImpactLegacy:      sourceRisk.ImpactLegacy,
		ProbabilityLegacy: sourceRisk.ProbabilityLegacy,
		Status:            domain.RiskOpen, // Reset to "open"
		Level:             sourceRisk.Level,
		CreatedBy:         duplicatedBy,
		AssignedTo:        sourceRisk.AssignedTo, // Copy assignment
		ReviewerID:        sourceRisk.ReviewerID,
		Owner:             sourceRisk.Owner,
		AssetID:           sourceRisk.AssetID,
		Tags:              sourceRisk.Tags,
		Frameworks:        sourceRisk.Frameworks,
		ControlIDs:        sourceRisk.ControlIDs,
		TreatmentPlan:     sourceRisk.TreatmentPlan,
		ResidualRisk:      sourceRisk.ResidualRisk,
		Source:            "manual", // Mark as manually created (not auto-generated)
		SourceCVEID:       sourceRisk.SourceCVEID,
		CustomFields:      sourceRisk.CustomFields,
	}

	// 3. Persist new risk
	if err := uc.riskRepo.Create(ctx, newRisk); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to duplicate risk: %v", err))
	}

	return newRisk, nil
}
