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
