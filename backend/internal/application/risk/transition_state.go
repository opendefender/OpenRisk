// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// MitigationSnapshot is what the FSM needs to know about a risk's treatment
// plan, and nothing more. Assembled by the adapter so this package never
// imports the mitigation module.
type MitigationSnapshot struct {
	ID    uuid.UUID
	Ref   string // human reference shown in the blocking reason, e.g. "MIT-14"
	Title string
	// Active means the plan counts as real work: planned, in progress or in
	// review — anything but cancelled.
	Active bool
	// SubActions counts on this plan.
	TotalSubActions     int
	CompletedSubActions int
}

// RemainingSubActions is what the stepper quotes back to the user.
func (m MitigationSnapshot) RemainingSubActions() int {
	if m.TotalSubActions <= m.CompletedSubActions {
		return 0
	}
	return m.TotalSubActions - m.CompletedSubActions
}

// MitigationInspector answers the two mitigation guards. Optional port: with no
// inspector wired, those guards cannot be evaluated and the use case says so
// rather than silently letting the transition through (see evaluate).
type MitigationInspector interface {
	// SnapshotForRisk returns every mitigation plan of a risk, tenant-scoped.
	SnapshotForRisk(ctx context.Context, tenantID, riskID uuid.UUID) ([]MitigationSnapshot, error)
}

// ApprovalChecker answers the residual-acceptance guard: is there a VALIDATED
// governance approval authorising the acceptance of this risk's residual risk?
// Optional port.
type ApprovalChecker interface {
	// HasApprovedAcceptance reports whether an approval request for this risk
	// (entity_type "risk_acceptance") has been approved, plus the id of the
	// pending one if any, so the UI can link straight to it.
	HasApprovedAcceptance(ctx context.Context, tenantID, riskID uuid.UUID) (approved bool, pendingRequestID *uuid.UUID, err error)
}

// TransitionRiskStateInput is the payload of POST /risks/:id/transition.
type TransitionRiskStateInput struct {
	To      domain.RiskState
	Comment string
	Actor   uuid.UUID
	Locale  string
}

// TransitionRiskStateUseCase moves a risk through the single canonical
// lifecycle, ENFORCING the guards server-side.
//
// The guards are the point of this use case. They were previously "enforced" by
// a frontend that offered the buttons it felt like offering, which is how a risk
// came to be MITIGATED with two sub-actions still open.
type TransitionRiskStateUseCase struct {
	riskRepo    domain.RiskRepository
	mitigations MitigationInspector
	approvals   ApprovalChecker
}

func NewTransitionRiskStateUseCase(riskRepo domain.RiskRepository) *TransitionRiskStateUseCase {
	return &TransitionRiskStateUseCase{riskRepo: riskRepo}
}

// WithMitigations attaches the inspector backing the treatment guards. Nil-safe.
func (uc *TransitionRiskStateUseCase) WithMitigations(m MitigationInspector) *TransitionRiskStateUseCase {
	uc.mitigations = m
	return uc
}

// WithApprovals attaches the checker backing the residual-acceptance guard.
func (uc *TransitionRiskStateUseCase) WithApprovals(a ApprovalChecker) *TransitionRiskStateUseCase {
	uc.approvals = a
	return uc
}

