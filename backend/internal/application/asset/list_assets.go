// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ListAssetsUseCase lists every asset in a tenant's inventory.
type ListAssetsUseCase struct {
	repo domain.AssetRepository
}

func NewListAssetsUseCase(repo domain.AssetRepository) *ListAssetsUseCase {
	return &ListAssetsUseCase{repo: repo}
}

func (uc *ListAssetsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	return uc.Search(ctx, tenantID, ListFilter{})
}

// ListFilter narrows an inventory listing. Every field is optional; the zero
// value returns the whole inventory (so Execute stays the same call).
type ListFilter struct {
	// Category restricts to one typed category.
	Category domain.AssetCategory
	// Attributes are `attr.<key>=<value>` search terms, AND-ed together.
	Attributes []domain.AttributeSearchTerm
}

// Search lists a tenant's assets, filtered by category and/or typed attributes.
//
// Filtering happens in memory, deliberately: the inventory is loaded whole by
// every other caller in this module (the universe graph needs all nodes, the
// correlator needs all fingerprints), the tenant predicate is already applied by
// the repository, and a JSONB containment query per term would need its own
// index per key. If an inventory ever outgrows this, the fix is a GIN index on
// the attributes column and a repository-level filter — not a different call
// shape here.
func (uc *ListAssetsUseCase) Search(ctx context.Context, tenantID uuid.UUID, f ListFilter) ([]domain.Asset, error) {
	assets, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if f.Category == "" && len(f.Attributes) == 0 {
		return assets, nil
	}
	out := make([]domain.Asset, 0, len(assets))
	for _, a := range assets {
		if f.Category != "" && a.Category != f.Category {
			continue
		}
		if len(f.Attributes) > 0 && !domain.MatchesAttributes(a.Attributes, f.Attributes) {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}
