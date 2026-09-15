// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"

	"github.com/google/uuid"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
)

// LinkVendorAssetInput names the asset to link and how the vendor supplies it.
type LinkVendorAssetInput struct {
	AssetID     uuid.UUID
	Verb        domain.DependencyType
	Description string
}

// LinkVendorAssetUseCase links an asset to a vendor (#669, ADR 0004 D2).
//
// A vendor link IS an asset dependency edge. This use case adds only what makes
// an edge a vendor link — a creatable verb, a vendor at the verb's end, and a
// non-vendor at the other — and hands the edge to CreateAssetDependencyUseCase,
// so the tenant, self-reference and duplicate guards are the ones already
// tested on /asset-dependencies rather than a second copy of them.
type LinkVendorAssetUseCase struct {
	vendors domain.VendorRepository
	assets  domain.AssetRepository
	create  *assetuc.CreateAssetDependencyUseCase
}

func NewLinkVendorAssetUseCase(
	vendors domain.VendorRepository,
	assets domain.AssetRepository,
	create *assetuc.CreateAssetDependencyUseCase,
) *LinkVendorAssetUseCase {
	return &LinkVendorAssetUseCase{vendors: vendors, assets: assets, create: create}
}

func (uc *LinkVendorAssetUseCase) Execute(ctx context.Context, tenantID, vendorID uuid.UUID, in LinkVendorAssetInput) (*domain.AssetDependency, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	if !domain.IsCreatableVendorLinkVerb(in.Verb) {
		return nil, domain.NewValidationError("verb must be one of managed_by, hosted_by, processes_data_of, depends_on")
	}
	if in.AssetID == uuid.Nil {
		return nil, domain.NewValidationError("asset_id is required")
	}

	vendor, err := uc.vendors.GetVendor(ctx, vendorID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if vendor == nil {
		return nil, domain.NewNotFoundError("vendor", vendorID)
	}

	asset, err := uc.assets.GetByID(ctx, in.AssetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if asset == nil {
		return nil, domain.NewNotFoundError("asset", in.AssetID)
	}
	if asset.Category == domain.CategoryVendor {
		// A vendor of a vendor is a fourth party. Concentration and fourth-party
		// risk are out of scope for TPRM v1 (ADR 0004 D8, #376); accepting the
		// edge here would let the register count a vendor as a supplied asset.
		return nil, domain.NewValidationError("a vendor cannot be linked to another vendor: fourth-party links are out of scope (#376)")
	}

	end, _ := domain.VendorLinkEndFor(in.Verb)
	source, target := asset.ID, vendor.ID
	if end == domain.VendorAtSource {
		source, target = vendor.ID, asset.ID
	}

	return uc.create.Execute(ctx, tenantID, assetuc.CreateAssetDependencyInput{
		SourceAssetID: source,
		TargetAssetID: target,
		Type:          in.Verb,
		Description:   in.Description,
	})
}
