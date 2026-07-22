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
	assets, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return assets, nil
}
