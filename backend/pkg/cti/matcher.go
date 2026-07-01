// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package cti

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Matcher performs matching between vulnerabilities and assets.
type Matcher struct {
	repo        Repository
	riskCreator RiskCreator
}

func NewMatcher(repo Repository, rc RiskCreator) *Matcher {
	return &Matcher{repo: repo, riskCreator: rc}
}

// MatchCPEsToAssets finds vulnerabilities overlapping the provided CPE list
// and requests risk creation (idempotent) via RiskCreator for each match.
func (m *Matcher) MatchCPEsToAssets(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID, cpes []string) error {
	if len(cpes) == 0 {
		return nil
	}

	vulns, err := m.repo.FindByCPEOverlap(ctx, cpes)
	if err != nil {
		return fmt.Errorf("failed to find vulns by cpe: %w", err)
	}

	for _, v := range vulns {
		// Delegate risk creation to RiskCreator (which must be idempotent)
		if err := m.riskCreator.CreateRiskFromCVE(ctx, tenantID, assetID, v); err != nil {
			// Log and continue
			// In production, use structured logger; return error only for fatal problems
			continue
		}
	}

	return nil
}
