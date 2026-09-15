// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// store is an in-memory, TENANT-ENFORCING fake of the asset, vendor and
// dependency repositories. It applies the tenant exactly where the GORM
// repositories do, so a use case that forgot to pass its tenant — or passed the
// wrong one — gets the same empty answer it would get from the database.
type store struct {
	assets map[uuid.UUID]domain.Asset
	edges  []domain.AssetDependency
	// risks is keyed by asset id. Each risk carries the tenant of its PARENT
	// RISK, which is the only gate risk_assets has (ADR 0004 D2).
	risks   map[uuid.UUID][]tenantRisk
	deleted []uuid.UUID
}

type tenantRisk struct {
	tenantID uuid.UUID
	risk     domain.VendorChainRisk
}

func newStore() *store {
	return &store{assets: map[uuid.UUID]domain.Asset{}, risks: map[uuid.UUID][]tenantRisk{}}
}

func (s *store) addAsset(tenantID uuid.UUID, name string, cat domain.AssetCategory, attrs domain.AssetAttributes) domain.Asset {
	a := domain.Asset{ID: uuid.New(), TenantID: tenantID, Name: name, Category: cat, Type: string(cat), Attributes: attrs, Criticality: domain.CriticalityMedium}
	s.assets[a.ID] = a
	return a
}

func (s *store) addEdge(tenantID, source, target uuid.UUID, t domain.DependencyType) domain.AssetDependency {
	e := domain.AssetDependency{ID: uuid.New(), TenantID: tenantID, SourceAssetID: source, TargetAssetID: target, Type: t}
	s.edges = append(s.edges, e)
	return e
}

func (s *store) addRisk(riskTenant, assetID uuid.UUID, title string, score float64) domain.VendorChainRisk {
	r := domain.VendorChainRisk{ID: uuid.New(), Title: title, Score: score, Criticality: "high", Status: "open"}
	s.risks[assetID] = append(s.risks[assetID], tenantRisk{tenantID: riskTenant, risk: r})
	return r
}

// --- domain.VendorRepository -------------------------------------------------

func (s *store) ListVendors(_ context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	var out []domain.Asset
	for _, a := range s.assets {
		if a.TenantID == tenantID && a.Category == domain.CategoryVendor {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *store) GetVendor(_ context.Context, id, tenantID uuid.UUID) (*domain.Asset, error) {
	a, ok := s.assets[id]
	if !ok || a.TenantID != tenantID || a.Category != domain.CategoryVendor {
		return nil, nil
	}
	return &a, nil
}

func (s *store) AssetsByIDs(_ context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]domain.Asset, error) {
	var out []domain.Asset
	for _, id := range ids {
		if a, ok := s.assets[id]; ok && a.TenantID == tenantID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *store) RisksByAssetIDs(_ context.Context, tenantID uuid.UUID, assetIDs []uuid.UUID) (map[uuid.UUID][]domain.VendorChainRisk, error) {
	out := map[uuid.UUID][]domain.VendorChainRisk{}
	for _, id := range assetIDs {
		for _, tr := range s.risks[id] {
			if tr.tenantID == tenantID {
				out[id] = append(out[id], tr.risk)
			}
		}
	}
	return out, nil
}

// --- domain.AssetRepository (the parts the use cases reach) ------------------

func (s *store) Create(_ context.Context, a *domain.Asset) error { s.assets[a.ID] = *a; return nil }

func (s *store) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.Asset, error) {
	a, ok := s.assets[id]
	if !ok || a.TenantID != tenantID {
		return nil, nil
	}
	return &a, nil
}

func (s *store) List(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	var out []domain.Asset
	for _, a := range s.assets {
		if a.TenantID == tenantID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *store) Update(_ context.Context, a *domain.Asset) error { s.assets[a.ID] = *a; return nil }
func (s *store) Delete(_ context.Context, id, _ uuid.UUID) error { delete(s.assets, id); return nil }
func (s *store) CreateSnapshot(context.Context, *domain.AssetSnapshot) error {
	return nil
}
func (s *store) ListSnapshots(context.Context, uuid.UUID, uuid.UUID) ([]domain.AssetSnapshot, error) {
	return nil, nil
}

// deps adapts the store to domain.AssetDependencyRepository. It is a separate
// type because AssetRepository and AssetDependencyRepository both declare
// Create, GetByID and Delete with different signatures.
type deps struct{ s *store }

func (d deps) Create(_ context.Context, dep *domain.AssetDependency) error {
	d.s.edges = append(d.s.edges, *dep)
	return nil
}

func (d deps) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.AssetDependency, error) {
	for _, e := range d.s.edges {
		if e.ID == id && e.TenantID == tenantID {
			e := e
			return &e, nil
		}
	}
	return nil, nil
}

func (d deps) ListByTenant(_ context.Context, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	var out []domain.AssetDependency
	for _, e := range d.s.edges {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (d deps) ListByAsset(_ context.Context, assetID, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	var out []domain.AssetDependency
	for _, e := range d.s.edges {
		if e.TenantID == tenantID && (e.SourceAssetID == assetID || e.TargetAssetID == assetID) {
			out = append(out, e)
		}
	}
	return out, nil
}

func (d deps) Exists(_ context.Context, tenantID, source, target uuid.UUID, t domain.DependencyType) (bool, error) {
	for _, e := range d.s.edges {
		if e.TenantID == tenantID && e.SourceAssetID == source && e.TargetAssetID == target && e.Type == t {
			return true, nil
		}
	}
	return false, nil
}

func (d deps) Delete(_ context.Context, id, tenantID uuid.UUID) error {
	kept := d.s.edges[:0]
	for _, e := range d.s.edges {
		if e.ID == id && e.TenantID == tenantID {
			d.s.deleted = append(d.s.deleted, id)
			continue
		}
		kept = append(kept, e)
	}
	d.s.edges = kept
	return nil
}

func (d deps) DeleteByAsset(context.Context, uuid.UUID, uuid.UUID) error { return nil }

// latestReader is a fake VendorLatestAssessmentReader that honours the tenant.
type latestReader struct {
	tenantID uuid.UUID
	byVendor map[uuid.UUID]domain.VendorLatestAssessment
}

func (l latestReader) LatestByVendor(_ context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]domain.VendorLatestAssessment, error) {
	out := map[uuid.UUID]domain.VendorLatestAssessment{}
	if tenantID != l.tenantID {
		return out, nil
	}
	for _, id := range ids {
		if a, ok := l.byVendor[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

var _ = time.Time{}
