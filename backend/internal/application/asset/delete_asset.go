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

// DeleteAssetUseCase handles removing an asset from a tenant's inventory.
// A final snapshot is recorded before deletion (reason "delete") so the
// asset's last known state remains visible in its history even after removal.
type DeleteAssetUseCase struct {
	repo domain.AssetRepository
	// deps is optional. When set, deleting an asset also prunes every
	// dependency edge touching it, so the cartography never shows dangling
	// links to a removed asset. Wired via WithDependencyRepository so the
	// original 1-arg constructor (and its tests) keep working unchanged.
	deps domain.AssetDependencyRepository
}

func NewDeleteAssetUseCase(repo domain.AssetRepository) *DeleteAssetUseCase {
	return &DeleteAssetUseCase{repo: repo}
}

// WithDependencyRepository enables dependency-edge cascade on delete.
func (uc *DeleteAssetUseCase) WithDependencyRepository(deps domain.AssetDependencyRepository) *DeleteAssetUseCase {
	uc.deps = deps
	return uc
}

// Execute deletes an asset. changedBy is the ID of the user performing the
// deletion; it is recorded on the final snapshot so the asset's last known
// state remains attributable in its history (uuid.Nil when unknown/system).
func (uc *DeleteAssetUseCase) Execute(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID, changedBy uuid.UUID) error {
	existing, err := uc.repo.GetByID(ctx, assetID, tenantID)
	if err != nil {
		return domain.NewInternalError(err.Error())
	}
	if existing == nil {
		return domain.NewNotFoundError("asset", assetID)
	}

	snapshot := &domain.AssetSnapshot{
		ID:          uuid.New(),
		TenantID:    tenantID,
		AssetID:     existing.ID,
		Name:        existing.Name,
		Type:        existing.Type,
		Criticality: existing.Criticality,
		Owner:       existing.Owner,
		Reason:      "delete",
		ChangedBy:   changedBy,
	}
	if err := uc.repo.CreateSnapshot(ctx, snapshot); err != nil {
		return domain.NewInternalError(err.Error())
	}

	if uc.deps != nil {
		if err := uc.deps.DeleteByAsset(ctx, assetID, tenantID); err != nil {
			return domain.NewInternalError(err.Error())
		}
	}

	if err := uc.repo.Delete(ctx, assetID, tenantID); err != nil {
		return domain.NewInternalError(err.Error())
	}
	return nil
}
