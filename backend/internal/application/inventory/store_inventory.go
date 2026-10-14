// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package inventory receives the software inventory the OpenRisk agent reports
// for its host, and has it checked for known vulnerabilities (#910, spec
// docs/910_AGENT_PACKAGE_VULN_SCAN.md).
package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/vulnscan"
)

// AssetLister reads the tenant's assets, to attach the host to one.
type AssetLister interface {
	List(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error)
}

// Matcher checks a stored inventory for known vulnerabilities. Optional: the
// inventory is stored either way.
type Matcher interface {
	MatchInventory(ctx context.Context, tenantID, inventoryID uuid.UUID) error
}

// HostReport is the host as the agent describes it.
type HostReport struct {
	Hostname  string `json:"hostname"`
	Platform  string `json:"platform"`
	OS        string `json:"os"`
	OSVersion string `json:"os_version"`
}

// StoreInput is one inventory push. The agent is resolved by the agent auth
// (bearer + HMAC) and is never read from the body.
type StoreInput struct {
	Agent     *domain.ScannerAgent
	JobID     string
	Host      HostReport
	Packages  []domain.HostPackageInput
	Collector string
	Errors    []string
}

// StoreInventoryUseCase stores the snapshot, attaches it to an asset when one
// matches unambiguously, and hands it to the matcher.
type StoreInventoryUseCase struct {
	repo    domain.HostInventoryRepository
	assets  AssetLister
	matcher Matcher
	now     func() time.Time
	// async runs the match off the request; tests make it synchronous.
	async func(func())
}

// NewStoreInventoryUseCase builds the use case.
func NewStoreInventoryUseCase(r domain.HostInventoryRepository, a AssetLister) *StoreInventoryUseCase {
	return &StoreInventoryUseCase{repo: r, assets: a, now: time.Now, async: func(f func()) { go f() }}
}

// WithMatcher wires the vulnerability match that follows every inventory.
func (uc *StoreInventoryUseCase) WithMatcher(m Matcher) *StoreInventoryUseCase {
	uc.matcher = m
	return uc
}

// matchTimeout bounds a background match: NVD's limits make Windows hosts slow.
const matchTimeout = 30 * time.Minute

// Execute stores the inventory and returns it.
func (uc *StoreInventoryUseCase) Execute(ctx context.Context, in StoreInput) (*domain.HostInventory, error) {
	if in.Agent == nil || in.Agent.TenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("unauthenticated agent")
	}
	tenantID := in.Agent.TenantID

	// The agent's own registered hostname stands in when osquery could not
	// read one (osquery_missing).
	hostname := strings.TrimSpace(in.Host.Hostname)
	if hostname == "" {
		hostname = in.Agent.Hostname
	}
	key := domain.HostKey(hostname)
	if key == "" {
		return nil, domain.NewValidationError("the inventory names no host")
	}
	if len(in.Packages) > domain.MaxInventoryPackages {
		return nil, domain.NewValidationError(fmt.Sprintf("too many packages: %d (max %d)", len(in.Packages), domain.MaxInventoryPackages))
	}
	pkgs := domain.NormalisePackages(in.Packages)

	inv := &domain.HostInventory{
		TenantID:     tenantID,
		AgentID:      &in.Agent.ID,
		HostKey:      key,
		Hostname:     capped(hostname, 255),
		Platform:     capped(strings.ToLower(in.Host.Platform), 32),
		OS:           capped(in.Host.OS, 128),
		OSVersion:    capped(in.Host.OSVersion, 128),
		Ecosystem:    domain.OSVEcosystem(in.Host.Platform, in.Host.OSVersion, dominantSource(pkgs)),
		PackageCount: len(pkgs),
		CollectedAt:  uc.now(),
	}
	if hasError(in.Errors, domain.InventoryErrNoOsquery) {
		inv.MatchError = domain.InventoryErrNoOsquery
	}

	// Attach to an asset with the same rule the vulnerability ingest applies:
	// only an unambiguous, confident match. Otherwise the host stays unattached
	// and its findings land in the unassigned queue.
	if assets, err := uc.assets.List(ctx, tenantID); err == nil {
		corr := vulnapp.NewCorrelator(assets).Resolve(vulnscan.NormalizedFinding{
			AssetName: hostname,
			Raw:       map[string]any{"hostname": hostname},
		}, nil)
		if corr.Asset != nil {
			id := corr.Asset.ID
			inv.AssetID = &id
		}
	}

	if err := uc.repo.Replace(ctx, inv, pkgs); err != nil {
		return nil, domain.NewInternalError("could not store the inventory: " + err.Error())
	}

	if uc.matcher != nil && inv.MatchError == "" && len(pkgs) > 0 {
		id := inv.ID
		uc.async(func() {
			mctx, cancel := context.WithTimeout(context.Background(), matchTimeout)
			defer cancel()
			_ = uc.matcher.MatchInventory(mctx, tenantID, id)
		})
	}
	return inv, nil
}

// dominantSource is the package source most packages come from, which decides
// the host's OSV ecosystem.
func dominantSource(pkgs []domain.HostPackageInput) string {
	counts := map[string]int{}
	best, n := "", 0
	for _, p := range pkgs {
		counts[p.Source]++
		if counts[p.Source] > n {
			best, n = p.Source, counts[p.Source]
		}
	}
	return best
}

func hasError(errs []string, code string) bool {
	for _, e := range errs {
		if e == code {
			return true
		}
	}
	return false
}

func capped(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