// AvailableTransitions answers GET /risks/:id/transitions: every reachable
// state, whether it is allowed right now, and what is blocking it otherwise.
//
// This is the contract the <LifecycleStepper> renders. It returns blocked
// options rather than hiding them — a user who cannot see the next step has no
// way to learn what to do about it.
func (uc *TransitionRiskStateUseCase) AvailableTransitions(ctx context.Context, tenantID, riskID uuid.UUID, locale string) (*TransitionsView, error) {
	r, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	current := r.State()
	view := &TransitionsView{
		Current:      current,
		CurrentLabel: current.Label(locale),
		StepIndex:    current.StepIndex(),
		StepCount:    domain.StepCount(),
		Options:      make([]domain.TransitionOption, 0, 4),
	}

	for _, target := range current.NextStates() {
		opt := domain.TransitionOption{
			To:        target,
			Label:     target.Label(locale),
			Allowed:   true,
			IsForward: target.StepIndex() > current.StepIndex(),
		}
		if guard, reason, err := uc.evaluate(ctx, tenantID, r, target, locale); err != nil {
			return nil, err
		} else if reason != "" {
			opt.Allowed = false
			opt.Reason = reason
			opt.Guard = guard
		}
		view.Options = append(view.Options, opt)
		if opt.Allowed && opt.IsForward && view.Next == "" {
			view.Next = target
			view.NextLabel = opt.Label
		}
	}
	// The natural next step, even when blocked, so the stepper can show it greyed
	// out with its reason instead of showing nothing.
	if view.Next == "" {
		for _, opt := range view.Options {
			if opt.IsForward {
				view.Next = opt.To
				view.NextLabel = opt.Label
				view.BlockedReason = opt.Reason
				break
			}
		}
	}
	return view, nil
}

// TransitionsView is the response body of GET /risks/:id/transitions.
type TransitionsView struct {
	Current      domain.RiskState `json:"current"`
	CurrentLabel string           `json:"current_label"`
	// Next is the natural forward step (blocked or not), so the stepper always
	// has something to point at.
	Next          domain.RiskState          `json:"next,omitempty"`
	NextLabel     string                    `json:"next_label,omitempty"`
	BlockedReason string                    `json:"blocked_reason,omitempty"`
	StepIndex     int                       `json:"step_index"`
	StepCount     int                       `json:"step_count"`
	Options       []domain.TransitionOption `json:"options"`
}

// Execute validates and applies a transition. Tenant-scoped; a risk owned by
// another tenant reads back as not found, never as forbidden.
func (uc *TransitionRiskStateUseCase) Execute(ctx context.Context, tenantID, riskID uuid.UUID, in TransitionRiskStateInput) (*domain.Risk, error) {
	target, err := domain.ParseRiskState(string(in.To))
	if err != nil {
		return nil, err
	}
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

	current := r.State()
	if current == target {
		return nil, domain.NewValidationError(fmt.Sprintf("risk is already %s", target))
	}
	if !current.CanTransitionTo(target) {
		return nil, domain.NewValidationError(fmt.Sprintf(
			"invalid lifecycle transition: %s → %s", current, target))
	}

	// The guards. A blocked transition is a VALIDATION error carrying the
	// concrete blocker, not a bare 403 — the caller must be able to show the
	// user what to fix.
	if _, reason, err := uc.evaluate(ctx, tenantID, r, target, in.Locale); err != nil {
		return nil, err
	} else if reason != "" {
		return nil, domain.NewValidationError(reason)
	}

	r.SetState(target)
	if target == domain.StateMitigated || target == domain.StateResidualAccepted {
		now := time.Now()
		r.LastMitigatedAt = &now
	}
	r.UpdatedAt = time.Now()

	if err := uc.riskRepo.Update(ctx, r); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to transition risk: %v", err))
	}

	// Audit trail (best-effort — the risk is already updated).
	entry := &domain.AuditLogEntry{
		ID:        uuid.New(),
		RiskID:    riskID,
		Timestamp: time.Now(),
		ChangedBy: in.Actor,
		Action:    "lifecycle_transition",
		OldValue:  map[string]interface{}{"lifecycle_state": current},
		NewValue: map[string]interface{}{
			"lifecycle_state": target,
			"status":          r.Status,
			"lifecycle_phase": r.LifecyclePhase,
			"comment":         in.Comment,
		},
	}
	if err := uc.riskRepo.CreateAuditEntry(ctx, entry); err != nil {
		fmt.Printf("Warning: failed to audit lifecycle transition on risk %s: %v\n", riskID, err)
	}

	return r, nil
}

