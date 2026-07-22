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

// CreateAssetInput is the input for registering a new asset in a tenant's inventory.
type CreateAssetInput struct {
	Name        string
	Type        string
	Criticality domain.AssetCriticality
	Owner       string
}

// CreateAssetUseCase handles registering a new asset for a tenant.
type CreateAssetUseCase struct {
	repo domain.AssetRepository
}

func NewCreateAssetUseCase(repo domain.AssetRepository) *CreateAssetUseCase {
	return &CreateAssetUseCase{repo: repo}
}

func (uc *CreateAssetUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input CreateAssetInput) (*domain.Asset, error) {
	if input.Name == "" {
		return nil, domain.NewValidationError("name is required")
	}

	criticality := input.Criticality
	if criticality == "" {
		criticality = domain.CriticalityMedium
	}

	assetEntity := &domain.Asset{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        input.Name,
		Type:        input.Type,
		Criticality: criticality,
		Owner:       input.Owner,
		Source:      "MANUAL",
	}

	if err := uc.repo.Create(ctx, assetEntity); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return assetEntity, nil
}
