// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package governance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// In-memory fakes (tenant-scoped, mirror the Gorm repos' contract).
// ---------------------------------------------------------------------------

type fakeDelegationRepo struct {
	items map[uuid.UUID]*domain.Delegation
}

func newFakeDelegationRepo() *fakeDelegationRepo {
	return &fakeDelegationRepo{items: map[uuid.UUID]*domain.Delegation{}}
}
func (f *fakeDelegationRepo) Create(_ context.Context, d *domain.Delegation) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	cp := *d
	f.items[d.ID] = &cp
	return nil
}
func (f *fakeDelegationRepo) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.Delegation, error) {
	d, ok := f.items[id]
	if !ok || d.TenantID != tenantID {
		return nil, nil
	}
	cp := *d
	return &cp, nil
}
func (f *fakeDelegationRepo) List(_ context.Context, tenantID uuid.UUID, flt domain.DelegationFilter) ([]domain.Delegation, error) {
	var out []domain.Delegation
	for _, d := range f.items {
		if d.TenantID != tenantID {
			continue
		}
		if flt.DelegateID != nil && d.DelegateID != *flt.DelegateID {
			continue
		}
		if flt.DelegatorID != nil && d.DelegatorID != *flt.DelegatorID {
			continue
		}
		if flt.ActiveOnly && d.Status != domain.DelegationActive {
			continue
		}
		out = append(out, *d)
	}
	return out, nil
}
func (f *fakeDelegationRepo) Update(_ context.Context, d *domain.Delegation) error {
	cur, ok := f.items[d.ID]
	if !ok || cur.TenantID != d.TenantID {
		return domain.NewNotFoundError("delegation", d.ID)
	}
	cp := *d
	f.items[d.ID] = &cp
	return nil
}

type fakeApprovalRepo struct {
	workflows map[uuid.UUID]*domain.ApprovalWorkflow
	requests  map[uuid.UUID]*domain.ApprovalRequest
}

