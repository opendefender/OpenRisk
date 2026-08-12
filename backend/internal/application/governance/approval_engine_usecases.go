// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package governance

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// The approval workflow, end to end.
//
// The pure decision rules live in domain/approval_engine.go. This file is the
// wiring: who is asking (including the rights they hold by delegation), what
// they may sign, notifying the people who now have to act, and closing requests
// nobody decided in time.
// ---------------------------------------------------------------------------

// ApprovalNotifier tells the people who now have to act. Optional and
// best-effort everywhere: a notification failure must never lose a decision that
// was validly made.
type ApprovalNotifier interface {
	// NotifyPending tells the approvers of the open steps that a request awaits
	// them. roles and userIDs are the union of what those steps accept.
	NotifyPending(ctx context.Context, tenantID uuid.UUID, req *domain.ApprovalRequest, roles []string, userIDs []uuid.UUID)
	// NotifyResolved tells the requester the outcome.
	NotifyResolved(ctx context.Context, tenantID uuid.UUID, req *domain.ApprovalRequest)
}

// DelegationResolver reports the delegations a user currently holds, so an
// approver covering for an absent colleague can actually sign.
type DelegationResolver interface {
	ActiveDelegationsTo(ctx context.Context, tenantID, userID uuid.UUID, at time.Time) ([]domain.Delegation, error)
}

// RoleResolver reports the org roles a user holds, used to turn a delegation
// from a person into the roles that person could sign for.
type RoleResolver interface {
	RolesFor(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)
}

// ApproverIdentity is what the handler knows from the token, before delegations.
type ApproverIdentity struct {
	UserID  uuid.UUID
	Email   string
	Roles   []string
	IsAdmin bool
}

// DecideInput is a checker's ruling.
type DecideInput struct {
	Decision string // "approve" | "reject"
	Comment  string
	// StepOrder targets a specific branch in parallel mode. Ignored in
	// sequential mode, where there is only ever one open step.
	StepOrder *int
}

// DecideApprovalUseCase advances a request. It is the whole Maker-Checker
// control in one place: four-eyes, eligibility (role, named user, or delegation),
// no double-signing, quorum, sequential vs parallel advancement, expiry, and the
// rule that a refusal must be explained.
type DecideApprovalUseCase struct {
	requests    domain.ApprovalRequestRepository
	recorder    *AuditRecorder
	notifier    ApprovalNotifier
	delegations DelegationResolver
	roles       RoleResolver
}

func NewDecideApprovalUseCase(r domain.ApprovalRequestRepository) *DecideApprovalUseCase {
	return &DecideApprovalUseCase{requests: r}
}

func (uc *DecideApprovalUseCase) WithRecorder(r *AuditRecorder) *DecideApprovalUseCase {
	uc.recorder = r
	return uc
}
func (uc *DecideApprovalUseCase) WithNotifier(n ApprovalNotifier) *DecideApprovalUseCase {
	uc.notifier = n
	return uc
}
func (uc *DecideApprovalUseCase) WithDelegations(d DelegationResolver, r RoleResolver) *DecideApprovalUseCase {
	uc.delegations = d
	uc.roles = r
	return uc
}

func (uc *DecideApprovalUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, who ApproverIdentity, in DecideInput) (*domain.ApprovalRequest, error) {
	decision := strings.ToLower(strings.TrimSpace(in.Decision))
	if decision != "approve" && decision != "reject" {
		return nil, domain.NewValidationError("decision must be 'approve' or 'reject'")
	}
	comment := strings.TrimSpace(in.Comment)
	// A refusal without a reason is the single most expensive gap in an approval
	// system: the requester learns only that they failed, and re-submits the same
	// thing. Refusing to record it is cheaper than explaining it later.
	if decision == "reject" && comment == "" {
		return nil, domain.NewValidationError("a comment is required when refusing a request — say what would have to change")
	}

	req, err := uc.requests.GetRequestByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, domain.NewNotFoundError("approval request", id)
	}
	now := time.Now().UTC()

	// A request past its deadline is closed on read rather than being silently
	// decidable long after the window everyone agreed to.
	if domain.IsExpired(req, now) {
		domain.Expire(req, now)
		if err := uc.requests.UpdateRequest(ctx, req); err != nil {
			return nil, err
		}
		return nil, domain.NewValidationError("this request expired on " + req.ExpiresAt.Format(time.RFC3339) + " and can no longer be decided")
	}
	if req.Status != domain.ApprovalPending {
		return nil, domain.NewValidationError("request is already " + string(req.Status))
	}

	approver := uc.resolveApprover(ctx, tenantID, who, now)

	step := domain.StepFor(req, in.StepOrder)
	if step == nil {
		if in.StepOrder != nil {
			return nil, domain.NewValidationError("that step is not open for signature")
		}
		return nil, domain.NewValidationError("this request has no step awaiting a decision")
	}
	if verdict := domain.CanSign(req, step, approver); !verdict.Eligible {
		return nil, domain.NewForbiddenError(verdict.Reason)
	}

	d := domain.ApprovalDecision{
		StepOrder:     step.Order,
		ApproverID:    approver.UserID.String(),
		ApproverEmail: approver.Email,
		Decision:      decision,
		Comment:       comment,
		DecidedAt:     now,
	}
	domain.ApplyDecision(req, step, d, now)

	if err := uc.requests.UpdateRequest(ctx, req); err != nil {
		return nil, err
	}

	uc.record(ctx, tenantID, approver.UserID, req, decision, comment, step)
	uc.announce(ctx, tenantID, req)
	return req, nil
}

