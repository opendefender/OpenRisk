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

// DeleteAssetDependencyUseCase removes an edge from a tenant's dependency graph.
type DeleteAssetDependencyUseCase struct {
	deps domain.AssetDependencyRepository
}

func NewDeleteAssetDependencyUseCase(deps domain.AssetDependencyRepository) *DeleteAssetDependencyUseCase {
	return &DeleteAssetDependencyUseCase{deps: deps}
}

func (uc *DeleteAssetDependencyUseCase) Execute(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	existing, err := uc.deps.GetByID(ctx, id, tenantID)
	if err != nil {
		return domain.NewInternalError(err.Error())
	}
	if existing == nil {
		return domain.NewNotFoundError("asset dependency", id)
	}
	if err := uc.deps.Delete(ctx, id, tenantID); err != nil {
		return domain.NewInternalError(err.Error())
	}
	return nil
}
