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
	// Category selects the typed attribute schema. Optional: an asset created
	// without one is untyped and simply has no attributes (that is the state
	// every pre-existing asset is in).
	Category domain.AssetCategory
	// Attributes is the raw, unvalidated bag from the client. It is only ever
	// persisted after AttributeValidator has checked and coerced it.
	Attributes map[string]any
}

// AttributeValidator validates a raw attribute bag against the tenant's schema
// for a category and returns the coerced bag plus the schema that governed it.
// Narrow port, satisfied structurally by application/assetschema.Validator.
type AttributeValidator interface {
	ValidateFor(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory, in map[string]any) (domain.AssetAttributes, []domain.AttributeDef, error)
}

// ActivationRecorder notes the "connected an asset" milestone. Narrow port,
// satisfied structurally by application/activation.Recorder; nil-safe.
type ActivationRecorder interface {
	Record(ctx context.Context, tenantID uuid.UUID, key string, payload map[string]interface{})
}

// CreateAssetUseCase handles registering a new asset for a tenant.
type CreateAssetUseCase struct {
	repo       domain.AssetRepository
	activation ActivationRecorder
	attrs      AttributeValidator
}

func NewCreateAssetUseCase(repo domain.AssetRepository) *CreateAssetUseCase {
	return &CreateAssetUseCase{repo: repo}
}

// WithActivation attaches the optional activation recorder.
func (uc *CreateAssetUseCase) WithActivation(rec ActivationRecorder) *CreateAssetUseCase {
	uc.activation = rec
	return uc
}

// WithAttributeValidator wires typed-attribute validation. Without it, an asset
// may still be created — but only without attributes: see Execute.
func (uc *CreateAssetUseCase) WithAttributeValidator(v AttributeValidator) *CreateAssetUseCase {
	uc.attrs = v
	return uc
}

func (uc *CreateAssetUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input CreateAssetInput) (*domain.Asset, error) {
	if input.Name == "" {
		return nil, domain.NewValidationError("name is required")
	}

	criticality := input.Criticality
	if criticality == "" {
		criticality = domain.CriticalityMedium
	}

	// Typed attributes: a category is required to have any, because the schema
	// is what makes them typed. Attributes without a category are rejected
	// rather than stored loose — an unvalidated bag is the untyped inventory
	// this feature exists to replace.
	var (
		attrs    domain.AssetAttributes
		defs     []domain.AttributeDef
		category domain.AssetCategory
	)
	if input.Category != "" {
		parsed, err := domain.ParseAssetCategory(string(input.Category))
		if err != nil {
			return nil, err
		}
		category = parsed
		attrs, defs, err = uc.validateAttributes(ctx, tenantID, category, input.Attributes)
		if err != nil {
			return nil, err
		}
	} else if len(input.Attributes) > 0 {
		return nil, domain.NewValidationError("attributes require an asset category — pick one so the values can be validated against its schema")
	}

	assetEntity := &domain.Asset{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        input.Name,
		Type:        input.Type,
		Criticality: criticality,
		Owner:       input.Owner,
		Source:      "MANUAL",
		Category:    category,
		Attributes:  attrs,
	}
	assetEntity.RefreshFingerprints(defs)

	if err := uc.repo.Create(ctx, assetEntity); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	if uc.activation != nil {
		uc.activation.Record(ctx, tenantID, string(domain.ActivationAssetConnected), map[string]interface{}{
			"asset_id": assetEntity.ID.String(),
			"type":     assetEntity.Type,
		})
	}
	return assetEntity, nil
}
