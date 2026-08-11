// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"fmt"
	"strings"
)

// RiskState is the SINGLE canonical lifecycle of a risk.
//
// It replaces two overlapping vocabularies that both claimed to say where a risk
// stood and regularly disagreed:
//
//   - RiskStatus  — open / in_progress / mitigated / accepted / closed, plus a
//     legacy uppercase set (DRAFT / ACTIVE / MITIGATED / ACCEPTED)
//   - RiskPhase   — the ISO 31000 phases identified / analyzed / evaluated /
//     treated / monitored / closed
//
// Nothing reconciled them, so a risk could read "mitigated" while sitting in the
// "treated" phase with no completed mitigation anywhere. That is the "cycle de
// vie flou" bug. Both are now DERIVED from this state (see DerivedStatus and
// DerivedPhase) and kept in the columns they always occupied, so every existing
// reader, filter and pill keeps working while there is exactly one writable
// source of truth.
//
// The graph, as specified:
//
//	DRAFT → IDENTIFIED → ASSESSED → TREATMENT_PLANNED → IN_TREATMENT
//	      → (RESIDUAL_ACCEPTED | MITIGATED) → CLOSED   ↘ REOPENED ↗
type RiskState string

const (
	StateDraft            RiskState = "draft"             // captured, not yet part of the register
	StateIdentified       RiskState = "identified"        // in the register, context described
	StateAssessed         RiskState = "assessed"          // probability × impact scored
	StateTreatmentPlanned RiskState = "treatment_planned" // a treatment has been chosen
	StateInTreatment      RiskState = "in_treatment"      // mitigation work is under way
	StateResidualAccepted RiskState = "residual_accepted" // residual risk formally accepted
	StateMitigated        RiskState = "mitigated"         // treatment completed
	StateClosed           RiskState = "closed"            // resolved / no longer relevant
	StateReopened         RiskState = "reopened"          // came back after closure
)

// riskStateOrder is the forward spine of the lifecycle, used for progress
// display (step N of M) — NOT for validating transitions, which are explicit.
var riskStateOrder = []RiskState{
	StateDraft, StateIdentified, StateAssessed, StateTreatmentPlanned,
	StateInTreatment, StateMitigated, StateClosed,
}

// riskStateTransitions is the WHOLE truth about what may follow what. A state
// absent from a list cannot be reached, no matter which client asks.
var riskStateTransitions = map[RiskState][]RiskState{
	StateDraft:            {StateIdentified},
	StateIdentified:       {StateAssessed, StateClosed},
	StateAssessed:         {StateTreatmentPlanned, StateClosed},
	StateTreatmentPlanned: {StateInTreatment, StateAssessed, StateClosed},
	StateInTreatment:      {StateMitigated, StateResidualAccepted, StateTreatmentPlanned},
	StateResidualAccepted: {StateClosed, StateInTreatment},
	StateMitigated:        {StateClosed, StateInTreatment},
	StateClosed:           {StateReopened},
	// Re-entry after a reopen: back to assessment by default, or straight into
	// treatment when the plan still stands.
	StateReopened: {StateAssessed, StateTreatmentPlanned, StateInTreatment, StateClosed},
}

// riskStateLabels are the human labels shown in the stepper (fr, en).
var riskStateLabels = map[RiskState][2]string{
	StateDraft:            {"Brouillon", "Draft"},
	StateIdentified:       {"Identifié", "Identified"},
	StateAssessed:         {"Évalué", "Assessed"},
	StateTreatmentPlanned: {"Traitement planifié", "Treatment planned"},
	StateInTreatment:      {"En traitement", "In treatment"},
	StateResidualAccepted: {"Risque résiduel accepté", "Residual accepted"},
	StateMitigated:        {"Traité", "Mitigated"},
	StateClosed:           {"Clôturé", "Closed"},
	StateReopened:         {"Rouvert", "Reopened"},
}

// AllRiskStates returns every state in spine order, with the two terminal
// branches and the reopen state appended.
func AllRiskStates() []RiskState {
	return []RiskState{
		StateDraft, StateIdentified, StateAssessed, StateTreatmentPlanned,
		StateInTreatment, StateResidualAccepted, StateMitigated, StateClosed, StateReopened,
	}
}

// IsRiskState reports whether s names a known state.
func IsRiskState(s RiskState) bool {
	_, ok := riskStateTransitions[s]
	return ok
}

// ParseRiskState validates and normalises a state name. An empty string maps to
// StateDraft; the legacy uppercase spellings ("IN_TREATMENT") are accepted so a
// client that follows the spec's diagram literally is not punished for it.
func ParseRiskState(s string) (RiskState, error) {
	if s == "" {
		return StateDraft, nil
	}
	candidate := RiskState(strings.ToLower(strings.TrimSpace(s)))
	if IsRiskState(candidate) {
		return candidate, nil
	}
	return "", NewValidationError(fmt.Sprintf("invalid risk state: %q", s))
}

// Label returns the localized label of a state.
func (s RiskState) Label(locale string) string {
	pair, ok := riskStateLabels[s]
	if !ok {
		return string(s)
	}
	if strings.HasPrefix(strings.ToLower(locale), "en") {
		return pair[1]
	}
	return pair[0]
}

// NextStates returns the states structurally reachable from s. It says nothing
// about the guards — GuardedTransition answers that, because the guards need
// data this package must not reach for.
func (s RiskState) NextStates() []RiskState {
	next := riskStateTransitions[s]
	out := make([]RiskState, len(next))
	copy(out, next)
	return out
}

// CanTransitionTo reports whether the edge s→target exists at all.
func (s RiskState) CanTransitionTo(target RiskState) bool {
	for _, allowed := range riskStateTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}

