// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import "testing"

// The specified graph, written out once so the test asserts the SPEC rather
// than re-deriving it from the implementation it is checking.
//
//	DRAFT → IDENTIFIED → ASSESSED → TREATMENT_PLANNED → IN_TREATMENT
//	      → (RESIDUAL_ACCEPTED | MITIGATED) → CLOSED   ↘ REOPENED ↗
var specifiedEdges = map[RiskState][]RiskState{
	StateDraft:            {StateIdentified},
	StateIdentified:       {StateAssessed, StateClosed},
	StateAssessed:         {StateTreatmentPlanned, StateClosed},
	StateTreatmentPlanned: {StateInTreatment, StateAssessed, StateClosed},
	StateInTreatment:      {StateMitigated, StateResidualAccepted, StateTreatmentPlanned},
	StateResidualAccepted: {StateClosed, StateInTreatment},
	StateMitigated:        {StateClosed, StateInTreatment},
	StateClosed:           {StateReopened},
	StateReopened:         {StateAssessed, StateTreatmentPlanned, StateInTreatment, StateClosed},
}

// TestRiskFSM_EveryValidTransitionIsAccepted walks every edge of the specified
// graph.
func TestRiskFSM_EveryValidTransitionIsAccepted(t *testing.T) {
	for from, targets := range specifiedEdges {
		for _, to := range targets {
			if !from.CanTransitionTo(to) {
				t.Errorf("%s → %s is specified but was refused", from, to)
			}
		}
	}
}

// TestRiskFSM_EveryInvalidTransitionIsRefused is the other half, and the one
// that matters: EVERY pair not in the graph must be refused. Written as the
// complete cross product so a new state cannot be added without deciding where
// it may go.
func TestRiskFSM_EveryInvalidTransitionIsRefused(t *testing.T) {
	all := AllRiskStates()
	for _, from := range all {
		allowed := map[RiskState]bool{}
		for _, to := range specifiedEdges[from] {
			allowed[to] = true
		}
		for _, to := range all {
			if allowed[to] {
				continue
			}
			if from.CanTransitionTo(to) {
				t.Errorf("%s → %s is NOT specified but was accepted", from, to)
			}
		}
	}
}

func TestRiskFSM_SelfTransitionIsNeverAllowed(t *testing.T) {
	for _, s := range AllRiskStates() {
		if s.CanTransitionTo(s) {
			t.Errorf("%s → itself must be refused (a no-op is a silent write)", s)
		}
	}
}

func TestRiskFSM_UnknownStatesAreInert(t *testing.T) {
	bogus := RiskState("banana")
	if IsRiskState(bogus) {
		t.Fatal("unknown state reported as known")
	}
	if bogus.CanTransitionTo(StateClosed) {
		t.Fatal("an unknown state must not be able to reach anything")
	}
	if StateDraft.CanTransitionTo(bogus) {
		t.Fatal("nothing must be able to reach an unknown state")
	}
	if len(bogus.NextStates()) != 0 {
		t.Fatal("an unknown state has no successors")
	}
}

// Every state except the terminal one must be able to reach CLOSED, directly or
// not. A state you cannot leave is a trap, and a register full of traps is how
// risks quietly stop being managed.
func TestRiskFSM_NoDeadEnds(t *testing.T) {
	for _, start := range AllRiskStates() {
		if !reaches(start, StateClosed, map[RiskState]bool{}) {
			t.Errorf("%s cannot reach CLOSED — dead end", start)
		}
	}
	// And CLOSED itself is not a grave: it reopens.
	if !StateClosed.CanTransitionTo(StateReopened) {
		t.Error("a closed risk must be reopenable")
	}
}

