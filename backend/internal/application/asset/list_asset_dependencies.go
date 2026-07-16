// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
