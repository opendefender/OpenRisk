// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func req(mode string, steps ...WorkflowStep) *ApprovalRequest {
	for i := range steps {
		steps[i].Order = i
		if steps[i].MinApprovals < 1 {
			steps[i].MinApprovals = 1
		}
	}
	return &ApprovalRequest{
		ID: uuid.New(), TenantID: uuid.New(), RequestedBy: uuid.New(),
		Title: "Accept the residual risk on the payment gateway",
		Status: ApprovalPending, Mode: mode,
		Steps: WorkflowStepList(steps), Decisions: ApprovalDecisionList{},
	}
}

func approve(r *ApprovalRequest, step int, who uuid.UUID, now time.Time) {
	s := &r.Steps[step]
	ApplyDecision(r, s, ApprovalDecision{
		StepOrder: s.Order, ApproverID: who.String(), Decision: "approve", DecidedAt: now,
	}, now)
}

func TestCanSign_FourEyesBeatsEverything(t *testing.T) {
	r := req(ApprovalModeSequential, WorkflowStep{Name: "Security", ApproverRole: "admin"})
	// Even an admin cannot approve their own request.
	who := Approver{UserID: r.RequestedBy, Roles: []string{"admin"}, IsAdmin: true}

	v := CanSign(r, &r.Steps[0], who)

	if v.Eligible {
		t.Fatal("the requester must never be able to approve their own request")
	}
	if !strings.Contains(v.Reason, "four-eyes") {
		t.Fatalf("the refusal should name the control, got %q", v.Reason)
	}
}

func TestCanSign_RoleAndNamedApprovers(t *testing.T) {
	ciso, dpo, stranger := uuid.New(), uuid.New(), uuid.New()
	r := req(ApprovalModeSequential, WorkflowStep{
		Name: "Named", ApproverUserIDs: []string{ciso.String(), dpo.String()},
	})

	if v := CanSign(r, &r.Steps[0], Approver{UserID: ciso}); !v.Eligible {
		t.Fatalf("a named approver must be eligible: %s", v.Reason)
	}
	v := CanSign(r, &r.Steps[0], Approver{UserID: stranger, Roles: []string{"manager"}})
	if v.Eligible {
		t.Fatal("someone not named must not be able to sign a named-approver step")
	}
	if !strings.Contains(v.Reason, "named") {
		t.Fatalf("the refusal should explain why, got %q", v.Reason)
	}

	roleStep := req(ApprovalModeSequential, WorkflowStep{Name: "Sec", ApproverRole: "rssi"})
	if v := CanSign(roleStep, &roleStep.Steps[0], Approver{UserID: stranger, Roles: []string{"rssi"}}); !v.Eligible {
		t.Fatalf("a role holder must be eligible: %s", v.Reason)
	}
	if v := CanSign(roleStep, &roleStep.Steps[0], Approver{UserID: stranger, Roles: []string{"viewer"}}); v.Eligible {
		t.Fatal("the wrong role must not be able to sign")
	}
}

func TestCanSign_DelegationCarriesEligibility(t *testing.T) {
	absent, cover := uuid.New(), uuid.New()
	r := req(ApprovalModeSequential, WorkflowStep{Name: "CISO", ApproverUserIDs: []string{absent.String()}})

	// Without the delegation, the stand-in cannot sign.
	if v := CanSign(r, &r.Steps[0], Approver{UserID: cover}); v.Eligible {
		t.Fatal("a stand-in with no delegation must not be able to sign")
	}
	// With it, they can — and the verdict records whose rights were used.
	v := CanSign(r, &r.Steps[0], Approver{UserID: cover, DelegatedFrom: []uuid.UUID{absent}})
	if !v.Eligible {
		t.Fatalf("an active delegation must carry eligibility: %s", v.Reason)
	}
	if v.ViaDelegation != absent.String() {
		t.Fatalf("the verdict must name whose rights were used, got %q", v.ViaDelegation)
	}

	// Role delegation works the same way.
	roleStep := req(ApprovalModeSequential, WorkflowStep{Name: "Sec", ApproverRole: "rssi"})
	if v := CanSign(roleStep, &roleStep.Steps[0], Approver{UserID: cover, DelegatedRoles: []string{"rssi"}}); !v.Eligible {
		t.Fatalf("a delegated role must carry eligibility: %s", v.Reason)
	}
}

func TestCanSign_NoDoubleSigning(t *testing.T) {
	who := uuid.New()
	r := req(ApprovalModeSequential, WorkflowStep{Name: "Two eyes", MinApprovals: 2})
	now := time.Now().UTC()
	approve(r, 0, who, now)

	if v := CanSign(r, &r.Steps[0], Approver{UserID: who}); v.Eligible {
		t.Fatal("one person must not be able to satisfy a two-approver quorum alone")
	}
	if v := CanSign(r, &r.Steps[0], Approver{UserID: uuid.New()}); !v.Eligible {
		t.Fatal("a second, different approver must be able to sign")
	}
}

