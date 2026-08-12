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
	// Category re-types the asset. Changing it re-validates the attribute bag
	// against the NEW schema — an asset moved from Server to Vendor cannot keep
	// its firmware version.
	Category *domain.AssetCategory
	// Attributes, when non-nil, REPLACES the whole bag (it is validated as a
	// whole, so a partial merge would let a required attribute be dropped
	// without the validator ever seeing its absence). nil leaves it untouched.
	Attributes map[string]any
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
	repo  domain.AssetRepository
	attrs AttributeValidator
}

func NewUpdateAssetUseCase(repo domain.AssetRepository) *UpdateAssetUseCase {
	return &UpdateAssetUseCase{repo: repo}
}

// WithAttributeValidator wires typed-attribute validation.
func (uc *UpdateAssetUseCase) WithAttributeValidator(v AttributeValidator) *UpdateAssetUseCase {
	uc.attrs = v
	return uc
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

	// Re-type and/or re-validate the attribute bag. Both a category change and
	// an attribute edit go through the SAME validation, against whichever schema
	// the asset ends up governed by.
	if input.Category != nil {
		parsed, err := domain.ParseAssetCategory(string(*input.Category))
		if err != nil {
			return nil, err
		}
		existing.Category = parsed
	}
	if input.Attributes != nil || input.Category != nil {
		if existing.Category == "" {
			if len(input.Attributes) > 0 {
				return nil, domain.NewValidationError("attributes require an asset category — pick one so the values can be validated against its schema")
			}
		} else {
			raw := input.Attributes
			if raw == nil {
				// Category changed but no new values were sent: re-validate what
				// the asset already carries against the new schema, so the caller
				// is told which values no longer belong instead of silently
				// keeping attributes that the new category never declared.
				raw = map[string]any(existing.Attributes)
			}
			attrs, defs, err := uc.validateAttributes(ctx, tenantID, existing.Category, raw)
			if err != nil {
				return nil, err
			}
			existing.Attributes = attrs
			existing.RefreshFingerprints(defs)
		}
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
