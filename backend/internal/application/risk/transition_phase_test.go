// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// --- TransitionPhase Tests ---

func TestTransitionPhase_Success(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{
		ID: riskID, OrganizationID: orgID, TenantID: orgID,
		Title: "Exposed S3 bucket", Status: domain.RiskOpen,
		LifecyclePhase: domain.PhaseIdentified,
	}
	var saved *domain.Risk
	audited := false

	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			if id == riskID && tid == orgID {
				return existing, nil
			}
			return nil, nil
		},
		updateFunc: func(ctx context.Context, r *domain.Risk) error { saved = r; return nil },
		createAuditEntryFunc: func(ctx context.Context, e *domain.AuditLogEntry) error {
			audited = true
			if e.Action != "phase_transition" {
				t.Errorf("expected action phase_transition, got %q", e.Action)
			}
			return nil
		},
	}

	uc := NewTransitionPhaseUseCase(repo)
	r, err := uc.Execute(context.Background(), orgID, riskID, TransitionPhaseInput{Phase: domain.PhaseAnalyzed, Note: "prob/impact assessed"}, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.LifecyclePhase != domain.PhaseAnalyzed {
		t.Errorf("expected phase analyzed, got %s", r.LifecyclePhase)
	}
	if saved == nil || saved.LifecyclePhase != domain.PhaseAnalyzed {
		t.Error("expected the risk to be persisted with the new phase")
	}
	if !audited {
		t.Error("expected an audit entry to be created")
	}
}

func TestTransitionPhase_ClosingSetsStatusClosed(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{
		ID: riskID, OrganizationID: orgID, TenantID: orgID,
		Status: domain.RiskInProgress, LifecyclePhase: domain.PhaseMonitored,
	}
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			return existing, nil
		},
		updateFunc: func(ctx context.Context, r *domain.Risk) error { return nil },
	}
	uc := NewTransitionPhaseUseCase(repo)
	r, err := uc.Execute(context.Background(), orgID, riskID, TransitionPhaseInput{Phase: domain.PhaseClosed}, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.LifecyclePhase != domain.PhaseClosed {
		t.Errorf("expected phase closed, got %s", r.LifecyclePhase)
	}
	if r.Status != domain.RiskClosed {
		t.Errorf("expected status closed to follow phase closed, got %s", r.Status)
	}
}

func TestTransitionPhase_NotFound(t *testing.T) {
	repo := &MockRiskRepository{} // getByID returns (nil, nil)
	uc := NewTransitionPhaseUseCase(repo)
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), TransitionPhaseInput{Phase: domain.PhaseAnalyzed}, uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTransitionPhase_WrongTenant(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()
	riskID := uuid.New()
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			if id == riskID && tid == orgA {
				return &domain.Risk{ID: riskID, OrganizationID: orgA, LifecyclePhase: domain.PhaseIdentified}, nil
			}
			return nil, nil // orgB sees nothing
		},
	}
	uc := NewTransitionPhaseUseCase(repo)
	_, err := uc.Execute(context.Background(), orgB, riskID, TransitionPhaseInput{Phase: domain.PhaseAnalyzed}, uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for cross-tenant transition, got %v", err)
	}
}

func TestTransitionPhase_InvalidTransition(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{ID: riskID, OrganizationID: orgID, LifecyclePhase: domain.PhaseIdentified}
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			return existing, nil
		},
		updateFunc: func(ctx context.Context, r *domain.Risk) error {
			t.Fatal("Update must not be called on an invalid transition")
			return nil
		},
	}
	uc := NewTransitionPhaseUseCase(repo)
	// identified → monitored is a two-step jump (not allowed; only ±1 or →closed).
	_, err := uc.Execute(context.Background(), orgID, riskID, TransitionPhaseInput{Phase: domain.PhaseMonitored}, uuid.New())
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation for invalid transition, got %v", err)
	}
}

func TestTransitionPhase_InvalidPhaseValue(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewTransitionPhaseUseCase(repo)
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), TransitionPhaseInput{Phase: domain.RiskPhase("banana")}, uuid.New())
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation for unknown phase, got %v", err)
	}
}

func TestTransitionPhase_NoOpRejected(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{ID: riskID, OrganizationID: orgID, LifecyclePhase: domain.PhaseAnalyzed}
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			return existing, nil
		},
	}
	uc := NewTransitionPhaseUseCase(repo)
	_, err := uc.Execute(context.Background(), orgID, riskID, TransitionPhaseInput{Phase: domain.PhaseAnalyzed}, uuid.New())
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation for no-op transition, got %v", err)
	}
}
