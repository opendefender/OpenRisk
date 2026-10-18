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
	// AcceptanceApproval returns the most recent approved request for this risk
	// (entity_type "risk_acceptance") decided after `since`, or nil when there
	// is none, plus the id of a pending one if any, so the UI can link straight
	// to it.
	//
	// `since` is the start of the risk's previous acceptance (#811). An approval
	// decided before it has already been spent on that acceptance; accepting or
	// renewing again needs a new decision, never a re-used one.
	AcceptanceApproval(ctx context.Context, tenantID, riskID uuid.UUID, since *time.Time) (approved *domain.AcceptanceApproval, pendingRequestID *uuid.UUID, err error)
}

// AcceptancePolicy answers the tenant's maximum acceptance length, in months.
// Optional port: unwired, the default of 12 months is also the maximum.
type AcceptancePolicy interface {
	MaxAcceptanceMonths(ctx context.Context, tenantID uuid.UUID) (int, error)
}

// TransitionRiskStateInput is the payload of POST /risks/:id/transition.
type TransitionRiskStateInput struct {
	To      domain.RiskState
	Comment string
	Actor   uuid.UUID
	Locale  string
	// AcceptanceExpiresAt is the end of a residual-risk acceptance. Read only
	// when To is RESIDUAL_ACCEPTED; nil means the default (12 months, capped
	// by the tenant maximum).
	AcceptanceExpiresAt *time.Time
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
	policy      AcceptancePolicy
	journal     AuditJournal
	now         func() time.Time
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

// WithAcceptancePolicy attaches the tenant maximum for acceptances. Nil-safe.
func (uc *TransitionRiskStateUseCase) WithAcceptancePolicy(p AcceptancePolicy) *TransitionRiskStateUseCase {
	uc.policy = p
	return uc
}

// WithJournal attaches the hash-chained audit trail. An acceptance and its
// renewal are recorded there with approver, date and expiry (#811).
func (uc *TransitionRiskStateUseCase) WithJournal(j AuditJournal) *TransitionRiskStateUseCase {
	uc.journal = j
	return uc
}

// WithClock pins the clock (tests).
func (uc *TransitionRiskStateUseCase) WithClock(now func() time.Time) *TransitionRiskStateUseCase {
	uc.now = now
	return uc
}

func (uc *TransitionRiskStateUseCase) clock() time.Time {
	if uc.now != nil {
		return uc.now().UTC()
	}
	return time.Now().UTC()
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
	}

	// The "natural next step" is the NEAREST rung forward on the spine — blocked
	// or not — never merely the first allowed one.
	//
	// Getting this wrong is visible: from TREATMENT_PLANNED the options are
	// IN_TREATMENT (blocked on the mitigation guard) and CLOSED (early closure,
	// always allowed). Picking the first *allowed* forward option made the
	// stepper announce "next: Clôturé" and hide the blocker entirely — pointing
	// the user at abandoning the risk instead of telling them a mitigation is
	// missing. Nearest-first says IN_TREATMENT and surfaces its reason.
	best := -1
	for _, opt := range view.Options {
		if !opt.IsForward {
			continue
		}
		idx := opt.To.StepIndex()
		if best == -1 || idx < best {
			best = idx
			view.Next = opt.To
			view.NextLabel = opt.Label
			view.BlockedReason = opt.Reason // empty when it is allowed
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

	now := uc.clock()
	before := r.AcceptanceSnapshot()
	if target == domain.StateResidualAccepted {
		// The guard above proved an unspent approval exists; read it again to
		// record who gave it and when. Both reads are tenant-scoped.
		approval, _, err := uc.approvals.AcceptanceApproval(ctx, tenantID, r.ID, r.AcceptedAt)
		if err != nil {
			return nil, err
		}
		if approval == nil {
			return nil, domain.NewValidationError("no unspent governance approval for this acceptance")
		}
		expiresAt, err := uc.resolveExpiry(ctx, tenantID, now, in.AcceptanceExpiresAt)
		if err != nil {
			return nil, err
		}
		r.BeginAcceptance(now, expiresAt, *approval)
	}

	// Leaving ASSESSED after an expiry means someone has picked the risk up
	// again; the Action Center item that asked for it goes away.
	if current == domain.StateAssessed && target != domain.StateResidualAccepted {
		r.AcceptanceExpiredAt = nil
	}

	r.SetState(target)
	if target == domain.StateMitigated || target == domain.StateResidualAccepted {
		r.LastMitigatedAt = &now
	}
	r.UpdatedAt = now

	if err := uc.riskRepo.Update(ctx, r); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to transition risk: %v", err))
	}

	// Audit trail (best-effort — the risk is already updated).
	newValue := map[string]interface{}{
		"lifecycle_state": target,
		"status":          r.Status,
		"lifecycle_phase": r.LifecyclePhase,
		"comment":         in.Comment,
	}
	if target == domain.StateResidualAccepted {
		for k, v := range r.AcceptanceSnapshot() {
			newValue[k] = v
		}
	}
	entry := &domain.AuditLogEntry{
		ID:        uuid.New(),
		RiskID:    riskID,
		Timestamp: now,
		ChangedBy: in.Actor,
		Action:    "lifecycle_transition",
		OldValue:  map[string]interface{}{"lifecycle_state": current},
		NewValue:  newValue,
	}
	if err := uc.riskRepo.CreateAuditEntry(ctx, entry); err != nil {
		fmt.Printf("Warning: failed to audit lifecycle transition on risk %s: %v\n", riskID, err)
	}
	if target == domain.StateResidualAccepted {
		uc.journalAcceptance(ctx, tenantID, r, in.Actor, "residual risk accepted", before, in.Comment)
	}

	uc.notifyOwner(ctx, tenantID, r, current, target, in.Actor)
	return r, nil
}

// resolveExpiry applies the tenant maximum to the requested end date.
func (uc *TransitionRiskStateUseCase) resolveExpiry(ctx context.Context, tenantID uuid.UUID, now time.Time, requested *time.Time) (time.Time, error) {
	maxMonths := domain.DefaultAcceptanceMonths
	if uc.policy != nil {
		m, err := uc.policy.MaxAcceptanceMonths(ctx, tenantID)
		if err != nil {
			return time.Time{}, err
		}
		maxMonths = m
	}
	return domain.ResolveAcceptanceExpiry(now, requested, maxMonths)
}

// journalAcceptance writes the acceptance to the hash-chained trail: the
// approver, the request, the date and the expiry, so an auditor reading the
// trail can tell whether the acceptance still stands without opening the risk.
func (uc *TransitionRiskStateUseCase) journalAcceptance(ctx context.Context, tenantID uuid.UUID, r *domain.Risk, actor uuid.UUID, summary string, before map[string]interface{}, comment string) {
	if uc.journal == nil {
		return
	}
	var actorID *uuid.UUID
	if actor != uuid.Nil {
		a := actor
		actorID = &a
	}
	after := r.AcceptanceSnapshot()
	if comment != "" {
		after["comment"] = comment
	}
	ev := &domain.AuditEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		ActorID:       actorID,
		ActorType:     domain.AuditActorUser,
		Action:        domain.AuditActionUpdate,
		EntityType:    "risk",
		EntityID:      r.ID.String(),
		Summary:       summary,
		Before:        domain.JSONMap(before),
		After:         domain.JSONMap(after),
		ChangedFields: domain.StringList{"lifecycle_state", "accepted_at", "acceptance_expires_at", "acceptance_approval_id", "acceptance_approved_by"},
		Source:        domain.AuditSourceExplicit,
	}
	if err := uc.journal.Append(ctx, ev); err != nil {
		fmt.Printf("Warning: failed to journal acceptance on risk %s: %v\n", r.ID, err)
	}
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
			approval, pending, err := uc.approvals.AcceptanceApproval(ctx, tenantID, r.ID, r.AcceptedAt)
			if err != nil {
				return "", "", err
			}
			approved := approval != nil
			if !approved {
				if pending != nil {
					if en {
						return guard, fmt.Sprintf("Approval request %s is still pending. Residual risk can only be accepted once Governance has validated it.", shortID(*pending)), nil
					}
					return guard, fmt.Sprintf("La demande d'approbation %s est encore en attente. Le risque résiduel ne peut être accepté qu'une fois validé par la Gouvernance.", shortID(*pending)), nil
				}
				if r.AcceptedAt != nil {
					// An approval that was already spent on a previous acceptance
					// does not count again (#811).
					if en {
						return guard, "The previous approval was used for the earlier acceptance. Accepting again requires a new validated Governance approval.", nil
					}
					return guard, "L'approbation précédente a servi à l'acceptation antérieure. Accepter à nouveau exige une nouvelle approbation Gouvernance validée.", nil
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
