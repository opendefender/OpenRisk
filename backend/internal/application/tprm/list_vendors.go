// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package tprm holds the third-party risk management use cases (ADR 0004).
//
// It is named tprm and not vendor on purpose: Go's ./... pattern skips every
// directory named vendor, so a package there would silently drop out of
// `go test ./...` and `go vet ./...`.
package tprm

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

const (
	// DefaultVendorPageSize is the register page size when none is asked for.
	DefaultVendorPageSize = 50
	// MaxVendorPageSize caps a page, whatever the caller asks for.
	MaxVendorPageSize = 200
)

// ListVendorsUseCase returns the vendor register (#669, ADR 0004 D1).
//
// Filtering and pagination run in memory over ONE tenant-scoped read, the way
// the asset inventory applies its category and attribute filters: service
// criticality lives in the JSONB attribute bag, and a vendor register holds
// hundreds of rows, not the tens of thousands that would justify a JSONB index.
type ListVendorsUseCase struct {
	vendors     domain.VendorRepository
	deps        domain.AssetDependencyRepository
	assessments domain.VendorLatestAssessmentReader
}

func NewListVendorsUseCase(vendors domain.VendorRepository, deps domain.AssetDependencyRepository) *ListVendorsUseCase {
	return &ListVendorsUseCase{vendors: vendors, deps: deps}
}

// WithLatestAssessments wires the assessment module's reader (#670). Until it
// is wired, every entry reports latest_assessment: null — absent, not invented.
func (uc *ListVendorsUseCase) WithLatestAssessments(r domain.VendorLatestAssessmentReader) *ListVendorsUseCase {
	uc.assessments = r
	return uc
}

func (uc *ListVendorsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, f domain.VendorFilter) (*domain.VendorPage, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}

	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = DefaultVendorPageSize
	}
	if limit > MaxVendorPageSize {
		limit = MaxVendorPageSize
	}
	if offset < 0 {
		offset = 0
	}

	all, err := uc.vendors.ListVendors(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	search := strings.ToLower(strings.TrimSpace(f.Search))
	matched := make([]domain.Asset, 0, len(all))
	for _, v := range all {
		// Belt and braces over the repository's own category predicate: a row
		// that is not a vendor never reaches the register.
		if v.Category != domain.CategoryVendor || v.TenantID != tenantID {
			continue
		}
		if f.ServiceCriticality != "" &&
			domain.AssetAttrString(v.Attributes, "service_criticality") != f.ServiceCriticality {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(v.Name), search) &&
			!strings.Contains(strings.ToLower(domain.AssetAttrString(v.Attributes, "legal_name")), search) {
			continue
		}
		matched = append(matched, v)
	}

	page := &domain.VendorPage{
		Items:  []domain.VendorRegisterEntry{},
		Total:  len(matched),
		Limit:  limit,
		Offset: offset,
	}
	if offset >= len(matched) {
		return page, nil
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	window := matched[offset:end]

	ids := make([]uuid.UUID, 0, len(window))
	for _, v := range window {
		ids = append(ids, v.ID)
	}

	counts, err := uc.linkedAssetCounts(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}

	var latest map[uuid.UUID]domain.VendorLatestAssessment
	if uc.assessments != nil {
		latest, err = uc.assessments.LatestByVendor(ctx, tenantID, ids)
		if err != nil {
			return nil, domain.NewInternalError(err.Error())
		}
	}

	for _, v := range window {
		entry := domain.NewVendorRegisterEntry(v)
		entry.LinkedAssets = counts[v.ID]
		if a, ok := latest[v.ID]; ok {
			a := a
			entry.LatestAssessment = &a
		}
		page.Items = append(page.Items, entry)
	}
	return page, nil
}

// linkedAssetCounts counts DISTINCT assets linked to each vendor. An asset both
// managed by and sharing data with the same vendor is one linked asset, not two.
// The edges come from the tenant-scoped ListByTenant, so a foreign edge cannot
// inflate a count.
func (uc *ListVendorsUseCase) linkedAssetCounts(ctx context.Context, tenantID uuid.UUID, vendorIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	want := make(map[uuid.UUID]struct{}, len(vendorIDs))
	for _, id := range vendorIDs {
		want[id] = struct{}{}
	}

	edges, err := uc.deps.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	linked := make(map[uuid.UUID]map[uuid.UUID]struct{}, len(vendorIDs))
	record := func(vendorID uuid.UUID, e domain.AssetDependency) {
		if _, ok := want[vendorID]; !ok {
			return
		}
		assetID, ok := domain.VendorLinkedAsset(e, vendorID)
		if !ok {
			return
		}
		if linked[vendorID] == nil {
			linked[vendorID] = map[uuid.UUID]struct{}{}
		}
		linked[vendorID][assetID] = struct{}{}
	}
	for _, e := range edges {
		if e.TenantID != tenantID {
			continue
		}
		record(e.TargetAssetID, e)
		record(e.SourceAssetID, e)
	}

	counts := make(map[uuid.UUID]int, len(linked))
	for vendorID, assets := range linked {
		counts[vendorID] = len(assets)
	}
	return counts, nil
}
