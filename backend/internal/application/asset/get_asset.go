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
