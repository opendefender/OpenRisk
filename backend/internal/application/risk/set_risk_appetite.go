// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// RiskAppetiteWriter stores the tenant's risk appetite. It returns a
// domain NotFound error when the organization does not exist.
type RiskAppetiteWriter interface {
	SetOrganizationRiskAppetite(ctx context.Context, tenantID uuid.UUID, appetiteXAF float64) error
}

// maxRiskAppetiteXAF bounds the input to something a balance sheet can hold
// (10^15 XAF); beyond it the number is a typo, not a policy.
const maxRiskAppetiteXAF = 1e15

// SetRiskAppetiteUseCase records the annual loss the board accepts, the
// threshold drawn on the loss-exceedance curve (#904). The amount is in XAF,
// the canonical store; the page converts from the display currency.
type SetRiskAppetiteUseCase struct {
	writer RiskAppetiteWriter
}

// NewSetRiskAppetiteUseCase builds the use case.
func NewSetRiskAppetiteUseCase(w RiskAppetiteWriter) *SetRiskAppetiteUseCase {
	return &SetRiskAppetiteUseCase{writer: w}
}

// Execute validates and stores the appetite for the caller's tenant.
func (uc *SetRiskAppetiteUseCase) Execute(ctx context.Context, tenantID uuid.UUID, appetiteXAF float64) (float64, error) {
	if tenantID == uuid.Nil {
		return 0, domain.NewForbiddenError("no organization in the session")
	}
	if math.IsNaN(appetiteXAF) || math.IsInf(appetiteXAF, 0) || appetiteXAF <= 0 || appetiteXAF > maxRiskAppetiteXAF {
		return 0, domain.NewValidationError("risk appetite must be a positive amount")
	}
	v := math.Round(appetiteXAF)
	if err := uc.writer.SetOrganizationRiskAppetite(ctx, tenantID, v); err != nil {
		return 0, err
	}
	return v, nil
}
