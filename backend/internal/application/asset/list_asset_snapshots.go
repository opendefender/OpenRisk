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

// ListAssetSnapshotsUseCase retrieves the history of an asset (ROADMAP.md M3
// "historical snapshots"): every prior state recorded before an update or
// deletion, newest first.
type ListAssetSnapshotsUseCase struct {
	repo domain.AssetRepository
}

func NewListAssetSnapshotsUseCase(repo domain.AssetRepository) *ListAssetSnapshotsUseCase {
	return &ListAssetSnapshotsUseCase{repo: repo}
}

func (uc *ListAssetSnapshotsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID) ([]domain.AssetSnapshot, error) {
	// Confirm the asset exists (and belongs to this tenant) before returning
	// history — otherwise an empty history for a nonexistent/foreign asset
	// ID would look identical to "no history yet", leaking existence info.
	existing, err := uc.repo.GetByID(ctx, assetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if existing == nil {
		return nil, domain.NewNotFoundError("asset", assetID)
	}

	history, err := uc.repo.ListSnapshots(ctx, assetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return history, nil
}
