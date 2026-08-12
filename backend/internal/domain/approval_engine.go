// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// The approval engine, as pure decision logic.
//
// Eligibility, quorum, sequential vs parallel advancement and expiry all live
// here as functions over values: no database, no HTTP, no clock beyond the
// instant handed in. That is what lets the rules be tested exhaustively, and it
// is why the same logic can answer "who can sign this?" in the UI and "may this
// person sign?" at the API without the two drifting apart.
// ---------------------------------------------------------------------------

// ApprovalMode says whether the chain's steps run one after another or all at
// once. Sequential is the default because it is what "chain" implies; parallel
// exists for the real case where legal, security and finance each have to sign
// and none of them should wait on the others.
const (
	ApprovalModeSequential = "sequential"
	ApprovalModeParallel   = "parallel"
)

// ApprovalExpired is the terminal state of a request nobody decided in time.
// Distinct from rejected: nobody said no, the window closed.
const ApprovalExpired ApprovalStatus = "expired"

// ApprovalRequestType is a kind of decision the organisation routes for sign-off.
// Shipping a catalogue rather than a free-text field is what makes "types de
// demandes" a shared vocabulary instead of five spellings of the same thing.
type ApprovalRequestType struct {
	Key         string `json:"key"`
	EntityType  string `json:"entity_type"`
	Action      string `json:"action"`
	Label       string `json:"label"`
	LabelEN     string `json:"label_en"`
	Description string `json:"description"`
	// LinkedToLifecycle names the product behaviour that depends on this type, so
	// an admin can see that deleting the workflow will block something concrete.
	LinkedToLifecycle string `json:"linked_to_lifecycle,omitempty"`
}

// ApprovalRequestTypes is the catalogue. risk_acceptance is first because it is
// the one the risk lifecycle actually enforces: a risk cannot move to
// RESIDUAL_ACCEPTED without an approved request of this type.
func ApprovalRequestTypes() []ApprovalRequestType {
	return []ApprovalRequestType{
		{
			Key: "risk_acceptance", EntityType: "risk_acceptance", Action: "accept",
			Label: "Acceptation de risque résiduel", LabelEN: "Residual risk acceptance",
			Description: "Accepter formellement un risque résiduel plutôt que de le traiter. Une décision, pas une case à cocher.",
			LinkedToLifecycle: "Requis pour faire passer un risque à RESIDUAL_ACCEPTED : " +
				"sans demande approuvée, la transition est refusée.",
		},
		{
			Key: "security_exception", EntityType: "security_exception", Action: "grant",
			Label: "Dérogation de sécurité", LabelEN: "Security exception",
			Description: "Autoriser temporairement un écart à une politique de sécurité, avec une date de fin.",
		},
		{
			Key: "policy_change", EntityType: "policy", Action: "publish",
			Label: "Changement de politique", LabelEN: "Policy change",
			Description: "Publier ou modifier une politique interne opposable.",
		},
		{
			Key: "access_grant", EntityType: "access_request", Action: "grant",
			Label: "Octroi d'accès privilégié", LabelEN: "Privileged access grant",
			Description: "Accorder un accès élevé à un système sensible.",
		},
		{
			Key: "vendor_onboarding", EntityType: "vendor", Action: "onboard",
			Label: "Référencement d'un fournisseur", LabelEN: "Vendor onboarding",
			Description: "Valider l'entrée d'un tiers dans le périmètre, après évaluation.",
		},
		{
			Key: "control_waiver", EntityType: "compliance_control", Action: "waive",
			Label: "Dispense de contrôle", LabelEN: "Control waiver",
			Description: "Marquer un contrôle comme non applicable, avec justification.",
		},
	}
}

// FindApprovalRequestType resolves a catalogue entry by key.
func FindApprovalRequestType(key string) (ApprovalRequestType, bool) {
	for _, t := range ApprovalRequestTypes() {
		if t.Key == key {
			return t, true
		}
	}
	return ApprovalRequestType{}, false
}

// =============================================================================
// Eligibility
// =============================================================================

// Approver is who is trying to sign, and everything that bears on whether they
// may. Delegations are part of the identity on purpose: an approver covering for
// an absent colleague signs as themselves, with the colleague's rights, and the
// decision records who really signed.
type Approver struct {
	UserID  uuid.UUID
	Email   string
	Roles   []string
	IsAdmin bool
	// DelegatedFrom lists users whose approval rights this person currently holds
	// (resolved from active delegations).
	DelegatedFrom []uuid.UUID
	// DelegatedRoles are the roles those delegations confer.
	DelegatedRoles []string
}

