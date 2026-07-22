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

// CreateAssetDependencyInput describes a new directed edge Source → Target.
type CreateAssetDependencyInput struct {
	SourceAssetID uuid.UUID
	TargetAssetID uuid.UUID
	Type          domain.DependencyType
	Description   string
}

// CreateAssetDependencyUseCase adds an edge to a tenant's asset dependency
// graph. It guarantees the graph stays sane and tenant-isolated:
//   - both endpoints must exist AND belong to the caller's tenant,
//   - an asset cannot depend on itself,
//   - the same (source, target, type) edge cannot be created twice.
type CreateAssetDependencyUseCase struct {
	deps   domain.AssetDependencyRepository
	assets domain.AssetRepository
}

func NewCreateAssetDependencyUseCase(deps domain.AssetDependencyRepository, assets domain.AssetRepository) *CreateAssetDependencyUseCase {
	return &CreateAssetDependencyUseCase{deps: deps, assets: assets}
}

func (uc *CreateAssetDependencyUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input CreateAssetDependencyInput) (*domain.AssetDependency, error) {
	if input.SourceAssetID == uuid.Nil || input.TargetAssetID == uuid.Nil {
		return nil, domain.NewValidationError("source_asset_id and target_asset_id are required")
	}
	if input.SourceAssetID == input.TargetAssetID {
		return nil, domain.NewValidationError("an asset cannot depend on itself")
	}

	depType := input.Type
	if depType == "" {
		depType = domain.DepDependsOn
	}
	if !depType.IsValid() {
		return nil, domain.NewValidationError("invalid dependency type")
	}

	// Both endpoints must exist within this tenant. GetByID is already
	// tenant-scoped and returns (nil, nil) for foreign/absent assets, so this
	// doubles as the cross-tenant guard.
	source, err := uc.assets.GetByID(ctx, input.SourceAssetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if source == nil {
		return nil, domain.NewNotFoundError("source asset", input.SourceAssetID)
	}
	target, err := uc.assets.GetByID(ctx, input.TargetAssetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if target == nil {
		return nil, domain.NewNotFoundError("target asset", input.TargetAssetID)
	}

	exists, err := uc.deps.Exists(ctx, tenantID, input.SourceAssetID, input.TargetAssetID, depType)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if exists {
		return nil, domain.NewConflictError("asset dependency", "source+target+type")
	}

	dep := &domain.AssetDependency{
		ID:            uuid.New(),
		TenantID:      tenantID,
		SourceAssetID: input.SourceAssetID,
		TargetAssetID: input.TargetAssetID,
		Type:          depType,
		Description:   input.Description,
	}
	if err := uc.deps.Create(ctx, dep); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return dep, nil
}
