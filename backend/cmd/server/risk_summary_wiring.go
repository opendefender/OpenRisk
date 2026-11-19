// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"

	"github.com/google/uuid"

	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// featureLevelReader is the one method of the entitlement service the summary
// needs.
type featureLevelReader interface {
	Level(ctx context.Context, tenant uuid.UUID, f ent.Feature) (ent.Level, ent.Plan, error)
}

// riskFinancialEntitlement answers the risk summary's "does this tenant get the
// ALE figure". The register shows the closed-form ALE, which is the "basic"
// depth of financial quantification (#613): the same depth GET /risks/:id/financial
// admits. A resolver error is returned, and the use case treats it as "no".
type riskFinancialEntitlement struct{ levels featureLevelReader }

func (e riskFinancialEntitlement) HasFinancialQuantification(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	level, _, err := e.levels.Level(ctx, tenantID, ent.FeatFinancialQuant)
	if err != nil {
		return false, err
	}
	return level.AtLeast(ent.LevelBasic), nil
}