func newFakeApprovalRepo() *fakeApprovalRepo {
	return &fakeApprovalRepo{
		workflows: map[uuid.UUID]*domain.ApprovalWorkflow{},
		requests:  map[uuid.UUID]*domain.ApprovalRequest{},
	}
}
func (f *fakeApprovalRepo) CreateWorkflow(_ context.Context, w *domain.ApprovalWorkflow) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	cp := *w
	f.workflows[w.ID] = &cp
	return nil
}
func (f *fakeApprovalRepo) GetWorkflowByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ApprovalWorkflow, error) {
	w, ok := f.workflows[id]
	if !ok || w.TenantID != tenantID {
		return nil, nil
	}
	cp := *w
	return &cp, nil
}
func (f *fakeApprovalRepo) ListWorkflows(_ context.Context, tenantID uuid.UUID) ([]domain.ApprovalWorkflow, error) {
	var out []domain.ApprovalWorkflow
	for _, w := range f.workflows {
		if w.TenantID == tenantID {
			out = append(out, *w)
		}
	}
	return out, nil
}
func (f *fakeApprovalRepo) FindWorkflow(_ context.Context, tenantID uuid.UUID, entityType, action string) (*domain.ApprovalWorkflow, error) {
	for _, w := range f.workflows {
		if w.TenantID == tenantID && w.EntityType == entityType && w.Enabled {
			if action == "" || w.Action == action || w.Action == "" {
				cp := *w
				return &cp, nil
			}
		}
	}
	return nil, nil
}
func (f *fakeApprovalRepo) UpdateWorkflow(_ context.Context, w *domain.ApprovalWorkflow) error {
	cur, ok := f.workflows[w.ID]
	if !ok || cur.TenantID != w.TenantID {
		return domain.NewNotFoundError("workflow", w.ID)
	}
	cp := *w
	f.workflows[w.ID] = &cp
	return nil
}
func (f *fakeApprovalRepo) DeleteWorkflow(_ context.Context, id, tenantID uuid.UUID) error {
	w, ok := f.workflows[id]
	if !ok || w.TenantID != tenantID {
		return domain.NewNotFoundError("workflow", id)
	}
	delete(f.workflows, id)
	return nil
}
func (f *fakeApprovalRepo) CreateRequest(_ context.Context, r *domain.ApprovalRequest) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	cp := *r
	f.requests[r.ID] = &cp
	return nil
}
func (f *fakeApprovalRepo) GetRequestByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ApprovalRequest, error) {
	r, ok := f.requests[id]
	if !ok || r.TenantID != tenantID {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}
func (f *fakeApprovalRepo) ListRequests(_ context.Context, tenantID uuid.UUID, flt domain.ApprovalRequestFilter) ([]domain.ApprovalRequest, error) {
	var out []domain.ApprovalRequest
	for _, r := range f.requests {
		if r.TenantID != tenantID {
			continue
		}
		if flt.Status != "" && string(r.Status) != flt.Status {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}
func (f *fakeApprovalRepo) UpdateRequest(_ context.Context, r *domain.ApprovalRequest) error {
	cur, ok := f.requests[r.ID]
	if !ok || cur.TenantID != r.TenantID {
		return domain.NewNotFoundError("approval request", r.ID)
	}
	cp := *r
	f.requests[r.ID] = &cp
	return nil
}

type fakeAuditRepo struct{ events []domain.AuditEvent }

func (f *fakeAuditRepo) Append(_ context.Context, e *domain.AuditEvent) error {
	f.events = append(f.events, *e)
	return nil
}
func (f *fakeAuditRepo) List(_ context.Context, tenantID uuid.UUID, flt domain.AuditEventFilter) ([]domain.AuditEvent, int64, error) {
	var out []domain.AuditEvent
	for _, e := range f.events {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, int64(len(out)), nil
}

// ---------------------------------------------------------------------------
// Delegations
// ---------------------------------------------------------------------------

func TestCreateDelegation_Success(t *testing.T) {
	repo := newFakeDelegationRepo()
	audit := &fakeAuditRepo{}
	uc := NewCreateDelegationUseCase(repo).WithRecorder(NewAuditRecorder(audit)).WithMembers(everyoneIsMember{})
	tenant, actor, delegate := uuid.New(), uuid.New(), uuid.New()
	end := time.Now().Add(48 * time.Hour)

	d, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegateID:  delegate,
		Reason:      "annual leave",
		Permissions: []string{"risks:*", "risks:*"}, // dedup
		EndsAt:      &end,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.DelegatorID != actor || d.DelegateID != delegate {
		t.Fatalf("wrong parties: %+v", d)
	}
	if len(d.Permissions) != 1 {
		t.Fatalf("expected deduped permissions, got %v", d.Permissions)
	}
	if d.Status != domain.DelegationActive {
		t.Fatalf("expected active, got %s", d.Status)
	}
	if len(audit.events) != 1 || audit.events[0].Action != domain.AuditActionDelegate {
		t.Fatalf("expected one delegate audit event, got %+v", audit.events)
	}
}

func TestCreateDelegation_Validation(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewCreateDelegationUseCase(repo).WithMembers(everyoneIsMember{})
	tenant, actor := uuid.New(), uuid.New()
	end := time.Now().Add(24 * time.Hour)

	// self-delegation
	if _, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegateID: actor, Permissions: []string{"*"}, EndsAt: &end,
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for self-delegation, got %v", err)
	}
	// no permissions
	if _, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegateID: uuid.New(), Permissions: nil, EndsAt: &end,
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for empty permissions, got %v", err)
	}
	// missing end date
	if _, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegateID: uuid.New(), Permissions: []string{"*"},
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for missing end date, got %v", err)
	}
	// end before start
	start := time.Now().Add(72 * time.Hour)
	if _, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegateID: uuid.New(), Permissions: []string{"*"}, StartsAt: &start, EndsAt: &end,
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for end<=start, got %v", err)
	}
}

