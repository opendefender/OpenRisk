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

// GetAssetUseCase retrieves a single asset scoped to a tenant.
type GetAssetUseCase struct {
	repo domain.AssetRepository
}

func NewGetAssetUseCase(repo domain.AssetRepository) *GetAssetUseCase {
	return &GetAssetUseCase{repo: repo}
}

func (uc *GetAssetUseCase) Execute(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID) (*domain.Asset, error) {
	assetEntity, err := uc.repo.GetByID(ctx, assetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if assetEntity == nil {
		return nil, domain.NewNotFoundError("asset", assetID)
	}
	return assetEntity, nil
}