// EligibilityVerdict explains a yes or a no. A bare boolean would make the UI
// guess at the reason, and guessing produces "you are not allowed" with no way
// forward.
type EligibilityVerdict struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason"`
	// ViaDelegation names the person whose rights were used, when they were.
	ViaDelegation string `json:"via_delegation,omitempty"`
}

// CanSign decides whether an approver may sign a given step of a given request.
// The rules, in order, are the Maker-Checker control itself:
//
//  1. four-eyes — the requester may never sign their own request, admin or not;
//  2. the step names eligible users and/or a role; an admin satisfies any step;
//  3. an active delegation from an eligible user carries that eligibility;
//  4. nobody signs the same step twice.
func CanSign(req *ApprovalRequest, step *WorkflowStep, who Approver) EligibilityVerdict {
	if req == nil || step == nil {
		return EligibilityVerdict{Reason: "there is no pending step to sign"}
	}
	if who.UserID == req.RequestedBy {
		return EligibilityVerdict{
			Reason: "you raised this request, so you cannot approve it (four-eyes control)",
		}
	}
	for _, d := range req.Decisions {
		if d.StepOrder == step.Order && d.ApproverID == who.UserID.String() && d.Decision == "approve" {
			return EligibilityVerdict{Reason: "you have already approved this step"}
		}
	}

	// Named users take precedence: a step that names people means those people.
	if len(step.ApproverUserIDs) > 0 {
		for _, id := range step.ApproverUserIDs {
			if strings.EqualFold(strings.TrimSpace(id), who.UserID.String()) {
				return EligibilityVerdict{Eligible: true, Reason: "you are named as an approver for this step"}
			}
			for _, from := range who.DelegatedFrom {
				if strings.EqualFold(strings.TrimSpace(id), from.String()) {
					return EligibilityVerdict{
						Eligible:      true,
						Reason:        "you hold an active delegation from a named approver",
						ViaDelegation: from.String(),
					}
				}
			}
		}
		// A named-user step may ALSO name a role; fall through to the role check
		// rather than refusing outright.
		if strings.TrimSpace(step.ApproverRole) == "" {
			if who.IsAdmin {
				return EligibilityVerdict{Eligible: true, Reason: "administrators may sign any step"}
			}
			return EligibilityVerdict{Reason: "this step is reserved for named approvers"}
		}
	}

	role := strings.ToLower(strings.TrimSpace(step.ApproverRole))
	if role == "" || role == "any" {
		return EligibilityVerdict{Eligible: true, Reason: "any member of the organisation may sign this step"}
	}
	for _, r := range who.Roles {
		if strings.EqualFold(strings.TrimSpace(r), role) {
			return EligibilityVerdict{Eligible: true, Reason: "your role (" + role + ") may sign this step"}
		}
	}
	for _, r := range who.DelegatedRoles {
		if strings.EqualFold(strings.TrimSpace(r), role) {
			return EligibilityVerdict{
				Eligible: true,
				Reason:   "a delegation grants you the " + role + " role for this step",
			}
		}
	}
	if who.IsAdmin {
		return EligibilityVerdict{Eligible: true, Reason: "administrators may sign any step"}
	}
	return EligibilityVerdict{Reason: "this step requires the " + role + " role"}
}

// =============================================================================
// Quorum and advancement
// =============================================================================

// RequiredApprovals is how many distinct approvers a step needs. A percentage
// quorum is resolved against the number of named approvers; without named
// approvers it falls back to the absolute count, because a percentage of an
// unknown population is not a rule, it is a wish.
func RequiredApprovals(step WorkflowStep) int {
	min := step.MinApprovals
	if min < 1 {
		min = 1
	}
	if step.QuorumPercent > 0 && len(step.ApproverUserIDs) > 0 {
		needed := (len(step.ApproverUserIDs)*step.QuorumPercent + 99) / 100 // ceil
		if needed > min {
			min = needed
		}
	}
	if min > len(step.ApproverUserIDs) && len(step.ApproverUserIDs) > 0 {
		// Never require more signatures than there are people who can give them:
		// that is a request that can only ever expire.
		min = len(step.ApproverUserIDs)
	}
	return min
}