func TestRevokeDelegation_Success_NotFound_AlreadyRevoked(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewRevokeDelegationUseCase(repo)
	tenant, actor := uuid.New(), uuid.New()
	end := time.Now().Add(24 * time.Hour)
	d := &domain.Delegation{TenantID: tenant, DelegatorID: actor, DelegateID: uuid.New(), Status: domain.DelegationActive, EndsAt: end}
	_ = repo.Create(context.Background(), d)

	got, err := uc.Execute(context.Background(), tenant, actor, RevokeDelegationInput{ID: d.ID})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Status != domain.DelegationRevoked || got.RevokedAt == nil {
		t.Fatalf("expected revoked, got %+v", got)
	}
	// NotFound
	if _, err := uc.Execute(context.Background(), tenant, actor, RevokeDelegationInput{ID: uuid.New(), ActorIsAdmin: true}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	// Already revoked → validation
	if _, err := uc.Execute(context.Background(), tenant, actor, RevokeDelegationInput{ID: d.ID}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for double revoke, got %v", err)
	}
}

func TestResolveEffectivePermissions_ActiveWindowUnion(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewResolveEffectivePermissionsUseCase(repo)
	tenant, delegate := uuid.New(), uuid.New()
	now := time.Now()

	// active
	_ = repo.Create(context.Background(), &domain.Delegation{
		TenantID: tenant, DelegatorID: uuid.New(), DelegateID: delegate,
		Status: domain.DelegationActive, Permissions: domain.StringList{"risks:read", "assets:read"},
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	})
	// expired window (active status but past end) → excluded by IsActiveAt
	_ = repo.Create(context.Background(), &domain.Delegation{
		TenantID: tenant, DelegatorID: uuid.New(), DelegateID: delegate,
		Status: domain.DelegationActive, Permissions: domain.StringList{"compliance:write"},
		StartsAt: now.Add(-48 * time.Hour), EndsAt: now.Add(-24 * time.Hour),
	})
	// revoked → excluded (ActiveOnly filter)
	_ = repo.Create(context.Background(), &domain.Delegation{
		TenantID: tenant, DelegatorID: uuid.New(), DelegateID: delegate,
		Status: domain.DelegationRevoked, Permissions: domain.StringList{"admin:*"},
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	})

	perms, err := uc.Execute(context.Background(), tenant, delegate)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("expected 2 effective perms, got %v", perms)
	}
}

// ---------------------------------------------------------------------------
// Approval workflows + Maker-Checker state machine
// ---------------------------------------------------------------------------

func seedTwoStepWorkflow(t *testing.T, repo *fakeApprovalRepo, tenant uuid.UUID) *domain.ApprovalWorkflow {
	t.Helper()
	create := NewCreateWorkflowUseCase(repo)
	w, err := create.Execute(context.Background(), tenant, uuid.New(), WorkflowInput{
		Name:       "Risk acceptance",
		EntityType: "risk_acceptance",
		Action:     "accept",
		Steps: []domain.WorkflowStep{
			{Name: "Asset owner", ApproverRole: "manager"},
			{Name: "CISO sign-off", ApproverRole: "ciso"},
		},
	})
	if err != nil {
		t.Fatalf("seed workflow: %v", err)
	}
	return w
}