// evaluate runs the guards a target state imposes. It returns ("", "") when the
// transition is allowed, and (guard, reason) when it is not.
//
// A guard whose port is NOT wired blocks rather than passes. An unverifiable
// precondition is not a satisfied precondition, and in a security tool the
// honest failure is the safe one — the reason says so plainly.
func (uc *TransitionRiskStateUseCase) evaluate(ctx context.Context, tenantID uuid.UUID, r *domain.Risk, target domain.RiskState, locale string) (domain.TransitionGuard, string, error) {
	en := strings.HasPrefix(strings.ToLower(locale), "en")

	for _, guard := range domain.GuardsFor(target) {
		switch guard {

		case domain.GuardActiveMitigation:
			if uc.mitigations == nil {
				return guard, unverifiable(en, "mitigations"), nil
			}
			plans, err := uc.mitigations.SnapshotForRisk(ctx, tenantID, r.ID)
			if err != nil {
				return "", "", err
			}
			if countActive(plans) == 0 {
				if en {
					return guard, "This risk has no active mitigation. Create one before starting treatment.", nil
				}
				return guard, "Ce risque n'a aucune mitigation active. Créez-en une avant de démarrer le traitement.", nil
			}

		case domain.GuardSubActionsComplete:
			if uc.mitigations == nil {
				return guard, unverifiable(en, "mitigations"), nil
			}
			plans, err := uc.mitigations.SnapshotForRisk(ctx, tenantID, r.ID)
			if err != nil {
				return "", "", err
			}
			if countActive(plans) == 0 {
				if en {
					return guard, "This risk has no active mitigation, so there is nothing to complete.", nil
				}
				return guard, "Ce risque n'a aucune mitigation active : il n'y a rien à terminer.", nil
			}
			if reason := incompleteReason(plans, en); reason != "" {
				return guard, reason, nil
			}

		case domain.GuardGovernanceApproval:
			if uc.approvals == nil {
				return guard, unverifiable(en, "approvals"), nil
			}
			approved, pending, err := uc.approvals.HasApprovedAcceptance(ctx, tenantID, r.ID)
			if err != nil {
				return "", "", err
			}
			if !approved {
				if pending != nil {
					if en {
						return guard, fmt.Sprintf("Approval request %s is still pending. Residual risk can only be accepted once Governance has validated it.", shortID(*pending)), nil
					}
					return guard, fmt.Sprintf("La demande d'approbation %s est encore en attente. Le risque résiduel ne peut être accepté qu'une fois validé par la Gouvernance.", shortID(*pending)), nil
				}
				if en {
					return guard, "Accepting residual risk requires a validated Governance approval. Submit one first.", nil
				}
				return guard, "L'acceptation du risque résiduel exige une approbation Gouvernance validée. Soumettez-en une d'abord.", nil
			}
		}
	}
	return "", "", nil
}

func countActive(plans []MitigationSnapshot) int {
	n := 0
	for _, p := range plans {
		if p.Active {
			n++
		}
	}
	return n
}

// incompleteReason names the FIRST plan with work left, quoting its reference and
// the exact count — "2 sous-actions restantes sur la mitigation MIT-14" — because
// "not all sub-actions are complete" tells a user nothing they can act on.
func incompleteReason(plans []MitigationSnapshot, en bool) string {
	for _, p := range plans {
		if !p.Active {
			continue
		}
		if remaining := p.RemainingSubActions(); remaining > 0 {
			ref := p.Ref
			if ref == "" {
				ref = p.Title
			}
			if en {
				return fmt.Sprintf("%d sub-action(s) remaining on mitigation %s", remaining, ref)
			}
			return fmt.Sprintf("%d sous-action(s) restante(s) sur la mitigation %s", remaining, ref)
		}
	}
	return ""
}

func unverifiable(en bool, what string) string {
	if en {
		return "This precondition cannot be verified right now (" + what + " unavailable). The transition is blocked rather than assumed."
	}
	return "Cette condition ne peut pas être vérifiée pour l'instant (" + what + " indisponible). La transition est bloquée plutôt que supposée."
}

func shortID(id uuid.UUID) string {
	s := id.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}
