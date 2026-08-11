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

// ListRisksUseCase handles listing risks with filtering and pagination.
type ListRisksUseCase struct {
	riskRepo domain.RiskRepository
	mappings domain.RiskControlMappingRepository
	owners   OwnershipResolver
}

func NewListRisksUseCase(riskRepo domain.RiskRepository) *ListRisksUseCase {
	return &ListRisksUseCase{riskRepo: riskRepo}
}

// WithMappings attaches the control-mapping repository so the register can
// render the "Référentiel" column. Optional and nil-safe: without it the column
// is empty, which is honest — an empty cell says "not mapped", where the old
// fallback said "ISO 27001" on the strength of a tag.
func (uc *ListRisksUseCase) WithMappings(m domain.RiskControlMappingRepository) *ListRisksUseCase {
	uc.mappings = m
	return uc
}

// WithOwnership attaches the email resolver for the owner/assignee avatars.
func (uc *ListRisksUseCase) WithOwnership(r OwnershipResolver) *ListRisksUseCase {
	uc.owners = r
	return uc
}

// Execute lists risks for the organization with the given query parameters.
func (uc *ListRisksUseCase) Execute(ctx context.Context, orgID uuid.UUID, query domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
	query.Sanitize()

	result, err := uc.riskRepo.List(ctx, orgID, query)
	if err != nil {
		return nil, domain.NewInternalError("failed to list risks")
	}

	uc.enrich(ctx, orgID, result)
	return result, nil
}

// enrich fills the two computed blocks the register renders: the control
// mappings and the ownership emails. Both are batched — one query for the whole
// page, not one per row — and both degrade to empty rather than failing the
// list.
func (uc *ListRisksUseCase) enrich(ctx context.Context, orgID uuid.UUID, result *domain.PaginatedResult[domain.Risk]) {
	if result == nil || len(result.Data) == 0 {
		return
	}

	if uc.mappings != nil {
		ids := make([]uuid.UUID, 0, len(result.Data))
		for i := range result.Data {
			ids = append(ids, result.Data[i].ID)
		}
		if byRisk, err := uc.mappings.ListByRisks(ctx, orgID, ids); err == nil {
			for i := range result.Data {
				result.Data[i].ControlMappings = byRisk[result.Data[i].ID]
			}
		}
	}

	if uc.owners != nil {
		entities := make([]domain.OwnedEntity, 0, len(result.Data))
		for i := range result.Data {
			entities = append(entities, &result.Data[i])
		}
		uc.owners.ResolveEmails(ctx, entities...)
	}
}
