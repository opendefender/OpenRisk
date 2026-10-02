// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgscoring "github.com/opendefender/openrisk/pkg/scoring"
)

// RiskAssetStore resolves the assets a risk is linked to and writes the link
// together with the risk, so the score stored on the row is always the one its
// linked assets produce.
type RiskAssetStore interface {
	// FindByIDs returns the tenant's assets among ids. Ids of another tenant's
	// assets, or of no asset at all, are absent from the result.
	FindByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*domain.Asset, error)

	// SaveWithAssets creates (create=true) or saves the risk and replaces its
	// asset links with assets, in one transaction.
	SaveWithAssets(ctx context.Context, risk *domain.Risk, assets []*domain.Asset, create bool) error
}

// applyScore stores on the risk the score the Score Engine computes for its
// current terms — probability, impact and the criticality of risk.Assets — and
// the band that goes with it. It is the only way create and update write a
// score, so a fresh row matches pkg/scoring before the worker ever runs (#792).
func applyScore(engine pkgscoring.Engine, r *domain.Risk) error {
	ac := domain.RiskAssetCriticality(domain.AssetCriticalities(r.Assets))
	b, err := engine.Breakdown(r.Probability, r.Impact, ac, nil)
	if err != nil {
		return domain.NewInternalError(fmt.Sprintf("score engine rejected the risk terms: %v", err))
	}
	r.Score = b.Score
	r.Criticality = domain.CriticalityLevel(b.Criticality)
	return nil
}
