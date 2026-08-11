// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// OwnershipResolver fills the computed owner/assignee/reviewer emails in one
// batched lookup. Narrow port, satisfied structurally by
// application/ownership.Service. Optional and nil-safe.
type OwnershipResolver interface {
	ResolveEmails(ctx context.Context, entities ...domain.OwnedEntity)
}

// GetRiskUseCase handles retrieving a single risk.
type GetRiskUseCase struct {
	riskRepo domain.RiskRepository
	mappings domain.RiskControlMappingRepository
	owners   OwnershipResolver
}

func NewGetRiskUseCase(riskRepo domain.RiskRepository) *GetRiskUseCase {
	return &GetRiskUseCase{riskRepo: riskRepo}
}

// WithMappings attaches the control-mapping repository. Optional, nil-safe.
func (uc *GetRiskUseCase) WithMappings(m domain.RiskControlMappingRepository) *GetRiskUseCase {
	uc.mappings = m
	return uc
}

// WithOwnership attaches the ownership email resolver. Optional, nil-safe.
func (uc *GetRiskUseCase) WithOwnership(r OwnershipResolver) *GetRiskUseCase {
	uc.owners = r
	return uc
}

// Execute retrieves a risk by ID, scoped to the organization.
func (uc *GetRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, riskID uuid.UUID) (*domain.Risk, error) {
	risk, err := uc.riskRepo.GetByID(ctx, riskID, orgID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// Both enrichments degrade to empty rather than failing the read.
	if uc.mappings != nil {
		if rows, err := uc.mappings.ListByRisk(ctx, orgID, riskID); err == nil {
			risk.ControlMappings = rows
		}
	}
	if uc.owners != nil {
		uc.owners.ResolveEmails(ctx, risk)
	}
	return risk, nil
}