// resolveApprover layers the delegations a user holds onto their own identity.
func (uc *DecideApprovalUseCase) resolveApprover(ctx context.Context, tenantID uuid.UUID, who ApproverIdentity, now time.Time) domain.Approver {
	a := domain.Approver{
		UserID: who.UserID, Email: who.Email, Roles: who.Roles, IsAdmin: who.IsAdmin,
	}
	if uc.delegations == nil {
		return a
	}
	delegations, err := uc.delegations.ActiveDelegationsTo(ctx, tenantID, who.UserID, now)
	if err != nil {
		return a // degrade to the user's own rights rather than failing the decision
	}
	seenRole := map[string]bool{}
	for _, d := range delegations {
		if !d.IsActiveAt(now) {
			continue
		}
		a.DelegatedFrom = append(a.DelegatedFrom, d.DelegatorID)
		if uc.roles == nil {
			continue
		}
		roles, err := uc.roles.RolesFor(ctx, tenantID, d.DelegatorID)
		if err != nil {
			continue
		}
		for _, r := range roles {
			if !seenRole[r] {
				seenRole[r] = true
				a.DelegatedRoles = append(a.DelegatedRoles, r)
			}
		}
	}
	return a
}

func (uc *DecideApprovalUseCase) record(ctx context.Context, tenantID, actorID uuid.UUID, req *domain.ApprovalRequest, decision, comment string, step *domain.WorkflowStep) {
	if uc.recorder == nil {
		return
	}
	action := domain.AuditActionApprove
	if decision == "reject" {
		action = domain.AuditActionReject
	}
	actor := actorID
	summary := string(action) + " step \"" + step.Name + "\" of \"" + req.Title + "\" → " + string(req.Status)
	after := domain.JSONMap{
		"status":       string(req.Status),
		"current_step": req.CurrentStep,
		"step_order":   step.Order,
	}
	if comment != "" {
		after["comment"] = comment
	}
	uc.recorder.Record(ctx, domain.AuditEvent{
		TenantID:   tenantID,
		ActorID:    &actor,
		Action:     action,
		EntityType: "approval_request",
		EntityID:   req.ID.String(),
		Summary:    summary,
		After:      after,
	})
}

// announce tells whoever must act next — the remaining approvers, or the
// requester once the request resolves.
func (uc *DecideApprovalUseCase) announce(ctx context.Context, tenantID uuid.UUID, req *domain.ApprovalRequest) {
	if uc.notifier == nil {
		return
	}
	if req.Status != domain.ApprovalPending {
		uc.notifier.NotifyResolved(ctx, tenantID, req)
		return
	}
	roles, users := PendingAudience(req)
	uc.notifier.NotifyPending(ctx, tenantID, req, roles, users)
}

// PendingAudience is who can act on a request right now: the roles and named
// users of its open steps.
func PendingAudience(req *domain.ApprovalRequest) ([]string, []uuid.UUID) {
	roleSet := map[string]bool{}
	userSet := map[uuid.UUID]bool{}
	for _, s := range domain.OpenSteps(req) {
		if r := strings.TrimSpace(s.ApproverRole); r != "" && !strings.EqualFold(r, "any") {
			roleSet[strings.ToLower(r)] = true
		}
		for _, raw := range s.ApproverUserIDs {
			if id, err := uuid.Parse(strings.TrimSpace(raw)); err == nil {
				userSet[id] = true
			}
		}
	}
	roles := make([]string, 0, len(roleSet))
	for r := range roleSet {
		roles = append(roles, r)
	}
	users := make([]uuid.UUID, 0, len(userSet))
	for u := range userSet {
		users = append(users, u)
	}
	return roles, users
}

// =============================================================================
// Expiry
// =============================================================================

// ExpirableRequestLister finds pending requests past their deadline, across all
// tenants (each row carries its own). Drives the expiry sweep.
type ExpirableRequestLister interface {
	ListExpired(ctx context.Context, now time.Time) ([]domain.ApprovalRequest, error)
}

// ExpireApprovalsUseCase closes requests nobody decided in time. Expiry is a
// distinct terminal state, not a rejection: nobody said no, the window closed,
// and the requester needs to know which of the two happened.
type ExpireApprovalsUseCase struct {
	lister   ExpirableRequestLister
	requests domain.ApprovalRequestRepository
	notifier ApprovalNotifier
	recorder *AuditRecorder
}

