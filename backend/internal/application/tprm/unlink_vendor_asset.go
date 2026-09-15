// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// UnlinkVendorAssetUseCase removes a vendor link (#669, ADR 0004 D2).
//
// The link is addressed under its vendor (/vendors/:id/assets/:linkId), so both
// ids are checked: the vendor must be this tenant's vendor, and the edge must be
// a vendor link OF THAT VENDOR. Any other edge — another vendor's link, a plain
// asset dependency, a foreign edge — answers the same not-found, so this route
// cannot be used to delete an arbitrary edge of the dependency graph.
type UnlinkVendorAssetUseCase struct {
	vendors domain.VendorRepository
	deps    domain.AssetDependencyRepository
}

func NewUnlinkVendorAssetUseCase(vendors domain.VendorRepository, deps domain.AssetDependencyRepository) *UnlinkVendorAssetUseCase {
	return &UnlinkVendorAssetUseCase{vendors: vendors, deps: deps}
}

func (uc *UnlinkVendorAssetUseCase) Execute(ctx context.Context, tenantID, vendorID, linkID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.NewForbiddenError("missing tenant context")
	}

	vendor, err := uc.vendors.GetVendor(ctx, vendorID, tenantID)
	if err != nil {
		return domain.NewInternalError(err.Error())
	}
	if vendor == nil {
		return domain.NewNotFoundError("vendor", vendorID)
	}

	dep, err := uc.deps.GetByID(ctx, linkID, tenantID)
	if err != nil {
		return domain.NewInternalError(err.Error())
	}
	if dep == nil || dep.TenantID != tenantID {
		return domain.NewNotFoundError("vendor link", linkID)
	}
	if _, ok := domain.VendorLinkedAsset(*dep, vendor.ID); !ok {
		return domain.NewNotFoundError("vendor link", linkID)
	}

	if err := uc.deps.Delete(ctx, linkID, tenantID); err != nil {
		return domain.NewInternalError(err.Error())
	}
	return nil
}