func TestCreateWorkflow_Success_Validation_Conflict(t *testing.T) {
	repo := newFakeApprovalRepo()
	tenant := uuid.New()
	seedTwoStepWorkflow(t, repo, tenant)

	// Validation: no steps
	_, err := NewCreateWorkflowUseCase(repo).Execute(context.Background(), tenant, uuid.New(), WorkflowInput{
		Name: "bad", EntityType: "x", Steps: nil,
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for no steps, got %v", err)
	}
	// Conflict: same (entity_type, action)
	_, err = NewCreateWorkflowUseCase(repo).Execute(context.Background(), tenant, uuid.New(), WorkflowInput{
		Name: "dup", EntityType: "risk_acceptance", Action: "accept",
		Steps: []domain.WorkflowStep{{Name: "s", ApproverRole: "manager"}},
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestSubmitApproval_Success_NoWorkflow(t *testing.T) {
	repo := newFakeApprovalRepo()
	tenant, requester := uuid.New(), uuid.New()
	seedTwoStepWorkflow(t, repo, tenant)
	submit := NewSubmitApprovalRequestUseCase(repo, repo)

	req, err := submit.Execute(context.Background(), tenant, requester, SubmitApprovalInput{
		EntityType: "risk_acceptance", Action: "accept", EntityID: "risk-1", Title: "Accept Log4Shell residual risk",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if req.Status != domain.ApprovalPending || req.CurrentStep != 0 || len(req.Steps) != 2 {
		t.Fatalf("bad initial request state: %+v", req)
	}
	// No workflow configured for a different action → validation
	if _, err := submit.Execute(context.Background(), tenant, requester, SubmitApprovalInput{
		EntityType: "budget", Action: "approve", Title: "x",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation when no workflow, got %v", err)
	}
}

// The core control: a full two-step approve walks pending → approved, enforcing
// four-eyes, role eligibility and no double-signing.
func TestDecideApproval_FullChain_And_Guards(t *testing.T) {
	repo := newFakeApprovalRepo()
	tenant, requester := uuid.New(), uuid.New()
	seedTwoStepWorkflow(t, repo, tenant)
	submit := NewSubmitApprovalRequestUseCase(repo, repo)
	decide := NewDecideApprovalUseCase(repo)

	req, _ := submit.Execute(context.Background(), tenant, requester, SubmitApprovalInput{
		EntityType: "risk_acceptance", Action: "accept", EntityID: "risk-1", Title: "Accept residual risk",
	})

	// Four-eyes: the maker cannot check their own request.
	if _, err := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: requester, Roles: []string{"manager"}},
		DecideInput{Decision: "approve"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected four-eyes forbidden, got %v", err)
	}

	// Role not eligible for step 1 (needs "manager").
	if _, err := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: uuid.New(), Roles: []string{"viewer"}},
		DecideInput{Decision: "approve"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected role-ineligible forbidden, got %v", err)
	}

	// Step 1 approved by a manager → advances to step 2, still pending.
	manager := uuid.New()
	got, err := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: manager, Roles: []string{"manager"}},
		DecideInput{Decision: "approve", Comment: "owner ok"})
	if err != nil {
		t.Fatalf("step1 approve: %v", err)
	}
	if got.Status != domain.ApprovalPending || got.CurrentStep != 1 {
		t.Fatalf("expected advance to step 1 pending, got status=%s step=%d", got.Status, got.CurrentStep)
	}

	// Step 2 approved by admin (admin satisfies any role) → fully approved.
	ciso := uuid.New()
	got, err = decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: ciso, IsAdmin: true},
		DecideInput{Decision: "approve"})
	if err != nil {
		t.Fatalf("step2 approve: %v", err)
	}
	if got.Status != domain.ApprovalApproved || got.ResolvedAt == nil {
		t.Fatalf("expected approved+resolved, got %+v", got)
	}

	// Deciding again on a resolved request → validation.
	if _, err := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: uuid.New(), IsAdmin: true},
		DecideInput{Decision: "approve"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation on resolved request, got %v", err)
	}
}

