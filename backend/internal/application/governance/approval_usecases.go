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

// =============================================================================
// Workflow configuration (the Checker chains an admin defines up-front)
// =============================================================================

// WorkflowInput is the payload to create or edit an approval workflow.
type WorkflowInput struct {
	Name           string
	Description    string
	EntityType     string
	Action         string
	RequestType    string
	Mode           string
	ExpiresInHours int
	Enabled        *bool
	Steps          []domain.WorkflowStep
}

// resolveType lets an admin pick a catalogue request type and have the
// (entity_type, action) pair filled in for them — the pair the submit path
// matches on. Typing them by hand is how a workflow ends up bound to nothing.
func (in *WorkflowInput) resolveType() {
	if t, ok := domain.FindApprovalRequestType(strings.TrimSpace(in.RequestType)); ok {
		if strings.TrimSpace(in.EntityType) == "" {
			in.EntityType = t.EntityType
		}
		if strings.TrimSpace(in.Action) == "" {
			in.Action = t.Action
		}
	}
}

func normaliseMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), domain.ApprovalModeParallel) {
		return domain.ApprovalModeParallel
	}
	return domain.ApprovalModeSequential
}

type CreateWorkflowUseCase struct {
	repo    domain.ApprovalWorkflowRepository
	members MemberLookup
}

// WithMembers lets Execute prove that every named approver belongs to the
// tenant (#1520). Without it a workflow that names approvers is refused.
func (uc *CreateWorkflowUseCase) WithMembers(m MemberLookup) *CreateWorkflowUseCase {
	uc.members = m
	return uc
}

func NewCreateWorkflowUseCase(repo domain.ApprovalWorkflowRepository) *CreateWorkflowUseCase {
	return &CreateWorkflowUseCase{repo: repo}
}

