// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// TransitionPhaseInput moves a risk to a new ISO 31000 lifecycle phase.
type TransitionPhaseInput struct {
	Phase domain.RiskPhase // target phase
	Note  string           // optional rationale, stored in the audit trail
}

// TransitionPhaseUseCase advances (or steps back) a risk through the managed
// lifecycle "Identifier → Analyser → Évaluer → Traiter → Surveiller →
// Clôturer". It is tenant-scoped, validates the transition, keeps the coarse
// RiskStatus loosely in sync, and records an audit entry.
type TransitionPhaseUseCase struct {
	riskRepo domain.RiskRepository
}

func NewTransitionPhaseUseCase(riskRepo domain.RiskRepository) *TransitionPhaseUseCase {
	return &TransitionPhaseUseCase{riskRepo: riskRepo}
}

// Execute validates and applies a phase transition for a risk owned by tenantID.
// Cross-tenant access resolves to ErrNotFound (never 403) per the ABSOLUTE rules.
func (uc *TransitionPhaseUseCase) Execute(ctx context.Context, tenantID uuid.UUID, riskID uuid.UUID, input TransitionPhaseInput, actorID uuid.UUID) (*domain.Risk, error) {
	// 1. Validate the target phase up front (typed validation error).
	target, err := domain.ParseRiskPhase(string(input.Phase))
	if err != nil {
		return nil, err
	}
	if len(input.Note) > 1000 {
		return nil, domain.NewValidationError("note must be 1000 characters or less")
	}

	// 2. Fetch existing risk (tenant-scoped).
	r, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// A risk created before this feature may have an empty phase; treat it as
	// "identified" so the first transition is always valid.
	current := r.LifecyclePhase
	if current == "" {
		current = domain.PhaseIdentified
	}

	// 3. Guard the transition.
	if !current.CanTransitionTo(target) {
		return nil, domain.NewValidationError(fmt.Sprintf("invalid lifecycle transition: %s → %s", current, target))
	}

	// 4. Apply. Keep the coarse RiskStatus loosely coupled so the register's
	// status pill and filters stay coherent with the lifecycle stage.
	oldPhase := current
	oldStatus := r.Status
	r.LifecyclePhase = target
	switch target {
	case domain.PhaseTreated:
		// Entering treatment: reflect active work unless already resolved.
		if r.Status == domain.RiskOpen || r.Status == domain.StatusDraft || r.Status == "" {
			r.Status = domain.RiskInProgress
		}
	case domain.PhaseClosed:
		r.Status = domain.RiskClosed
	default:
		// Re-opening a closed risk to an earlier phase → back to "open".
		if oldPhase == domain.PhaseClosed && r.Status == domain.RiskClosed {
			r.Status = domain.RiskOpen
		}
	}
	r.UpdatedAt = time.Now()

	// 5. Persist (repository enforces tenant_id in the WHERE clause).
	if err := uc.riskRepo.Update(ctx, r); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to transition risk phase: %v", err))
	}

	// 6. Audit trail (best-effort — the risk is already updated).
	entry := &domain.AuditLogEntry{
		ID:        uuid.New(),
		RiskID:    riskID,
		Timestamp: time.Now(),
		ChangedBy: actorID,
		Action:    "phase_transition",
		OldValue:  map[string]interface{}{"lifecycle_phase": oldPhase, "status": oldStatus},
		NewValue:  map[string]interface{}{"lifecycle_phase": target, "status": r.Status, "note": input.Note},
	}
	if err := uc.riskRepo.CreateAuditEntry(ctx, entry); err != nil {
		fmt.Printf("Warning: failed to create audit entry for phase transition on risk %s: %v\n", riskID, err)
	}

	return r, nil
}