func TestDecideApproval_Reject_And_NotFound(t *testing.T) {
	repo := newFakeApprovalRepo()
	tenant, requester := uuid.New(), uuid.New()
	seedTwoStepWorkflow(t, repo, tenant)
	submit := NewSubmitApprovalRequestUseCase(repo, repo)
	decide := NewDecideApprovalUseCase(repo)

	req, _ := submit.Execute(context.Background(), tenant, requester, SubmitApprovalInput{
		EntityType: "risk_acceptance", Action: "accept", Title: "x",
	})

	got, err := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: uuid.New(), Roles: []string{"manager"}},
		DecideInput{Decision: "reject", Comment: "not acceptable"})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if got.Status != domain.ApprovalRejected || got.ResolvedAt == nil {
		t.Fatalf("expected rejected, got %+v", got)
	}

	// NotFound
	if _, err := decide.Execute(context.Background(), tenant, uuid.New(),
		ApproverIdentity{UserID: uuid.New(), IsAdmin: true},
		DecideInput{Decision: "approve"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDecideApproval_MinApprovalsGate(t *testing.T) {
	repo := newFakeApprovalRepo()
	tenant, requester := uuid.New(), uuid.New()
	// single step requiring TWO distinct manager approvals
	_, err := NewCreateWorkflowUseCase(repo).Execute(context.Background(), tenant, uuid.New(), WorkflowInput{
		Name: "Two managers", EntityType: "payment", Action: "release",
		Steps: []domain.WorkflowStep{{Name: "Dual control", ApproverRole: "manager", MinApprovals: 2}},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	submit := NewSubmitApprovalRequestUseCase(repo, repo)
	decide := NewDecideApprovalUseCase(repo)
	req, _ := submit.Execute(context.Background(), tenant, requester, SubmitApprovalInput{
		EntityType: "payment", Action: "release", Title: "Release 5M FCFA",
	})

	m1 := uuid.New()
	got, _ := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: m1, Roles: []string{"manager"}}, DecideInput{Decision: "approve"})
	if got.Status != domain.ApprovalPending {
		t.Fatalf("expected still pending after 1/2 approvals, got %s", got.Status)
	}
	// Same approver again → cannot double-sign. This is an eligibility refusal
	// (403), not a malformed request: the person is simply not allowed to be the
	// second signature on a step they already signed.
	if _, err := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: m1, Roles: []string{"manager"}}, DecideInput{Decision: "approve"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected double-sign to be forbidden, got %v", err)
	}
	// second distinct manager → satisfies the gate → approved
	got, _ = decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: uuid.New(), Roles: []string{"manager"}}, DecideInput{Decision: "approve"})
	if got.Status != domain.ApprovalApproved {
		t.Fatalf("expected approved after 2/2, got %s", got.Status)
	}
}

// ---------------------------------------------------------------------------
// Audit trail list + recorder nil-safety
// ---------------------------------------------------------------------------

