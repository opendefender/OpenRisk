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

// UpdateAssetInput carries partial updates — nil/empty fields are left unchanged.
type UpdateAssetInput struct {
	Name        *string
	Type        *string
	Criticality *domain.AssetCriticality
	Owner       *string
}

// UpdateAssetResult reports the updated asset plus whether criticality
// changed, so the caller (handler) can decide whether to publish
// events.AssetCriticalityChanged — that's an infra/transport concern, kept
// out of the use case per the existing risk module's convention (see
// RiskHandler.CreateRisk, which publishes after the use case returns).
type UpdateAssetResult struct {
	Asset              *domain.Asset
	CriticalityChanged bool
	OldCriticality     domain.AssetCriticality
	NewCriticality     domain.AssetCriticality
}

// UpdateAssetUseCase handles updating an existing asset. Before applying any
// change, it snapshots the asset's current state (ROADMAP.md M3 "historical
// snapshots") so the inventory's history view can show what it used to be.
type UpdateAssetUseCase struct {
	repo domain.AssetRepository
}

func NewUpdateAssetUseCase(repo domain.AssetRepository) *UpdateAssetUseCase {
	return &UpdateAssetUseCase{repo: repo}
}

// Execute updates an asset. changedBy is the ID of the user performing the
// update; it is recorded on the pre-change snapshot so the history answers
// "qui a modifié quoi, et quand" (uuid.Nil when the actor is unknown/system).
func (uc *UpdateAssetUseCase) Execute(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID, changedBy uuid.UUID, input UpdateAssetInput) (*UpdateAssetResult, error) {
	existing, err := uc.repo.GetByID(ctx, assetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if existing == nil {
		return nil, domain.NewNotFoundError("asset", assetID)
	}

	snapshot := &domain.AssetSnapshot{
		ID:          uuid.New(),
		TenantID:    tenantID,
		AssetID:     existing.ID,
		Name:        existing.Name,
		Type:        existing.Type,
		Criticality: existing.Criticality,
		Owner:       existing.Owner,
		Reason:      "update",
		ChangedBy:   changedBy,
	}
	if err := uc.repo.CreateSnapshot(ctx, snapshot); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	oldCriticality := existing.Criticality
	criticalityChanged := false

	if input.Name != nil {
		if *input.Name == "" {
			return nil, domain.NewValidationError("name cannot be empty")
		}
		existing.Name = *input.Name
	}
	if input.Type != nil {
		existing.Type = *input.Type
	}
	if input.Owner != nil {
		existing.Owner = *input.Owner
	}
	if input.Criticality != nil && *input.Criticality != oldCriticality {
		existing.Criticality = *input.Criticality
		criticalityChanged = true
	}

	if err := uc.repo.Update(ctx, existing); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	return &UpdateAssetResult{
		Asset:              existing,
		CriticalityChanged: criticalityChanged,
		OldCriticality:     oldCriticality,
		NewCriticality:     existing.Criticality,
	}, nil
}
