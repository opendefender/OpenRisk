// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

type topoAssets struct {
	domain.AssetRepository
	byTenant map[uuid.UUID][]domain.Asset
	asked    uuid.UUID
}

func (f *topoAssets) List(_ context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	f.asked = tenantID
	return f.byTenant[tenantID], nil
}

type topoDeps struct {
	domain.AssetDependencyRepository
	byTenant map[uuid.UUID][]domain.AssetDependency
}

func (f *topoDeps) ListByTenant(_ context.Context, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	return f.byTenant[tenantID], nil
}

func topoWorld() (uuid.UUID, *topoAssets, *topoDeps, domain.Asset, domain.Asset) {
	tenant := uuid.New()
	fw := domain.Asset{ID: uuid.New(), TenantID: tenant, Name: "Pare-feu", Criticality: "HIGH",
		Attributes: domain.AssetAttributes{"internet_exposed": true}}
	db := domain.Asset{ID: uuid.New(), TenantID: tenant, Name: "Base clients", Criticality: "CRITICAL",
		Attributes: domain.AssetAttributes{}}
	assets := &topoAssets{byTenant: map[uuid.UUID][]domain.Asset{tenant: {fw, db}}}
	deps := &topoDeps{byTenant: map[uuid.UUID][]domain.AssetDependency{tenant: {
		{ID: uuid.New(), TenantID: tenant, SourceAssetID: fw.ID, TargetAssetID: db.ID, Type: "connects_to"},
	}}}
	return tenant, assets, deps, fw, db
}

func TestGetTopology_Success(t *testing.T) {
	tenant, assets, deps, fw, db := topoWorld()
	topo, err := NewGetTopologyUseCase(assets, deps).Execute(context.Background(), tenant, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.Nodes) != 2 || len(topo.Edges) != 1 {
		t.Fatalf("graph = %d nodes, %d edges", len(topo.Nodes), len(topo.Edges))
	}
	if len(topo.ExposurePaths) != 1 || topo.ExposurePaths[0].Hops != 2 ||
		topo.ExposurePaths[0].AssetIDs[0] != fw.ID || topo.ExposurePaths[0].AssetIDs[1] != db.ID {
		t.Fatalf("exposure paths = %+v", topo.ExposurePaths)
	}
}

// A tenant with nothing mapped gets an empty graph and no path, not an error.
func TestGetTopology_NotFound(t *testing.T) {
	_, assets, deps, _, _ := topoWorld()
	topo, err := NewGetTopologyUseCase(assets, deps).Execute(context.Background(), uuid.New(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.Nodes) != 0 || len(topo.ExposurePaths) != 0 {
		t.Fatalf("another tenant saw %d nodes, %d paths", len(topo.Nodes), len(topo.ExposurePaths))
	}
}

func TestGetTopology_Unauthorized(t *testing.T) {
	_, assets, deps, _, _ := topoWorld()
	_, err := NewGetTopologyUseCase(assets, deps).Execute(context.Background(), uuid.Nil, 0)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a session without a tenant must be refused, got %v", err)
	}
	if assets.asked != uuid.Nil {
		t.Fatal("assets were listed without a tenant")
	}
}
