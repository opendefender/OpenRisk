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
	"github.com/opendefender/openrisk/pkg/crq"
)

// FinancialEntitlement answers whether a tenant's plan includes financial
// quantification, the gate on every ALE figure.
type FinancialEntitlement interface {
	HasFinancialQuantification(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

// GetRiskSummaryUseCase backs GET /risks/summary (#913).
type GetRiskSummaryUseCase struct {
	repo        domain.RiskSummaryRepository
	quantifier  *crq.Quantifier
	entitlement FinancialEntitlement
}

// NewGetRiskSummaryUseCase builds the use case. A nil entitlement means the
// deployment has no plan gate (the route-level gates behave the same way); a nil
// quantifier cannot produce an ALE, so the figure stays null.
func NewGetRiskSummaryUseCase(repo domain.RiskSummaryRepository, q *crq.Quantifier, ent FinancialEntitlement) *GetRiskSummaryUseCase {
	return &GetRiskSummaryUseCase{repo: repo, quantifier: q, entitlement: ent}
}

// Execute computes the summary for the tenant. tenantID == uuid.Nil is
// unauthenticated (401): the fail-closed answer, never "all tenants".
//
// ale_xaf is null unless the plan has financial quantification; when the
// entitlement cannot be read it is null too, because a figure shown by mistake
// cannot be taken back.
func (uc *GetRiskSummaryUseCase) Execute(ctx context.Context, tenantID, userID uuid.UUID, view domain.RiskView, search string) (*domain.RiskSummary, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant required")
	}
	if view == "" {
		view = domain.RiskViewAll
	}

	views, err := uc.repo.ViewCounts(ctx, tenantID, userID)
	if err != nil {
		return nil, domain.NewInternalError("failed to count risk views")
	}
	bands, err := uc.repo.CriticalityCounts(ctx, tenantID, userID, view, search)
	if err != nil {
		return nil, domain.NewInternalError("failed to count risks by criticality")
	}

	summary := &domain.RiskSummary{Views: views, ByCriticality: bands}

	if uc.canQuantify(ctx, tenantID) {
		inputs, err := uc.repo.ActiveExposureInputs(ctx, tenantID)
		if err != nil {
			return nil, domain.NewInternalError("failed to read active risk exposure")
		}
		var total float64
		for _, in := range inputs {
			ale, _ := uc.quantifier.ALEXAF(in.SLEXAF, in.ARO, string(in.Criticality))
			total += ale
		}
		total = math.Round(total*100) / 100
		summary.Active = domain.RiskActiveSummary{Count: int64(len(inputs)), ALEXAF: &total}
		return summary, nil
	}

	n, err := uc.repo.CountActive(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("failed to count active risks")
	}
	summary.Active = domain.RiskActiveSummary{Count: n}
	return summary, nil
}

func (uc *GetRiskSummaryUseCase) canQuantify(ctx context.Context, tenantID uuid.UUID) bool {
	if uc.quantifier == nil {
		return false
	}
	if uc.entitlement == nil {
		return true
	}
	ok, err := uc.entitlement.HasFinancialQuantification(ctx, tenantID)
	return err == nil && ok
}
