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

// DefaultTopologyNodeLimit caps a single topology response.
//
// 2 000 is the figure the view is required to stay smooth at, so it is also the
// point past which returning more helps nobody: beyond it the answer stops
// being a picture. The response says when the cap bit (AssetTopology.Truncated)
// rather than quietly shipping a partial estate.
const DefaultTopologyNodeLimit = 2000

// VulnCounter reports open vulnerabilities per asset for a tenant. Narrow,
// OPTIONAL port — the topology is about dependencies, and a missing
// vulnerability register degrades one badge rather than the whole view.
type VulnCounter interface {
	CountOpenByAsset(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID]int, error)
}

// GetTopologyUseCase assembles the tenant's asset topology.
type GetTopologyUseCase struct {
	assets domain.AssetRepository
	deps   domain.AssetDependencyRepository
	vulns  VulnCounter // optional
}

func NewGetTopologyUseCase(assets domain.AssetRepository, deps domain.AssetDependencyRepository) *GetTopologyUseCase {
	return &GetTopologyUseCase{assets: assets, deps: deps}
}

// WithVulnCounter adds open-vulnerability counts to the nodes. Optional.
func (uc *GetTopologyUseCase) WithVulnCounter(v VulnCounter) *GetTopologyUseCase {
	uc.vulns = v
	return uc
}

// Execute returns the whole graph for a tenant, capped at nodeLimit (0 → the
// default cap).
func (uc *GetTopologyUseCase) Execute(ctx context.Context, tenantID uuid.UUID, nodeLimit int) (*domain.AssetTopology, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	if nodeLimit <= 0 || nodeLimit > DefaultTopologyNodeLimit {
		nodeLimit = DefaultTopologyNodeLimit
	}

	assets, err := uc.assets.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("failed to load assets: " + err.Error())
	}
	deps, err := uc.deps.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("failed to load dependencies: " + err.Error())
	}

	// Optional slice: a failure here costs the vulnerability badge, not the graph.
	var vulnCounts map[uuid.UUID]int
	if uc.vulns != nil {
		if counts, err := uc.vulns.CountOpenByAsset(ctx, tenantID); err == nil {
			vulnCounts = counts
		}
	}

	topo := domain.BuildTopology(assets, deps, vulnCounts, nodeLimit)
	return &topo, nil
}

// GetCompromiseChainUseCase answers "this asset is compromised — what else?".
type GetCompromiseChainUseCase struct {
	assets domain.AssetRepository
	deps   domain.AssetDependencyRepository
}

func NewGetCompromiseChainUseCase(assets domain.AssetRepository, deps domain.AssetDependencyRepository) *GetCompromiseChainUseCase {
	return &GetCompromiseChainUseCase{assets: assets, deps: deps}
}

// Execute walks the dependency graph from an asset in both directions.
//
// The origin is verified to belong to the tenant FIRST: without that check the
// endpoint would confirm whether an arbitrary asset id exists elsewhere, and
// would happily walk a graph the caller has no business seeing.
func (uc *GetCompromiseChainUseCase) Execute(ctx context.Context, tenantID, assetID uuid.UUID) (*domain.CompromiseChain, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	origin, err := uc.assets.GetByID(ctx, assetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("failed to load asset: " + err.Error())
	}
	if origin == nil {
		return nil, domain.NewNotFoundError("asset", assetID)
	}

	deps, err := uc.deps.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("failed to load dependencies: " + err.Error())
	}
	chain := domain.BuildCompromiseChain(assetID, deps)
	return &chain, nil
}