func TestListAuditEvents_Success(t *testing.T) {
	audit := &fakeAuditRepo{}
	tenant, actor := uuid.New(), uuid.New()
	_ = audit.Append(context.Background(), &domain.AuditEvent{TenantID: tenant, ActorID: &actor, Action: domain.AuditActionCreate, EntityType: "asset"})
	uc := NewListAuditEventsUseCase(audit)
	res, err := uc.Execute(context.Background(), tenant, domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.Total != 1 || len(res.Events) != 1 {
		t.Fatalf("expected one event, got %+v", res)
	}
}

func TestAuditRecorder_NilSafe(t *testing.T) {
	// A nil recorder / nil repo must never panic.
	var r *AuditRecorder
	r.Record(context.Background(), domain.AuditEvent{TenantID: uuid.New()})
	NewAuditRecorder(nil).Record(context.Background(), domain.AuditEvent{TenantID: uuid.New()})
	// Zero tenant is dropped.
	NewAuditRecorder(&fakeAuditRepo{}).Record(context.Background(), domain.AuditEvent{})
}

// ---------------------------------------------------------------------------
// #529 — delegation authority.
//
// POST /governance/delegations carries no route guard on purpose: lending your
// own rights while you are away is every member's business. What was missing is
// the check that the rights being lent are YOURS. Without it, any authenticated
// member could post {delegator_id: <an admin>, delegate_id: <themselves>,
// permissions: ["*"]} and DecideApproval would then layer the admin's roles
// onto them (resolveApprover → domain.CanSign) — a maker-checker bypass by
// anyone who can log in.
// ---------------------------------------------------------------------------

func TestCreateDelegation_Unauthorized_ForeignDelegator(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewCreateDelegationUseCase(repo).WithMembers(everyoneIsMember{})
	tenant, attacker, victim := uuid.New(), uuid.New(), uuid.New()
	end := time.Now().Add(24 * time.Hour)

	_, err := uc.Execute(context.Background(), tenant, attacker, CreateDelegationInput{
		DelegatorID: victim, // somebody else's authority
		DelegateID:  attacker,
		Permissions: []string{"*"},
		EndsAt:      &end,
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when naming another delegator, got %v", err)
	}
	if len(repo.items) != 0 {
		t.Fatalf("a refused delegation must not be persisted, found %d", len(repo.items))
	}
}

func TestCreateDelegation_AdminMayActForAnother(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewCreateDelegationUseCase(repo).WithMembers(everyoneIsMember{})
	tenant, admin, absent, cover := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	end := time.Now().Add(24 * time.Hour)

	d, err := uc.Execute(context.Background(), tenant, admin, CreateDelegationInput{
		DelegatorID:  absent,
		DelegateID:   cover,
		Permissions:  []string{"risks:update"},
		EndsAt:       &end,
		ActorIsAdmin: true,
	})
	if err != nil {
		t.Fatalf("an org admin may record a delegation for a colleague: %v", err)
	}
	if d.DelegatorID != absent || d.CreatedBy != admin {
		t.Fatalf("delegator must be the named user and CreatedBy the admin, got %+v", d)
	}
}

func TestCreateDelegation_OwnRightsNeedNoAdmin(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewCreateDelegationUseCase(repo).WithMembers(everyoneIsMember{})
	tenant, actor := uuid.New(), uuid.New()
	end := time.Now().Add(24 * time.Hour)

	// Both spellings of "my own rights": the field omitted, and the field set to
	// the caller. Neither may require admin, or the feature is unusable.
	if _, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegateID: uuid.New(), Permissions: []string{"risks:read"}, EndsAt: &end,
	}); err != nil {
		t.Fatalf("implicit self-delegation must be allowed: %v", err)
	}
	if _, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegatorID: actor, DelegateID: uuid.New(), Permissions: []string{"risks:read"}, EndsAt: &end,
	}); err != nil {
		t.Fatalf("explicit self-delegation must be allowed: %v", err)
	}
}

func TestRevokeDelegation_Unauthorized_Bystander(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewRevokeDelegationUseCase(repo)
	tenant := uuid.New()
	delegator, delegate, bystander := uuid.New(), uuid.New(), uuid.New()
	end := time.Now().Add(24 * time.Hour)

	newRow := func() *domain.Delegation {
		d := &domain.Delegation{
			TenantID: tenant, DelegatorID: delegator, DelegateID: delegate,
			Status: domain.DelegationActive, EndsAt: end,
		}
		_ = repo.Create(context.Background(), d)
		return d
	}

	// An unrelated member of the same tenant may not end the grant.
	d := newRow()
	if _, err := uc.Execute(context.Background(), tenant, bystander, RevokeDelegationInput{ID: d.ID}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for a bystander, got %v", err)
	}
	if got, _ := repo.GetByID(context.Background(), d.ID, tenant); got.Status != domain.DelegationActive {
		t.Fatalf("a refused revoke must leave the row active, got %s", got.Status)
	}

	// The three parties who may: delegator, delegate, org admin.
	for name, call := range map[string]func() error{
		"delegator": func() error {
			_, err := uc.Execute(context.Background(), tenant, delegator, RevokeDelegationInput{ID: newRow().ID})
			return err
		},
		"delegate": func() error {
			_, err := uc.Execute(context.Background(), tenant, delegate, RevokeDelegationInput{ID: newRow().ID})
			return err
		},
		"admin": func() error {
			_, err := uc.Execute(context.Background(), tenant, bystander, RevokeDelegationInput{ID: newRow().ID, ActorIsAdmin: true})
			return err
		},
	} {
		if err := call(); err != nil {
			t.Fatalf("%s must be able to revoke: %v", name, err)
		}
	}
}

