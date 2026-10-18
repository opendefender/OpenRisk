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
)

// AcceptancePolicyStore reads and writes the tenant's maximum acceptance
// length. Satisfied by the organization repository.
type AcceptancePolicyStore interface {
	AcceptancePolicy
	SetMaxAcceptanceMonths(ctx context.Context, tenantID uuid.UUID, months int) error
}

// AcceptancePolicyView is the body of GET/PUT /risk-acceptance/policy.
type AcceptancePolicyView struct {
	MaxAcceptanceMonths int `json:"max_acceptance_months"`
	DefaultMonths       int `json:"default_months"`
	CeilingMonths       int `json:"ceiling_months"`
}

// AcceptancePolicyUseCase reads and sets how long a residual-risk acceptance
// may run in this tenant (#811). The default is 12 months; the tenant may
// lower or raise the maximum within 1–36.
type AcceptancePolicyUseCase struct {
	store AcceptancePolicyStore
}

func NewAcceptancePolicyUseCase(store AcceptancePolicyStore) *AcceptancePolicyUseCase {
	return &AcceptancePolicyUseCase{store: store}
}

// Get returns the tenant's policy.
func (uc *AcceptancePolicyUseCase) Get(ctx context.Context, tenantID uuid.UUID) (*AcceptancePolicyView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("no organization in the session")
	}
	m, err := uc.store.MaxAcceptanceMonths(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return policyView(m), nil
}

// Set stores a new maximum. It applies to acceptances given from now on;
// acceptances already running keep the end date they were approved with.
func (uc *AcceptancePolicyUseCase) Set(ctx context.Context, tenantID uuid.UUID, months int) (*AcceptancePolicyView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("no organization in the session")
	}
	if months < 1 || months > domain.AcceptanceMonthsCeiling {
		return nil, domain.NewValidationError(fmt.Sprintf(
			"max_acceptance_months must be between 1 and %d", domain.AcceptanceMonthsCeiling))
	}
	if err := uc.store.SetMaxAcceptanceMonths(ctx, tenantID, months); err != nil {
		return nil, err
	}
	return policyView(months), nil
}

func policyView(months int) *AcceptancePolicyView {
	return &AcceptancePolicyView{
		MaxAcceptanceMonths: domain.NormalizeAcceptanceMax(months),
		DefaultMonths:       domain.DefaultAcceptanceMonths,
		CeilingMonths:       domain.AcceptanceMonthsCeiling,
	}
}