func (uc *CreateWorkflowUseCase) Execute(ctx context.Context, tenantID, actorID uuid.UUID, in WorkflowInput) (*domain.ApprovalWorkflow, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	in.resolveType()
	w := &domain.ApprovalWorkflow{
		TenantID:       tenantID,
		Name:           strings.TrimSpace(in.Name),
		Description:    strings.TrimSpace(in.Description),
		EntityType:     strings.TrimSpace(in.EntityType),
		Action:         strings.TrimSpace(in.Action),
		RequestType:    strings.TrimSpace(in.RequestType),
		Mode:           normaliseMode(in.Mode),
		ExpiresInHours: in.ExpiresInHours,
		Enabled:        true,
		Steps:          domain.WorkflowStepList(normaliseSteps(in.Steps)),
		CreatedBy:      actorID,
	}
	if w.ExpiresInHours < 0 {
		return nil, domain.NewValidationError("expires_in_hours cannot be negative (0 means the request never expires)")
	}
	if in.Enabled != nil {
		w.Enabled = *in.Enabled
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	if err := checkApprovers(ctx, uc.members, tenantID, []domain.WorkflowStep(w.Steps)); err != nil {
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
	repo    domain.ApprovalWorkflowRepository
	members MemberLookup
}

// WithMembers lets Execute prove that every named approver belongs to the
// tenant (#1520). Without it a workflow that names approvers is refused.
func (uc *UpdateWorkflowUseCase) WithMembers(m MemberLookup) *UpdateWorkflowUseCase {
	uc.members = m
	return uc
}

func NewUpdateWorkflowUseCase(repo domain.ApprovalWorkflowRepository) *UpdateWorkflowUseCase {
	return &UpdateWorkflowUseCase{repo: repo}
}
func (uc *UpdateWorkflowUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, in WorkflowInput) (*domain.ApprovalWorkflow, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	in.resolveType()
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
	if s := strings.TrimSpace(in.RequestType); s != "" {
		w.RequestType = s
	}
	if in.Mode != "" {
		w.Mode = normaliseMode(in.Mode)
	}
	if in.ExpiresInHours < 0 {
		return nil, domain.NewValidationError("expires_in_hours cannot be negative (0 means the request never expires)")
	}
	w.ExpiresInHours = in.ExpiresInHours
	if in.Enabled != nil {
		w.Enabled = *in.Enabled
	}
	if in.Steps != nil {
		w.Steps = domain.WorkflowStepList(normaliseSteps(in.Steps))
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	// Only the steps this call supplies are checked: stored approvers were
	// checked when they were saved, and an edit that leaves them alone must
	// not fail because someone has since left the organisation.
	if in.Steps != nil {
		if err := checkApprovers(ctx, uc.members, tenantID, []domain.WorkflowStep(w.Steps)); err != nil {
			return nil, err
		}
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
	notifier  ApprovalNotifier
	entities  EntityOwnership
}

// WithEntities lets Execute prove that a risk_acceptance or compliance_control
// request names an entity of the tenant (#1520).
func (uc *SubmitApprovalRequestUseCase) WithEntities(e EntityOwnership) *SubmitApprovalRequestUseCase {
	uc.entities = e
	return uc
}

func NewSubmitApprovalRequestUseCase(w domain.ApprovalWorkflowRepository, r domain.ApprovalRequestRepository) *SubmitApprovalRequestUseCase {
	return &SubmitApprovalRequestUseCase{workflows: w, requests: r}
}
func (uc *SubmitApprovalRequestUseCase) WithRecorder(r *AuditRecorder) *SubmitApprovalRequestUseCase {
	uc.recorder = r
	return uc
}

// WithNotifier tells the approvers that something now awaits them. Without it a
// request sits in an inbox nobody was told to open.
func (uc *SubmitApprovalRequestUseCase) WithNotifier(n ApprovalNotifier) *SubmitApprovalRequestUseCase {
	uc.notifier = n
	return uc
}

func (uc *SubmitApprovalRequestUseCase) Execute(ctx context.Context, tenantID, requesterID uuid.UUID, in SubmitApprovalInput) (*domain.ApprovalRequest, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	entityType := strings.TrimSpace(in.EntityType)
	if entityType == "" {
		return nil, domain.NewValidationError("entity_type is required")
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, domain.NewValidationError("title is required")
	}
	if err := checkEntity(ctx, uc.entities, tenantID, entityType, strings.TrimSpace(in.EntityID)); err != nil {
		return nil, err
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
		RequestType:  wf.RequestType,
		Mode:         normaliseMode(wf.Mode),
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
	// The deadline is frozen from the workflow at submit time, like the steps:
	// shortening the policy tomorrow must not retroactively expire a request
	// someone raised under yesterday's rules.
	if wf.ExpiresInHours > 0 {
		deadline := time.Now().UTC().Add(time.Duration(wf.ExpiresInHours) * time.Hour)
		req.ExpiresAt = &deadline
	}
	if err := uc.requests.CreateRequest(ctx, req); err != nil {
		return nil, err
	}
	if uc.notifier != nil {
		roles, users := PendingAudience(req)
		uc.notifier.NotifyPending(ctx, tenantID, req, roles, users)
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

// The decision engine — eligibility, quorum, sequential vs parallel
// advancement, expiry and the mandatory refusal comment — lives in
// approval_engine_usecases.go (DecideApprovalUseCase), on top of the pure rules
// in domain/approval_engine.go. The older DecideApprovalStepUseCase that lived
// here was removed rather than left beside its replacement: two decision paths
// with different rules is exactly how a control silently stops holding.

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

// checkApprovers proves that every approver_user_ids entry of the steps is a
// member of the tenant (#1520). A value that is not a UUID is a validation
// error. A well-formed id with no membership row in THIS tenant — foreign or
// unknown, indistinguishable — is not found, with the same message for both,
// so the answer is no oracle for whether the user exists elsewhere. A member
// who is deactivated is refused as in delegations. It fails closed when steps
// name approvers and no membership lookup is wired.
func checkApprovers(ctx context.Context, members MemberLookup, tenantID uuid.UUID, steps []domain.WorkflowStep) error {
	seen := map[uuid.UUID]struct{}{}
	var ids []uuid.UUID
	for _, st := range steps {
		for _, raw := range st.ApproverUserIDs {
			id, err := uuid.Parse(raw)
			if err != nil {
				return domain.NewValidationError("approver_user_ids must contain user ids (UUIDs)")
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if members == nil {
		return domain.NewForbiddenError("approvers cannot be verified: no membership lookup is configured")
	}
	for _, id := range ids {
		m, err := members.GetMember(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if m == nil {
			return domain.NewNotFoundError("user", id.String())
		}
		if !m.IsActive {
			return domain.NewValidationError("an approver is deactivated and cannot be named in a workflow")
		}
	}
	return nil
}

// EntityOwnership is the narrow, tenant-scoped port that proves the entity an
// approval request is about belongs to the tenant (#1520). Each method reports
// false (nil error) for an absent or foreign id.
type EntityOwnership interface {
	RiskOwned(ctx context.Context, tenantID, id uuid.UUID) (bool, error)
	ControlOwned(ctx context.Context, tenantID, id uuid.UUID) (bool, error)
}

// Approval entity types whose entity_id names a tenant-owned row. The other
// catalogue types (security_exception, policy, access_request, vendor) have no
// owning table: entity_id there is an opaque label, never joined.
const (
	entityTypeRiskAcceptance    = "risk_acceptance"
	entityTypeComplianceControl = "compliance_control"
)

// checkEntity validates entity_id for the types that map to an owned table.
func checkEntity(ctx context.Context, owner EntityOwnership, tenantID uuid.UUID, entityType, entityID string) error {
	if entityID == "" {
		return nil
	}
	if entityType != entityTypeRiskAcceptance && entityType != entityTypeComplianceControl {
		return nil
	}
	id, err := uuid.Parse(entityID)
	if err != nil {
		return domain.NewValidationError("entity_id must be a UUID for " + entityType)
	}
	if owner == nil {
		return domain.NewForbiddenError("entity cannot be verified: no ownership lookup is configured")
	}
	var ok bool
	resource := "risk"
	if entityType == entityTypeComplianceControl {
		resource = "control"
		ok, err = owner.ControlOwned(ctx, tenantID, id)
	} else {
		ok, err = owner.RiskOwned(ctx, tenantID, id)
	}
	if err != nil {
		return err
	}
	if !ok {
		return domain.NewNotFoundError(resource, id.String())
	}
	return nil
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
		cleaned := make([]string, 0, len(s.ApproverUserIDs))
		for _, id := range s.ApproverUserIDs {
			if id = strings.TrimSpace(id); id != "" {
				cleaned = append(cleaned, id)
			}
		}
		s.ApproverUserIDs = cleaned
		if s.QuorumPercent < 0 || s.QuorumPercent > 100 {
			s.QuorumPercent = 0
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