func TestRequiredApprovals_Quorum(t *testing.T) {
	five := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	cases := []struct {
		name string
		step WorkflowStep
		want int
	}{
		{"default is one", WorkflowStep{}, 1},
		{"absolute count", WorkflowStep{MinApprovals: 3}, 3},
		{"60% of five rounds up to three", WorkflowStep{ApproverUserIDs: five, QuorumPercent: 60}, 3},
		{"51% of five rounds up to three", WorkflowStep{ApproverUserIDs: five, QuorumPercent: 51}, 3},
		{"100% of five is five", WorkflowStep{ApproverUserIDs: five, QuorumPercent: 100}, 5},
		{"quorum never lowers an explicit minimum", WorkflowStep{ApproverUserIDs: five, QuorumPercent: 20, MinApprovals: 4}, 4},
		// A quorum that exceeds the people who could give it can only ever expire.
		{"never more signatures than approvers", WorkflowStep{ApproverUserIDs: five[:2], MinApprovals: 9}, 2},
		{"a percentage without named approvers is not a rule", WorkflowStep{QuorumPercent: 75}, 1},
	}
	for _, tc := range cases {
		if got := RequiredApprovals(tc.step); got != tc.want {
			t.Errorf("%s: expected %d, got %d", tc.name, tc.want, got)
		}
	}
}

func TestApplyDecision_SequentialAdvancesOneStepAtATime(t *testing.T) {
	r := req(ApprovalModeSequential,
		WorkflowStep{Name: "Security"},
		WorkflowStep{Name: "Legal"},
	)
	now := time.Now().UTC()

	approve(r, 0, uuid.New(), now)
	if r.Status != ApprovalPending {
		t.Fatal("one of two steps signed must leave the request pending")
	}
	if r.CurrentStep != 1 {
		t.Fatalf("expected the chain to advance to step 1, got %d", r.CurrentStep)
	}
	if len(OpenSteps(r)) != 1 || OpenSteps(r)[0].Order != 1 {
		t.Fatal("sequential mode must open exactly the next step")
	}

	approve(r, 1, uuid.New(), now)
	if r.Status != ApprovalApproved || r.ResolvedAt == nil {
		t.Fatalf("signing the last step must approve and resolve the request, got %s", r.Status)
	}
}

func TestApplyDecision_ParallelOpensEveryStepAtOnce(t *testing.T) {
	r := req(ApprovalModeParallel,
		WorkflowStep{Name: "Security"},
		WorkflowStep{Name: "Legal"},
		WorkflowStep{Name: "Finance"},
	)
	now := time.Now().UTC()

	if len(OpenSteps(r)) != 3 {
		t.Fatalf("parallel mode must open every step at once, got %d", len(OpenSteps(r)))
	}

	// Sign them out of order — that is the point of parallel.
	approve(r, 2, uuid.New(), now)
	approve(r, 0, uuid.New(), now)
	if r.Status != ApprovalPending {
		t.Fatal("two of three branches must leave the request pending")
	}
	if len(OpenSteps(r)) != 1 || OpenSteps(r)[0].Name != "Legal" {
		t.Fatalf("only the unsigned branch should remain open, got %+v", OpenSteps(r))
	}
	approve(r, 1, uuid.New(), now)
	if r.Status != ApprovalApproved {
		t.Fatalf("every branch signed must approve the request, got %s", r.Status)
	}
}

func TestApplyDecision_RejectionResolvesWhateverTheMode(t *testing.T) {
	for _, mode := range []string{ApprovalModeSequential, ApprovalModeParallel} {
		r := req(mode, WorkflowStep{Name: "A"}, WorkflowStep{Name: "B"})
		now := time.Now().UTC()
		s := &r.Steps[0]
		ApplyDecision(r, s, ApprovalDecision{
			StepOrder: 0, ApproverID: uuid.NewString(), Decision: "reject",
			Comment: "the compensating control is not in place yet", DecidedAt: now,
		}, now)

		if r.Status != ApprovalRejected {
			t.Errorf("%s: one refusal must resolve the whole request, got %s", mode, r.Status)
		}
		if r.ResolvedAt == nil {
			t.Errorf("%s: a rejected request must be resolved", mode)
		}
		if len(OpenSteps(r)) != 0 {
			t.Errorf("%s: a resolved request must have no open steps", mode)
		}
	}
}