// IsTerminal reports whether the risk has left the working part of the flow.
func (s RiskState) IsTerminal() bool { return s == StateClosed }

// StepIndex is the position of the state on the forward spine, for the stepper.
// The two branch states report the position of the step they replace, and an
// unknown state reports -1.
func (s RiskState) StepIndex() int {
	target := s
	switch s {
	case StateResidualAccepted:
		target = StateMitigated // the two are alternative outcomes of treatment
	case StateReopened:
		target = StateIdentified // a reopened risk is back near the start
	}
	for i, st := range riskStateOrder {
		if st == target {
			return i
		}
	}
	return -1
}

// StepCount is the length of the forward spine.
func StepCount() int { return len(riskStateOrder) }

// ---------------------------------------------------------------------------
// Derivation of the two legacy vocabularies.
//
// These are the ONLY place the old fields are computed. Both columns stay in the
// table and stay correct, but nothing writes them independently any more, which
// is what kept them disagreeing.
// ---------------------------------------------------------------------------

// DerivedStatus maps the canonical state onto the coarse RiskStatus the register
// pill, the filters and the dashboards already read.
func (s RiskState) DerivedStatus() RiskStatus {
	switch s {
	case StateDraft:
		return StatusDraft
	case StateIdentified, StateAssessed, StateTreatmentPlanned, StateReopened:
		return RiskOpen
	case StateInTreatment:
		return RiskInProgress
	case StateMitigated:
		return RiskMitigated
	case StateResidualAccepted:
		return RiskAccepted
	case StateClosed:
		return RiskClosed
	default:
		return RiskOpen
	}
}

// DerivedPhase maps the canonical state onto the ISO 31000 phase the existing
// stepper and the phase facet read.
func (s RiskState) DerivedPhase() RiskPhase {
	switch s {
	case StateDraft, StateIdentified, StateReopened:
		return PhaseIdentified
	case StateAssessed:
		return PhaseAnalyzed
	case StateTreatmentPlanned:
		return PhaseEvaluated
	case StateInTreatment:
		return PhaseTreated
	case StateMitigated, StateResidualAccepted:
		return PhaseMonitored
	case StateClosed:
		return PhaseClosed
	default:
		return PhaseIdentified
	}
}

// RiskStateFromLegacy reconstructs a canonical state from the two old fields.
// Used by migration 0045's backfill and as a read-time fallback for any row
// written before the column existed.
//
// Status wins over phase where they disagree: a resolution ("mitigated",
// "accepted", "closed") is a stronger statement than a phase, and it is the one
// users acted on in the UI.
func RiskStateFromLegacy(status RiskStatus, phase RiskPhase) RiskState {
	switch status {
	case RiskClosed, StatusMitigated:
		if status == RiskClosed {
			return StateClosed
		}
		return StateMitigated
	case RiskMitigated:
		return StateMitigated
	case RiskAccepted, StatusAccepted:
		return StateResidualAccepted
	case StatusDraft:
		return StateDraft
	}

	switch phase {
	case PhaseClosed:
		return StateClosed
	case PhaseMonitored, PhaseTreated:
		return StateInTreatment
	case PhaseEvaluated:
		return StateTreatmentPlanned
	case PhaseAnalyzed:
		return StateAssessed
	case PhaseIdentified:
		return StateIdentified
	}

	// No phase recorded: fall back on the coarse status.
	switch status {
	case RiskInProgress, StatusActive:
		return StateInTreatment
	default:
		return StateIdentified
	}
}

// ---------------------------------------------------------------------------
// Guards
// ---------------------------------------------------------------------------

// TransitionGuard names a precondition the target state imposes.
type TransitionGuard string

const (
	// GuardActiveMitigation — IN_TREATMENT requires at least one active
	// mitigation. Without it "in treatment" is a claim with nothing behind it.
	GuardActiveMitigation TransitionGuard = "active_mitigation"
	// GuardSubActionsComplete — MITIGATED requires every sub-action of every
	// active mitigation to be done. This is what makes the lifecycle and the
	// mitigation plan one flow rather than two things a user keeps in sync.
	GuardSubActionsComplete TransitionGuard = "subactions_complete"
	// GuardGovernanceApproval — RESIDUAL_ACCEPTED requires an approved
	// Governance request. Accepting residual risk is a decision, not a dropdown.
	GuardGovernanceApproval TransitionGuard = "governance_approval"
)

// GuardsFor returns the preconditions a target state imposes.
func GuardsFor(target RiskState) []TransitionGuard {
	switch target {
	case StateInTreatment:
		return []TransitionGuard{GuardActiveMitigation}
	case StateMitigated:
		return []TransitionGuard{GuardSubActionsComplete}
	case StateResidualAccepted:
		return []TransitionGuard{GuardGovernanceApproval}
	default:
		return nil
	}
}

// TransitionOption is one entry of GET /risks/:id/transitions: a reachable
// state, whether it is currently allowed, and — when it is not — exactly what is
// in the way, in words a user can act on.
type TransitionOption struct {
	To      RiskState `json:"to"`
	Label   string    `json:"label"`
	Allowed bool      `json:"allowed"`
	// Reason is empty when Allowed. Otherwise it names the blocker concretely
	// ("2 sous-actions restantes sur la mitigation MIT-14") rather than
	// restating the rule.
	Reason string `json:"reason,omitempty"`
	// Guard names the failed precondition, so the UI can offer the right way
	// out (open the mitigation, submit the approval) instead of a dead end.
	Guard TransitionGuard `json:"guard,omitempty"`
	// IsForward marks progress along the spine, for styling.
	IsForward bool `json:"is_forward"`
}
