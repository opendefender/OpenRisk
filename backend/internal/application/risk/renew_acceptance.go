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

// RenewAcceptanceInput is the payload of POST /risks/:id/acceptance/renew.
type RenewAcceptanceInput = TransitionRiskStateInput

// RenewAcceptance extends a residual-risk acceptance that still stands (#811).
//
// Renewing is a new decision, so it needs a new approved governance request:
// an approval decided before the current acceptance began was spent on it and
// is refused. Nobody can move the end date any other way — the update
// endpoint does not accept the acceptance fields at all.
//
// It shares the transition use case's ports (approvals, tenant policy, trail)
// because it is the same decision taken again, not a different one.
func (uc *TransitionRiskStateUseCase) RenewAcceptance(ctx context.Context, tenantID, riskID uuid.UUID, in RenewAcceptanceInput) (*domain.Risk, error) {
	if len(in.Comment) > 1000 {
		return nil, domain.NewValidationError("comment must be 1000 characters or less")
	}
	r, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}
	if r.State() != domain.StateResidualAccepted {
		return nil, domain.NewValidationError(fmt.Sprintf(
			"only an accepted residual risk can be renewed (risk is %s)", r.State()))
	}

	// Same guard as the transition, and the same honest refusal when it
	// cannot be checked.
	if _, reason, err := uc.evaluate(ctx, tenantID, r, domain.StateResidualAccepted, in.Locale); err != nil {
		return nil, err
	} else if reason != "" {
		return nil, domain.NewValidationError(reason)
	}
	approval, _, err := uc.approvals.AcceptanceApproval(ctx, tenantID, r.ID, r.AcceptedAt)
	if err != nil {
		return nil, err
	}
	if approval == nil {
		return nil, domain.NewValidationError("no unspent governance approval for this renewal")
	}

	now := uc.clock()
	expiresAt, err := uc.resolveExpiry(ctx, tenantID, now, in.AcceptanceExpiresAt)
	if err != nil {
		return nil, err
	}
	before := r.AcceptanceSnapshot()
	r.BeginAcceptance(now, expiresAt, *approval)
	r.UpdatedAt = now

	if err := uc.riskRepo.Update(ctx, r); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to renew acceptance: %v", err))
	}

	newValue := r.AcceptanceSnapshot()
	newValue["comment"] = in.Comment
	entry := &domain.AuditLogEntry{
		ID:        uuid.New(),
		RiskID:    riskID,
		Timestamp: now,
		ChangedBy: in.Actor,
		Action:    "acceptance_renewed",
		OldValue:  before,
		NewValue:  newValue,
	}
	if err := uc.riskRepo.CreateAuditEntry(ctx, entry); err != nil {
		fmt.Printf("Warning: failed to audit acceptance renewal on risk %s: %v\n", riskID, err)
	}
	uc.journalAcceptance(ctx, tenantID, r, in.Actor, "residual risk acceptance renewed", before, in.Comment)
	return r, nil
}