// StepProgress is one step's standing, for the UI and for advancement.
type StepProgress struct {
	Order     int      `json:"order"`
	Name      string   `json:"name"`
	Role      string   `json:"approver_role,omitempty"`
	Required  int      `json:"required_approvals"`
	Approvals int      `json:"approvals"`
	Satisfied bool     `json:"satisfied"`
	Rejected  bool     `json:"rejected"`
	Open      bool     `json:"open"` // may be signed right now
	Approvers []string `json:"approvers,omitempty"`
}

// Progress renders every step's standing under the request's mode.
func Progress(req *ApprovalRequest) []StepProgress {
	out := make([]StepProgress, 0, len(req.Steps))
	parallel := strings.EqualFold(req.Mode, ApprovalModeParallel)
	for i, s := range req.Steps {
		p := StepProgress{Order: s.Order, Name: s.Name, Role: s.ApproverRole, Required: RequiredApprovals(s)}
		seen := map[string]bool{}
		for _, d := range req.Decisions {
			if d.StepOrder != s.Order {
				continue
			}
			if d.Decision == "reject" {
				p.Rejected = true
			}
			if d.Decision == "approve" && !seen[d.ApproverID] {
				seen[d.ApproverID] = true
				p.Approvals++
				label := d.ApproverEmail
				if label == "" {
					label = d.ApproverID
				}
				p.Approvers = append(p.Approvers, label)
			}
		}
		p.Satisfied = p.Approvals >= p.Required
		if req.Status == ApprovalPending {
			p.Open = !p.Satisfied && !p.Rejected && (parallel || i == req.CurrentStep)
		}
		out = append(out, p)
	}
	return out
}

// OpenSteps returns the steps that may be signed right now. Sequential mode has
// at most one; parallel mode has every unsatisfied step.
func OpenSteps(req *ApprovalRequest) []*WorkflowStep {
	var out []*WorkflowStep
	progress := Progress(req)
	for i := range req.Steps {
		if progress[i].Open {
			out = append(out, &req.Steps[i])
		}
	}
	return out
}

// StepFor finds the step a decision targets. In sequential mode that is the
// current step; in parallel mode the approver names it, and any open step is
// acceptable. Returns nil when the request has nothing open for this approver.
func StepFor(req *ApprovalRequest, requestedOrder *int) *WorkflowStep {
	open := OpenSteps(req)
	if len(open) == 0 {
		return nil
	}
	if requestedOrder == nil {
		return open[0]
	}
	for _, s := range open {
		if s.Order == *requestedOrder {
			return s
		}
	}
	return nil
}

// ApplyDecision records a decision and advances the state machine. It is the
// single writer of Status / CurrentStep / ResolvedAt, so those three can never
// disagree.
//
// A rejection resolves the whole request whatever the mode: one "no" from an
// eligible approver is a no. That is deliberate — a chain where a rejection only
// stalls one branch produces requests that are neither approved nor refused.
func ApplyDecision(req *ApprovalRequest, step *WorkflowStep, decision ApprovalDecision, now time.Time) {
	req.Decisions = append(req.Decisions, decision)

	if decision.Decision == "reject" {
		req.Status = ApprovalRejected
		req.ResolvedAt = &now
		return
	}

	progress := Progress(req)
	if strings.EqualFold(req.Mode, ApprovalModeParallel) {
		for _, p := range progress {
			if !p.Satisfied {
				return // still waiting on at least one branch
			}
		}
		req.Status = ApprovalApproved
		req.ResolvedAt = &now
		req.CurrentStep = len(req.Steps)
		return
	}

	// Sequential: advance past every step that is now satisfied.
	for req.CurrentStep < len(req.Steps) && progress[req.CurrentStep].Satisfied {
		req.CurrentStep++
	}
	if req.CurrentStep >= len(req.Steps) {
		req.Status = ApprovalApproved
		req.ResolvedAt = &now
	}
}

// IsExpired reports whether a pending request has passed its deadline.
func IsExpired(req *ApprovalRequest, now time.Time) bool {
	return req.Status == ApprovalPending && req.ExpiresAt != nil && now.After(*req.ExpiresAt)
}

// Expire moves a request to its terminal expired state.
func Expire(req *ApprovalRequest, now time.Time) {
	req.Status = ApprovalExpired
	req.ResolvedAt = &now
}