func TestApplyDecision_QuorumHoldsTheChain(t *testing.T) {
	r := req(ApprovalModeSequential, WorkflowStep{Name: "Two of the board", MinApprovals: 2})
	now := time.Now().UTC()

	approve(r, 0, uuid.New(), now)
	if r.Status == ApprovalApproved {
		t.Fatal("one signature must not satisfy a two-approver quorum")
	}
	approve(r, 0, uuid.New(), now)
	if r.Status != ApprovalApproved {
		t.Fatalf("the second signature must complete the quorum, got %s", r.Status)
	}
}

func TestProgress_ShowsWhoSignedAndWhatRemains(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	r := req(ApprovalModeSequential, WorkflowStep{Name: "Security", MinApprovals: 2}, WorkflowStep{Name: "Legal"})
	now := time.Now().UTC()
	approve(r, 0, a, now)

	p := Progress(r)
	if p[0].Approvals != 1 || p[0].Required != 2 || p[0].Satisfied {
		t.Fatalf("step 0 should read 1 of 2 and be unsatisfied: %+v", p[0])
	}
	if !p[0].Open {
		t.Fatal("an unsatisfied current step must be open")
	}
	if p[1].Open {
		t.Fatal("a later step must not be open in sequential mode")
	}
	if len(p[0].Approvers) != 1 || p[0].Approvers[0] != a.String() {
		t.Fatalf("progress must name who signed: %+v", p[0].Approvers)
	}

	approve(r, 0, b, now)
	p = Progress(r)
	if !p[0].Satisfied || p[0].Open {
		t.Fatalf("a satisfied step must close: %+v", p[0])
	}
}

func TestExpiry(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	r := req(ApprovalModeSequential, WorkflowStep{Name: "Security"})
	if IsExpired(r, now) {
		t.Fatal("a request with no deadline never expires")
	}
	r.ExpiresAt = &future
	if IsExpired(r, now) {
		t.Fatal("a request inside its window is not expired")
	}
	r.ExpiresAt = &past
	if !IsExpired(r, now) {
		t.Fatal("a pending request past its deadline is expired")
	}

	Expire(r, now)
	if r.Status != ApprovalExpired || r.ResolvedAt == nil {
		t.Fatalf("expiring must resolve the request as expired, got %s", r.Status)
	}
	// Expired is not rejected: nobody refused, the window closed.
	if r.Status == ApprovalRejected {
		t.Fatal("expiry must not be recorded as a refusal")
	}
	if IsExpired(r, now) {
		t.Fatal("an already-expired request is no longer 'pending past its deadline'")
	}
}

func TestStepFor_ParallelTargeting(t *testing.T) {
	r := req(ApprovalModeParallel, WorkflowStep{Name: "A"}, WorkflowStep{Name: "B"})

	if s := StepFor(r, nil); s == nil {
		t.Fatal("with no target, the first open step is taken")
	}
	one := 1
	if s := StepFor(r, &one); s == nil || s.Name != "B" {
		t.Fatal("an approver must be able to target a specific open branch")
	}
	nope := 7
	if s := StepFor(r, &nope); s != nil {
		t.Fatal("targeting a step that is not open must find nothing")
	}

	resolved := req(ApprovalModeSequential, WorkflowStep{Name: "A"})
	resolved.Status = ApprovalApproved
	if s := StepFor(resolved, nil); s != nil {
		t.Fatal("a resolved request has nothing to sign")
	}
}

func TestApprovalRequestTypes_CatalogueIsWellFormed(t *testing.T) {
	types := ApprovalRequestTypes()
	if len(types) == 0 {
		t.Fatal("the catalogue must not be empty")
	}
	seen := map[string]bool{}
	for _, tp := range types {
		if seen[tp.Key] {
			t.Errorf("duplicate request type key %s", tp.Key)
		}
		seen[tp.Key] = true
		if tp.EntityType == "" || tp.Action == "" || tp.Label == "" || tp.Description == "" {
			t.Errorf("%s: a request type must bind to an (entity_type, action) pair and describe itself", tp.Key)
		}
	}
	// risk_acceptance is the one the risk lifecycle enforces; if its pair drifts,
	// RESIDUAL_ACCEPTED silently becomes unreachable.
	rt, ok := FindApprovalRequestType("risk_acceptance")
	if !ok {
		t.Fatal("risk_acceptance must exist — the risk lifecycle depends on it")
	}
	if rt.EntityType != "risk_acceptance" || rt.Action != "accept" {
		t.Fatalf("risk_acceptance must stay bound to (risk_acceptance, accept), got (%s, %s)", rt.EntityType, rt.Action)
	}
	if rt.LinkedToLifecycle == "" {
		t.Error("risk_acceptance should say what depends on it, so deleting its workflow is an informed choice")
	}
}