// fakeMembers answers GetMember for the users it was given, per tenant.
type fakeMembers struct{ in map[[2]uuid.UUID]bool }

func (f fakeMembers) GetMember(_ context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	if f.in[[2]uuid.UUID{tenantID, userID}] {
		return &domain.OrganizationMember{IsActive: true}, nil
	}
	return nil, nil
}

// everyoneIsMember answers every user as an active member, for the tests that
// are not about membership.
type everyoneIsMember struct{}

func (everyoneIsMember) GetMember(_ context.Context, _, _ uuid.UUID) (*domain.OrganizationMember, error) {
	return &domain.OrganizationMember{IsActive: true}, nil
}

// TestCreateDelegation_RejectsDelegateOutsideTenant: a delegate from another
// organisation reads back as not found, with the same error as an unknown id.
func TestCreateDelegation_RejectsDelegateOutsideTenant(t *testing.T) {
	repo := newFakeDelegationRepo()
	tenant, otherTenant, actor, member, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	members := fakeMembers{in: map[[2]uuid.UUID]bool{{tenant, member}: true, {otherTenant, foreign}: true}}
	uc := NewCreateDelegationUseCase(repo).WithMembers(members)
	end := time.Now().Add(24 * time.Hour)

	// Success: a member of the tenant.
	if _, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
		DelegateID: member, Permissions: []string{"risks:read"}, EndsAt: &end,
	}); err != nil {
		t.Fatalf("a member must be accepted: %v", err)
	}
	// NotFound: a user of another tenant, and a random id, answer the same.
	for name, id := range map[string]uuid.UUID{"foreign": foreign, "random": uuid.New()} {
		_, err := uc.Execute(context.Background(), tenant, actor, CreateDelegationInput{
			DelegateID: id, Permissions: []string{"risks:read"}, EndsAt: &end,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("%s delegate: want ErrNotFound, got %v", name, err)
		}
	}
	// Unauthorized: no actor at all is refused before any lookup.
	if _, err := uc.Execute(context.Background(), tenant, uuid.Nil, CreateDelegationInput{
		DelegateID: member, Permissions: []string{"risks:read"}, EndsAt: &end,
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("a missing actor must be refused, got %v", err)
	}
}

// TestCreateDelegation_RejectsDelegatorOutsideTenant: an admin naming a
// delegator from another organisation gets not found, the same as for an
// unknown id, and nothing is stored (#1319, GHSA-39cg-g3rv-985p).
func TestCreateDelegation_RejectsDelegatorOutsideTenant(t *testing.T) {
	repo := newFakeDelegationRepo()
	tenant, otherTenant, admin, colleague, cover, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	members := fakeMembers{in: map[[2]uuid.UUID]bool{
		{tenant, admin}: true, {tenant, colleague}: true, {tenant, cover}: true, {otherTenant, foreign}: true,
	}}
	uc := NewCreateDelegationUseCase(repo).WithMembers(members)
	end := time.Now().Add(24 * time.Hour)

	// Success: a colleague of the same tenant.
	if _, err := uc.Execute(context.Background(), tenant, admin, CreateDelegationInput{
		DelegatorID: colleague, DelegateID: cover, Permissions: []string{"risks:read"}, EndsAt: &end, ActorIsAdmin: true,
	}); err != nil {
		t.Fatalf("a colleague delegator must be accepted: %v", err)
	}
	// NotFound: a foreign and a random delegator answer the same.
	for name, id := range map[string]uuid.UUID{"foreign": foreign, "random": uuid.New()} {
		_, err := uc.Execute(context.Background(), tenant, admin, CreateDelegationInput{
			DelegatorID: id, DelegateID: cover, Permissions: []string{"risks:read"}, EndsAt: &end, ActorIsAdmin: true,
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("%s delegator: want ErrNotFound, got %v", name, err)
		}
	}
	if len(repo.items) != 1 {
		t.Fatalf("refused delegations must not be stored, found %d", len(repo.items))
	}
	// Unauthorized: no membership lookup wired means no delegation at all.
	if _, err := NewCreateDelegationUseCase(repo).Execute(context.Background(), tenant, admin, CreateDelegationInput{
		DelegateID: cover, Permissions: []string{"risks:read"}, EndsAt: &end,
	}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("an unwired membership lookup must fail closed, got %v", err)
	}
}

// inactiveMember answers every user as a deactivated member.
type inactiveMember struct{}

func (inactiveMember) GetMember(_ context.Context, _, _ uuid.UUID) (*domain.OrganizationMember, error) {
	return &domain.OrganizationMember{IsActive: false}, nil
}

func TestCreateDelegation_RejectsDeactivatedDelegate(t *testing.T) {
	repo := newFakeDelegationRepo()
	uc := NewCreateDelegationUseCase(repo).WithMembers(inactiveMember{})
	end := time.Now().Add(24 * time.Hour)
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), CreateDelegationInput{
		DelegateID: uuid.New(), Permissions: []string{"risks:read"}, EndsAt: &end,
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("a deactivated delegate must be refused, got %v", err)
	}
}

// fakeEmails resolves every id to an address and records what it was asked.
type fakeEmails struct{ asked []uuid.UUID }

func (f *fakeEmails) EmailsByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.asked = append(f.asked, ids...)
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		out[id] = id.String() + "@example.test"
	}
	return out, nil
}

