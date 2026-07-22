// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package governance

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// Workflow configuration (the Checker chains an admin defines up-front)
// =============================================================================

// WorkflowInput is the payload to create or edit an approval workflow.
type WorkflowInput struct {
	Name        string
	Description string
	EntityType  string
	Action      string
	Enabled     *bool
	Steps       []domain.WorkflowStep
}

type CreateWorkflowUseCase struct {
	repo domain.ApprovalWorkflowRepository
}

func NewCreateWorkflowUseCase(repo domain.ApprovalWorkflowRepository) *CreateWorkflowUseCase {
	return &CreateWorkflowUseCase{repo: repo}
}

func (uc *CreateWorkflowUseCase) Execute(ctx context.Context, tenantID, actorID uuid.UUID, in WorkflowInput) (*domain.ApprovalWorkflow, error) {
	w := &domain.ApprovalWorkflow{
		TenantID:    tenantID,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		EntityType:  strings.TrimSpace(in.EntityType),
		Action:      strings.TrimSpace(in.Action),
		Enabled:     true,
		Steps:       domain.WorkflowStepList(normaliseSteps(in.Steps)),
		CreatedBy:   actorID,
	}
	if in.Enabled != nil {
		w.Enabled = *in.Enabled
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	// One workflow per (tenant, entity_type, action).
	if existing, err := uc.repo.FindWorkflow(ctx, tenantID, w.EntityType, w.Action); err == nil && existing != nil {
		return nil, domain.NewConflictError("approval workflow", "entity_type+action")
	}
	if err := uc.repo.CreateWorkflow(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

type ListWorkflowsUseCase struct {
	repo domain.ApprovalWorkflowRepository
}

func NewListWorkflowsUseCase(repo domain.ApprovalWorkflowRepository) *ListWorkflowsUseCase {
	return &ListWorkflowsUseCase{repo: repo}
}
func (uc *ListWorkflowsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ApprovalWorkflow, error) {
	return uc.repo.ListWorkflows(ctx, tenantID)
}

type UpdateWorkflowUseCase struct {
	repo domain.ApprovalWorkflowRepository
}

func NewUpdateWorkflowUseCase(repo domain.ApprovalWorkflowRepository) *UpdateWorkflowUseCase {
	return &UpdateWorkflowUseCase{repo: repo}
}
func (uc *UpdateWorkflowUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, in WorkflowInput) (*domain.ApprovalWorkflow, error) {
	w, err := uc.repo.GetWorkflowByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, domain.NewNotFoundError("workflow", id)
	}
	if s := strings.TrimSpace(in.Name); s != "" {
		w.Name = s
	}
	w.Description = strings.TrimSpace(in.Description)
	if s := strings.TrimSpace(in.EntityType); s != "" {
		w.EntityType = s
	}
	w.Action = strings.TrimSpace(in.Action)
	if in.Enabled != nil {
		w.Enabled = *in.Enabled
	}
	if in.Steps != nil {
		w.Steps = domain.WorkflowStepList(normaliseSteps(in.Steps))
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	if err := uc.repo.UpdateWorkflow(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

type DeleteWorkflowUseCase struct {
	repo domain.ApprovalWorkflowRepository
}

func NewDeleteWorkflowUseCase(repo domain.ApprovalWorkflowRepository) *DeleteWorkflowUseCase {
	return &DeleteWorkflowUseCase{repo: repo}
}
func (uc *DeleteWorkflowUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.repo.DeleteWorkflow(ctx, id, tenantID)
}

// =============================================================================
// Requests (the live Maker-Checker state machine)
// =============================================================================

// SubmitApprovalInput is a Maker's request for a change that needs sign-off.
type SubmitApprovalInput struct {
	EntityType  string
	EntityID    string
	Action      string
	Title       string
	Description string
	Payload     domain.JSONMap
}

type SubmitApprovalRequestUseCase struct {
	workflows domain.ApprovalWorkflowRepository
	requests  domain.ApprovalRequestRepository
	recorder  *AuditRecorder
}

func NewSubmitApprovalRequestUseCase(w domain.ApprovalWorkflowRepository, r domain.ApprovalRequestRepository) *SubmitApprovalRequestUseCase {
	return &SubmitApprovalRequestUseCase{workflows: w, requests: r}
}
func (uc *SubmitApprovalRequestUseCase) WithRecorder(r *AuditRecorder) *SubmitApprovalRequestUseCase {
	uc.recorder = r
	return uc
}

func (uc *SubmitApprovalRequestUseCase) Execute(ctx context.Context, tenantID, requesterID uuid.UUID, in SubmitApprovalInput) (*domain.ApprovalRequest, error) {
	entityType := strings.TrimSpace(in.EntityType)
	if entityType == "" {
		return nil, domain.NewValidationError("entity_type is required")
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, domain.NewValidationError("title is required")
	}
	wf, err := uc.workflows.FindWorkflow(ctx, tenantID, entityType, strings.TrimSpace(in.Action))
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, domain.NewValidationError("no approval workflow is configured for " + entityType + "/" + in.Action)
	}
	if len(wf.Steps) == 0 {
		return nil, domain.NewValidationError("workflow has no steps")
	}

	req := &domain.ApprovalRequest{
		TenantID:     tenantID,
		WorkflowID:   &wf.ID,
		WorkflowName: wf.Name,
		EntityType:   entityType,
		EntityID:     strings.TrimSpace(in.EntityID),
		Action:       strings.TrimSpace(in.Action),
		Title:        strings.TrimSpace(in.Title),
		Description:  strings.TrimSpace(in.Description),
		Payload:      in.Payload,
		Status:       domain.ApprovalPending,
		CurrentStep:  0,
		Steps:        wf.Steps, // snapshot — later workflow edits don't rewrite this request
		Decisions:    domain.ApprovalDecisionList{},
		RequestedBy:  requesterID,
	}
	if err := uc.requests.CreateRequest(ctx, req); err != nil {
		return nil, err
	}
	if uc.recorder != nil {
		actor := requesterID
		uc.recorder.Record(ctx, domain.AuditEvent{
			TenantID:   tenantID,
			ActorID:    &actor,
			Action:     domain.AuditActionSubmit,
			EntityType: "approval_request",
			EntityID:   req.ID.String(),
			Summary:    "submitted " + entityType + " for approval: " + req.Title,
			After:      domain.JSONMap{"entity_type": entityType, "entity_id": req.EntityID, "action": req.Action},
		})
	}
	return req, nil
}

// ApproverContext identifies who is deciding and what they may sign.
type ApproverContext struct {
	UserID  uuid.UUID
	Roles   []string // org role names the user holds in this tenant
	IsAdmin bool
}

func (a ApproverContext) canSign(step *domain.WorkflowStep) bool {
	if a.IsAdmin {
		return true
	}
	role := strings.TrimSpace(strings.ToLower(step.ApproverRole))
	if role == "" || role == "any" {
		return true
	}
	for _, r := range a.Roles {
		if strings.ToLower(strings.TrimSpace(r)) == role {
			return true
		}
	}
	return false
}

// DecideApprovalInput is a Checker's ruling on the current step.
type DecideApprovalInput struct {
	Decision string // "approve" | "reject"
	Comment  string
}

// DecideApprovalStepUseCase advances the state machine. This is the heart of the
// Maker-Checker control: it enforces four-eyes (a requester can never approve
// their own request), role eligibility per step, no double-signing a step, and
// the min-approvals gate before the chain advances.
type DecideApprovalStepUseCase struct {
	requests domain.ApprovalRequestRepository
	recorder *AuditRecorder
}

func NewDecideApprovalStepUseCase(r domain.ApprovalRequestRepository) *DecideApprovalStepUseCase {
	return &DecideApprovalStepUseCase{requests: r}
}
func (uc *DecideApprovalStepUseCase) WithRecorder(r *AuditRecorder) *DecideApprovalStepUseCase {
	uc.recorder = r
	return uc
}

func (uc *DecideApprovalStepUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, who ApproverContext, in DecideApprovalInput) (*domain.ApprovalRequest, error) {
	decision := strings.ToLower(strings.TrimSpace(in.Decision))
	if decision != "approve" && decision != "reject" {
		return nil, domain.NewValidationError("decision must be 'approve' or 'reject'")
	}
	req, err := uc.requests.GetRequestByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, domain.NewNotFoundError("approval request", id)
	}
	if req.Status != domain.ApprovalPending {
		return nil, domain.NewValidationError("request is already " + string(req.Status))
	}
	// Four-eyes: the maker cannot be a checker on their own request.
	if who.UserID == req.RequestedBy {
		return nil, domain.NewForbiddenError("the requester cannot approve their own request (four-eyes)")
	}
	step := req.CurrentStepDef()
	if step == nil {
		return nil, domain.NewValidationError("request has no pending step")
	}
	if !who.canSign(step) {
		return nil, domain.NewForbiddenError("your role is not eligible to sign this step")
	}
	// No double-signing the same step.
	for _, d := range req.Decisions {
		if d.StepOrder == req.CurrentStep && d.ApproverID == who.UserID.String() && d.Decision == "approve" {
			return nil, domain.NewValidationError("you have already approved this step")
		}
	}

	now := time.Now().UTC()
	req.Decisions = append(req.Decisions, domain.ApprovalDecision{
		StepOrder:  req.CurrentStep,
		ApproverID: who.UserID.String(),
		Decision:   decision,
		Comment:    strings.TrimSpace(in.Comment),
		DecidedAt:  now,
	})

	auditAction := domain.AuditActionApprove
	if decision == "reject" {
		req.Status = domain.ApprovalRejected
		req.ResolvedAt = &now
		auditAction = domain.AuditActionReject
	} else {
		// Advance only once this step has enough distinct approvers.
		min := step.MinApprovals
		if min < 1 {
			min = 1
		}
		if req.ApprovalsAtStep(req.CurrentStep) >= min {
			req.CurrentStep++
			if req.CurrentStep >= len(req.Steps) {
				req.Status = domain.ApprovalApproved
				req.ResolvedAt = &now
			}
		}
	}

	if err := uc.requests.UpdateRequest(ctx, req); err != nil {
		return nil, err
	}
	if uc.recorder != nil {
		actor := who.UserID
		uc.recorder.Record(ctx, domain.AuditEvent{
			TenantID:   tenantID,
			ActorID:    &actor,
			Action:     auditAction,
			EntityType: "approval_request",
			EntityID:   req.ID.String(),
			Summary:    string(auditAction) + " step of \"" + req.Title + "\" → " + string(req.Status),
		})
	}
	return req, nil
}

// CancelApprovalRequestUseCase lets the maker withdraw their own pending request.
type CancelApprovalRequestUseCase struct {
	requests domain.ApprovalRequestRepository
}

func NewCancelApprovalRequestUseCase(r domain.ApprovalRequestRepository) *CancelApprovalRequestUseCase {
	return &CancelApprovalRequestUseCase{requests: r}
}
func (uc *CancelApprovalRequestUseCase) Execute(ctx context.Context, tenantID, actorID, id uuid.UUID) (*domain.ApprovalRequest, error) {
	req, err := uc.requests.GetRequestByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, domain.NewNotFoundError("approval request", id)
	}
	if req.RequestedBy != actorID {
		return nil, domain.NewForbiddenError("only the requester can cancel this request")
	}
	if req.Status != domain.ApprovalPending {
		return nil, domain.NewValidationError("request is already " + string(req.Status))
	}
	now := time.Now().UTC()
	req.Status = domain.ApprovalCancelled
	req.ResolvedAt = &now
	if err := uc.requests.UpdateRequest(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

// ListApprovalRequestsUseCase lists requests (the approval inbox / history) and
// resolves requester emails for display.
type ListApprovalRequestsUseCase struct {
	requests domain.ApprovalRequestRepository
	lookup   UserLookup
}

func NewListApprovalRequestsUseCase(r domain.ApprovalRequestRepository) *ListApprovalRequestsUseCase {
	return &ListApprovalRequestsUseCase{requests: r}
}
func (uc *ListApprovalRequestsUseCase) WithUserLookup(l UserLookup) *ListApprovalRequestsUseCase {
	uc.lookup = l
	return uc
}
func (uc *ListApprovalRequestsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, f domain.ApprovalRequestFilter) ([]domain.ApprovalRequest, error) {
	list, err := uc.requests.ListRequests(ctx, tenantID, f)
	if err != nil {
		return nil, err
	}
	if uc.lookup != nil && len(list) > 0 {
		idset := map[uuid.UUID]struct{}{}
		for i := range list {
			idset[list[i].RequestedBy] = struct{}{}
		}
		ids := make([]uuid.UUID, 0, len(idset))
		for id := range idset {
			if id != uuid.Nil {
				ids = append(ids, id)
			}
		}
		if emails, err := uc.lookup.EmailsByIDs(ctx, ids); err == nil {
			for i := range list {
				list[i].RequestedByEmail = emails[list[i].RequestedBy]
			}
		}
	}
	return list, nil
}

// normaliseSteps orders steps, backfills the order index and min-approvals.
func normaliseSteps(in []domain.WorkflowStep) []domain.WorkflowStep {
	out := make([]domain.WorkflowStep, 0, len(in))
	for i, s := range in {
		s.Order = i
		s.Name = strings.TrimSpace(s.Name)
		if s.Name == "" {
			s.Name = "Step " + itoa(i+1)
		}
		s.ApproverRole = strings.TrimSpace(s.ApproverRole)
		if s.MinApprovals < 1 {
			s.MinApprovals = 1
		}
		out = append(out, s)
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
