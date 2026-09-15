// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// GetVendorChainUseCase reads the vendor→asset→risk chain, one hop deep
// (#669, ADR 0004 D2).
//
// Every hop is resolved through its own tenant-scoped read rather than trusted
// because the previous hop was: the vendor by (id, tenant, category), the edges
// by tenant, the assets by tenant, and the risks through their parent risk's
// tenant. An edge naming an asset the tenant cannot read is dropped, never
// rendered with a blank asset.
type GetVendorChainUseCase struct {
	vendors domain.VendorRepository
	deps    domain.AssetDependencyRepository
}

func NewGetVendorChainUseCase(vendors domain.VendorRepository, deps domain.AssetDependencyRepository) *GetVendorChainUseCase {
	return &GetVendorChainUseCase{vendors: vendors, deps: deps}
}

func (uc *GetVendorChainUseCase) Execute(ctx context.Context, tenantID, vendorID uuid.UUID) (*domain.VendorChain, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}

	vendor, err := uc.vendors.GetVendor(ctx, vendorID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if vendor == nil {
		return nil, domain.NewNotFoundError("vendor", vendorID)
	}

	edges, err := uc.deps.ListByAsset(ctx, vendorID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	type hop struct {
		dep     domain.AssetDependency
		assetID uuid.UUID
	}
	var hops []hop
	seen := map[uuid.UUID]struct{}{}
	var assetIDs []uuid.UUID
	for _, e := range edges {
		if e.TenantID != tenantID {
			continue
		}
		assetID, ok := domain.VendorLinkedAsset(e, vendorID)
		if !ok {
			continue
		}
		hops = append(hops, hop{dep: e, assetID: assetID})
		if _, dup := seen[assetID]; !dup {
			seen[assetID] = struct{}{}
			assetIDs = append(assetIDs, assetID)
		}
	}

	chain := &domain.VendorChain{
		VendorID:   vendor.ID,
		VendorName: vendor.Name,
		Links:      []domain.VendorChainLink{},
	}
	if len(hops) == 0 {
		return chain, nil
	}

	assets, err := uc.vendors.AssetsByIDs(ctx, tenantID, assetIDs)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	byID := make(map[uuid.UUID]domain.Asset, len(assets))
	for _, a := range assets {
		if a.TenantID == tenantID {
			byID[a.ID] = a
		}
	}

	risks, err := uc.vendors.RisksByAssetIDs(ctx, tenantID, assetIDs)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	for _, h := range hops {
		a, ok := byID[h.assetID]
		if !ok {
			continue
		}
		linkRisks := risks[h.assetID]
		if linkRisks == nil {
			linkRisks = []domain.VendorChainRisk{}
		}
		chain.Links = append(chain.Links, domain.VendorChainLink{
			LinkID: h.dep.ID,
			Verb:   h.dep.Type,
			Asset: domain.VendorChainAsset{
				ID:          a.ID,
				Name:        a.Name,
				Type:        a.Type,
				Category:    a.Category,
				Criticality: a.Criticality,
			},
			Risks: linkRisks,
		})
	}

	sort.SliceStable(chain.Links, func(i, j int) bool {
		if chain.Links[i].Asset.Name != chain.Links[j].Asset.Name {
			return chain.Links[i].Asset.Name < chain.Links[j].Asset.Name
		}
		return chain.Links[i].Verb < chain.Links[j].Verb
	})
	return chain, nil
}