// TestListDelegations_HidesForeignEmails: a delegation stored before the
// create-time check, naming a foreign user, lists without that user's e-mail;
// a member's e-mail still resolves (#1319).
func TestListDelegations_HidesForeignEmails(t *testing.T) {
	repo := newFakeDelegationRepo()
	tenant, otherTenant, member, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	end := time.Now().Add(24 * time.Hour)
	_ = repo.Create(context.Background(), &domain.Delegation{
		ID: uuid.New(), TenantID: tenant, DelegatorID: member, DelegateID: foreign,
		Permissions: []string{"risks:read"}, StartsAt: time.Now().Add(-time.Hour), EndsAt: end, Status: domain.DelegationActive,
	})
	emails := &fakeEmails{}
	members := fakeMembers{in: map[[2]uuid.UUID]bool{{tenant, member}: true, {otherTenant, foreign}: true}}

	list, err := NewListDelegationsUseCase(repo).WithUserLookup(emails).WithMembers(members).Execute(context.Background(), tenant, domain.DelegationFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 delegation, got %d", len(list))
	}
	if list[0].DelegatorEmail == "" {
		t.Fatal("the member's e-mail must still resolve")
	}
	if list[0].DelegateEmail != "" {
		t.Fatalf("a foreign user's e-mail leaked: %q", list[0].DelegateEmail)
	}
	for _, id := range emails.asked {
		if id == foreign {
			t.Fatal("the foreign id was sent to the global e-mail lookup")
		}
	}
	// Unauthorized: no membership lookup means no e-mail resolution at all.
	emails2 := &fakeEmails{}
	list, _ = NewListDelegationsUseCase(repo).WithUserLookup(emails2).Execute(context.Background(), tenant, domain.DelegationFilter{})
	if len(emails2.asked) != 0 || list[0].DelegatorEmail != "" {
		t.Fatal("without a membership lookup no e-mail may be resolved")
	}
}