func reaches(from, target RiskState, seen map[RiskState]bool) bool {
	if from == target {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	for _, next := range from.NextStates() {
		if reaches(next, target, seen) {
			return true
		}
	}
	return false
}

func TestParseRiskState(t *testing.T) {
	cases := map[string]RiskState{
		"":                      StateDraft,
		"in_treatment":          StateInTreatment,
		"IN_TREATMENT":          StateInTreatment, // the spec's own spelling
		"  residual_accepted  ": StateResidualAccepted,
	}
	for in, want := range cases {
		got, err := ParseRiskState(in)
		if err != nil {
			t.Fatalf("ParseRiskState(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseRiskState(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ParseRiskState("done"); err == nil {
		t.Fatal("an unknown state must be a validation error")
	}
}

func TestGuardsFor_MatchTheSpec(t *testing.T) {
	cases := map[RiskState][]TransitionGuard{
		StateInTreatment:      {GuardActiveMitigation},
		StateMitigated:        {GuardSubActionsComplete},
		StateResidualAccepted: {GuardGovernanceApproval},
		StateClosed:           nil,
		StateIdentified:       nil,
	}
	for state, want := range cases {
		got := GuardsFor(state)
		if len(got) != len(want) {
			t.Fatalf("GuardsFor(%s) = %v, want %v", state, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("GuardsFor(%s) = %v, want %v", state, got, want)
			}
		}
	}
}

// SetState is the single writer: status and phase must never be set anywhere
// else, and both must follow the state.
func TestRisk_SetState_DerivesBothLegacyFields(t *testing.T) {
	cases := []struct {
		state  RiskState
		status RiskStatus
		phase  RiskPhase
	}{
		{StateDraft, StatusDraft, PhaseIdentified},
		{StateIdentified, RiskOpen, PhaseIdentified},
		{StateAssessed, RiskOpen, PhaseAnalyzed},
		{StateTreatmentPlanned, RiskOpen, PhaseEvaluated},
		{StateInTreatment, RiskInProgress, PhaseTreated},
		{StateMitigated, RiskMitigated, PhaseMonitored},
		{StateResidualAccepted, RiskAccepted, PhaseMonitored},
		{StateClosed, RiskClosed, PhaseClosed},
		{StateReopened, RiskOpen, PhaseIdentified},
	}
	for _, c := range cases {
		r := &Risk{}
		r.SetState(c.state)
		if r.LifecycleState != c.state || r.Status != c.status || r.LifecyclePhase != c.phase {
			t.Errorf("SetState(%s) → state=%s status=%s phase=%s; want status=%s phase=%s",
				c.state, r.LifecycleState, r.Status, r.LifecyclePhase, c.status, c.phase)
		}
	}
}

// A row written before the column existed must still resolve to a sane state
// rather than reading as "draft" and losing its place in the flow.
func TestRisk_State_FallsBackToLegacyFields(t *testing.T) {
	cases := []struct {
		status RiskStatus
		phase  RiskPhase
		want   RiskState
	}{
		{RiskMitigated, PhaseTreated, StateMitigated},       // status wins over phase
		{RiskAccepted, PhaseTreated, StateResidualAccepted}, // status wins
		{RiskClosed, PhaseIdentified, StateClosed},          // status wins
		{RiskOpen, PhaseTreated, StateInTreatment},          // phase decides
		{RiskOpen, PhaseAnalyzed, StateAssessed},            // phase decides
		{RiskOpen, PhaseEvaluated, StateTreatmentPlanned},   // phase decides
		{RiskInProgress, "", StateInTreatment},              // no phase: status decides
		{StatusDraft, PhaseIdentified, StateDraft},          // legacy uppercase
		{"", "", StateIdentified},                           // nothing recorded
	}
	for _, c := range cases {
		r := &Risk{Status: c.status, LifecyclePhase: c.phase}
		if got := r.State(); got != c.want {
			t.Errorf("State() with status=%q phase=%q = %q, want %q", c.status, c.phase, got, c.want)
		}
	}
	// An explicitly stored state always wins over the derivation.
	r := &Risk{Status: RiskOpen, LifecyclePhase: PhaseIdentified, LifecycleState: StateMitigated}
	if r.State() != StateMitigated {
		t.Fatal("a stored state must win over the legacy fallback")
	}
}

func TestRiskState_Labels(t *testing.T) {
	if StateInTreatment.Label("fr") != "En traitement" {
		t.Fatalf("fr label = %q", StateInTreatment.Label("fr"))
	}
	if StateInTreatment.Label("en") != "In treatment" {
		t.Fatalf("en label = %q", StateInTreatment.Label("en"))
	}
	for _, s := range AllRiskStates() {
		if s.Label("fr") == "" || s.Label("en") == "" {
			t.Fatalf("state %s is missing a label", s)
		}
	}
}

func TestRiskState_StepIndex(t *testing.T) {
	if StateDraft.StepIndex() != 0 {
		t.Fatalf("draft should be step 0, got %d", StateDraft.StepIndex())
	}
	// The two treatment outcomes occupy the same rung of the stepper.
	if StateResidualAccepted.StepIndex() != StateMitigated.StepIndex() {
		t.Fatal("residual_accepted and mitigated are alternatives; they share a step")
	}
	if StateClosed.StepIndex() != StepCount()-1 {
		t.Fatal("closed must be the last step")
	}
	if RiskState("banana").StepIndex() != -1 {
		t.Fatal("an unknown state has no step")
	}
}