func NewExpireApprovalsUseCase(l ExpirableRequestLister, r domain.ApprovalRequestRepository) *ExpireApprovalsUseCase {
	return &ExpireApprovalsUseCase{lister: l, requests: r}
}
func (uc *ExpireApprovalsUseCase) WithNotifier(n ApprovalNotifier) *ExpireApprovalsUseCase {
	uc.notifier = n
	return uc
}
func (uc *ExpireApprovalsUseCase) WithRecorder(r *AuditRecorder) *ExpireApprovalsUseCase {
	uc.recorder = r
	return uc
}

// Execute expires every overdue request and returns how many were closed.
func (uc *ExpireApprovalsUseCase) Execute(ctx context.Context, now time.Time) (int, error) {
	if uc.lister == nil {
		return 0, nil
	}
	overdue, err := uc.lister.ListExpired(ctx, now)
	if err != nil {
		return 0, err
	}
	closed := 0
	for i := range overdue {
		req := overdue[i]
		if !domain.IsExpired(&req, now) {
			continue
		}
		domain.Expire(&req, now)
		if err := uc.requests.UpdateRequest(ctx, &req); err != nil {
			continue // one failure must not stop the sweep
		}
		closed++
		if uc.recorder != nil {
			uc.recorder.Record(ctx, domain.AuditEvent{
				TenantID:   req.TenantID,
				Action:     domain.AuditActionUpdate,
				EntityType: "approval_request",
				EntityID:   req.ID.String(),
				Summary:    "approval request \"" + req.Title + "\" expired without a decision",
				After:      domain.JSONMap{"status": string(domain.ApprovalExpired)},
			})
		}
		if uc.notifier != nil {
			uc.notifier.NotifyResolved(ctx, req.TenantID, &req)
		}
	}
	return closed, nil
}

// =============================================================================
// Reading a request
// =============================================================================

// ApprovalDetail is a request plus everything the UI needs to render it without
// re-deriving the rules: per-step progress, what the caller may do, and why.
type ApprovalDetail struct {
	Request  *domain.ApprovalRequest `json:"request"`
	Progress []domain.StepProgress   `json:"progress"`
	// CanDecide and Verdict answer "why is this button disabled?" before the user
	// clicks it and gets a 403.
	CanDecide bool                        `json:"can_decide"`
	Verdict   domain.EligibilityVerdict   `json:"verdict"`
	OpenSteps []domain.WorkflowStep       `json:"open_steps"`
	Expired   bool                        `json:"expired"`
	Type      *domain.ApprovalRequestType `json:"request_type_info,omitempty"`
}

// GetApprovalDetailUseCase builds the detail view.
type GetApprovalDetailUseCase struct {
	requests    domain.ApprovalRequestRepository
	delegations DelegationResolver
	roles       RoleResolver
	lookup      UserLookup
}

func NewGetApprovalDetailUseCase(r domain.ApprovalRequestRepository) *GetApprovalDetailUseCase {
	return &GetApprovalDetailUseCase{requests: r}
}
func (uc *GetApprovalDetailUseCase) WithDelegations(d DelegationResolver, r RoleResolver) *GetApprovalDetailUseCase {
	uc.delegations = d
	uc.roles = r
	return uc
}
func (uc *GetApprovalDetailUseCase) WithUserLookup(l UserLookup) *GetApprovalDetailUseCase {
	uc.lookup = l
	return uc
}

func (uc *GetApprovalDetailUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, who ApproverIdentity) (*ApprovalDetail, error) {
	req, err := uc.requests.GetRequestByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, domain.NewNotFoundError("approval request", id)
	}
	now := time.Now().UTC()
	detail := &ApprovalDetail{
		Request:  req,
		Progress: domain.Progress(req),
		Expired:  domain.IsExpired(req, now),
	}
	if t, ok := domain.FindApprovalRequestType(req.RequestType); ok {
		detail.Type = &t
	}
	for _, s := range domain.OpenSteps(req) {
		detail.OpenSteps = append(detail.OpenSteps, *s)
	}

	if req.Status == domain.ApprovalPending && !detail.Expired {
		decider := NewDecideApprovalUseCase(uc.requests)
		if uc.delegations != nil {
			decider.WithDelegations(uc.delegations, uc.roles)
		}
		approver := decider.resolveApprover(ctx, tenantID, who, now)
		if step := domain.StepFor(req, nil); step != nil {
			detail.Verdict = domain.CanSign(req, step, approver)
			detail.CanDecide = detail.Verdict.Eligible
		} else {
			detail.Verdict = domain.EligibilityVerdict{Reason: "no step is awaiting a decision"}
		}
	} else if detail.Expired {
		detail.Verdict = domain.EligibilityVerdict{Reason: "this request expired without a decision"}
	} else {
		detail.Verdict = domain.EligibilityVerdict{Reason: "this request is already " + string(req.Status)}
	}

	if uc.lookup != nil && req.RequestedBy != uuid.Nil {
		if emails, err := uc.lookup.EmailsByIDs(ctx, []uuid.UUID{req.RequestedBy}); err == nil {
			req.RequestedByEmail = emails[req.RequestedBy]
		}
	}
	return detail, nil
}
