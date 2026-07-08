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

// ListAssetsUseCase lists every asset in a tenant's inventory.
type ListAssetsUseCase struct {
	repo domain.AssetRepository
}

func NewListAssetsUseCase(repo domain.AssetRepository) *ListAssetsUseCase {
	return &ListAssetsUseCase{repo: repo}
}

func (uc *ListAssetsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	assets, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return assets, nil
}
