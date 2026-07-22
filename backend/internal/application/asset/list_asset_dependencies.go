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

// ListAssetDependenciesUseCase returns the tenant's full dependency graph
// (every directed edge). The front-end builds the cartography by pairing this
// with the asset list.
type ListAssetDependenciesUseCase struct {
	deps domain.AssetDependencyRepository
}

func NewListAssetDependenciesUseCase(deps domain.AssetDependencyRepository) *ListAssetDependenciesUseCase {
	return &ListAssetDependenciesUseCase{deps: deps}
}

func (uc *ListAssetDependenciesUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	list, err := uc.deps.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return list, nil
}
