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
	notices     NoticeDeliverer
}

// NoticeDeliverer tells one member about an event, in-app and by e-mail
// (application/notification.Dispatcher). Optional port.
type NoticeDeliverer interface {
	Deliver(ctx context.Context, n domain.Notice) error
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

// WithNotices tells the risk's owner when someone else moves it through the
// lifecycle (#757). Nil-safe.
func (uc *TransitionRiskStateUseCase) WithNotices(n NoticeDeliverer) *TransitionRiskStateUseCase {
	uc.notices = n
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

	uc.notifyOwner(ctx, tenantID, r, current, target, in.Actor)
	return r, nil
}

// notifyOwner tells the risk's owner that it changed state — best-effort, the
// transition is already committed.
func (uc *TransitionRiskStateUseCase) notifyOwner(ctx context.Context, tenantID uuid.UUID, r *domain.Risk, from, to domain.RiskState, actor uuid.UUID) {
	notifyRiskStateChange(ctx, uc.notices, tenantID, riskStateChangeOf(r, from, to), actor)
}

// riskStateChange is what a state-change notice needs to know about one risk,
// captured at the moment of the change.
type riskStateChange struct {
	RiskID  uuid.UUID
	OwnerID *uuid.UUID
	Title   string
	From    domain.RiskState
	To      domain.RiskState
}

func riskStateChangeOf(r *domain.Risk, from, to domain.RiskState) riskStateChange {
	title := r.Title
	if title == "" {
		title = r.Name
	}
	var owner *uuid.UUID
	if r.OwnerID != nil {
		id := *r.OwnerID
		owner = &id
	}
	return riskStateChange{RiskID: r.ID, OwnerID: owner, Title: title, From: from, To: to}
}

// notifyRiskStateChange is shared by every path that moves a risk's state
// (lifecycle transition, bulk status change). Nobody is told about their own
// move, and a move that changes nothing is not news.
//
// A risk reaching an end state (mitigated, residual accepted, closed) is a
// "risk resolved" notice; every other move is a "risk update". Each follows
// its own e-mail switch in the owner's preferences, so the Settings screen
// keeps meaning what it says.
func notifyRiskStateChange(ctx context.Context, notices NoticeDeliverer, tenantID uuid.UUID, ch riskStateChange, actor uuid.UUID) {
	if notices == nil || ch.OwnerID == nil || *ch.OwnerID == uuid.Nil || *ch.OwnerID == actor || ch.From == ch.To {
		return
	}
	notifType := domain.NotificationTypeRiskUpdate
	switch ch.To {
	case domain.StateMitigated, domain.StateResidualAccepted, domain.StateClosed:
		notifType = domain.NotificationTypeRiskResolved
	}
	id := ch.RiskID
	_ = notices.Deliver(ctx, domain.Notice{
		TenantID:     tenantID,
		UserID:       *ch.OwnerID,
		Type:         notifType,
		ResourceID:   &id,
		ResourceType: "risk",
		Compose: func(locale string) domain.NoticeMessage {
			return riskStateNoticeCopy(locale, ch.Title, ch.From, ch.To)
		},
	})
}

func riskStateNoticeCopy(locale, title string, from, to domain.RiskState) domain.NoticeMessage {
	if locale == "en" {
		return domain.NoticeMessage{
			Subject: fmt.Sprintf("Risk “%s”: %s", title, to.Label("en")),
			Lines: []string{fmt.Sprintf("“%s”, a risk you own, moved from %s to %s.",
				title, from.Label("en"), to.Label("en"))},
		}
	}
	return domain.NoticeMessage{
		Subject: fmt.Sprintf("Risque « %s » : %s", title, to.Label("fr")),
		Lines: []string{fmt.Sprintf("« %s », un risque dont vous êtes responsable, est passé de %s à %s.",
			title, from.Label("fr"), to.Label("fr"))},
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
