// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cti

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// CPEMatcher matches a single asset's CPE list against known vulnerabilities.
// It is a narrower, per-asset primitive distinct from the Matcher port (which
// sweeps ALL tenant assets and is consumed by SyncWorker after each NVD sync).
// A future Matcher implementation is expected to enumerate tenants/assets and
// delegate to CPEMatcher.MatchCPEsToAssets per asset.
type CPEMatcher struct {
	repo        Repository
	riskCreator RiskCreator
}

func NewCPEMatcher(repo Repository, rc RiskCreator) *CPEMatcher {
	return &CPEMatcher{repo: repo, riskCreator: rc}
}

// MatchCPEsToAssets finds vulnerabilities overlapping the provided CPE list
// and requests a risk proposal (idempotent) via RiskCreator for each match.
// RiskCreator MUST NOT persist a risk silently — see RiskCreator doc (Rule 11).
func (m *CPEMatcher) MatchCPEsToAssets(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID, cpes []string) error {
	if len(cpes) == 0 {
		return nil
	}

	vulns, err := m.repo.MatchByAssetCPEs(ctx, tenantID, assetID, cpes)
	if err != nil {
		return fmt.Errorf("failed to match cpes for asset: %w", err)
	}

	for _, v := range vulns {
		// Delegate risk proposal to RiskCreator (which must be idempotent)
		if _, err := m.riskCreator.ProposeRiskFromCVE(ctx, tenantID, assetID, v); err != nil {
			// Log and continue
			// In production, use structured logger; return error only for fatal problems
			continue
		}
	}

	return nil
}
