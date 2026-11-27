// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package governance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// #1520: ids in the body of an approval request or workflow must belong to the
// caller's tenant. Foreign and unknown ids answer the same not-found.

// fakeEntities knows which (tenant, id) pairs are risks or controls.
type fakeEntities struct {
	risks, controls map[[2]uuid.UUID]bool
}

func (f fakeEntities) RiskOwned(_ context.Context, tenantID, id uuid.UUID) (bool, error) {
	return f.risks[[2]uuid.UUID{tenantID, id}], nil
}
func (f fakeEntities) ControlOwned(_ context.Context, tenantID, id uuid.UUID) (bool, error) {
	return f.controls[[2]uuid.UUID{tenantID, id}], nil
}

// ownershipFixture: tenant owns risk+control; other tenant owns foreignRisk+foreignControl.
type ownershipFixture struct {
	repo                        *fakeApprovalRepo
	tenant, other               uuid.UUID
	risk, control               uuid.UUID
	foreignRisk, foreignControl uuid.UUID
	submit                      *SubmitApprovalRequestUseCase
}

func newOwnershipFixture(t *testing.T) ownershipFixture {
	t.Helper()
	f := ownershipFixture{
		repo: newFakeApprovalRepo(), tenant: uuid.New(), other: uuid.New(),
		risk: uuid.New(), control: uuid.New(), foreignRisk: uuid.New(), foreignControl: uuid.New(),
	}
	seedTwoStepWorkflow(t, f.repo, f.tenant)
	_, err := NewCreateWorkflowUseCase(f.repo).Execute(context.Background(), f.tenant, uuid.New(), WorkflowInput{
		Name: "Waiver", EntityType: "compliance_control", Action: "waive",
		Steps: []domain.WorkflowStep{{Name: "s", ApproverRole: "manager"}},
	})
	if err != nil {
		t.Fatalf("seed waiver workflow: %v", err)
	}
	_, err = NewCreateWorkflowUseCase(f.repo).Execute(context.Background(), f.tenant, uuid.New(), WorkflowInput{
		Name: "Vendor", EntityType: "vendor", Action: "onboard",
		Steps: []domain.WorkflowStep{{Name: "s", ApproverRole: "manager"}},
	})
	if err != nil {
		t.Fatalf("seed vendor workflow: %v", err)
	}
	f.submit = NewSubmitApprovalRequestUseCase(f.repo, f.repo).WithEntities(fakeEntities{
		risks:    map[[2]uuid.UUID]bool{{f.tenant, f.risk}: true, {f.other, f.foreignRisk}: true},
		controls: map[[2]uuid.UUID]bool{{f.tenant, f.control}: true, {f.other, f.foreignControl}: true},
	})
	return f
}

func TestSubmitApproval_OwnEntities_Success(t *testing.T) {
	f := newOwnershipFixture(t)
	cases := []struct{ name, typ, action, entityID string }{
		{"own risk", "risk_acceptance", "accept", f.risk.String()},
		{"own control", "compliance_control", "waive", f.control.String()},
		{"empty entity id stays accepted", "risk_acceptance", "accept", ""},
		// Types with no owning table: entity_id is an opaque label, never joined.
		{"opaque vendor label", "vendor", "onboard", "acme-supplier-42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := f.submit.Execute(context.Background(), f.tenant, uuid.New(), SubmitApprovalInput{
				EntityType: tc.typ, Action: tc.action, EntityID: tc.entityID, Title: "t",
			})
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if req.EntityID != tc.entityID {
				t.Fatalf("entity id changed: %q", req.EntityID)
			}
		})
	}
}

func TestSubmitApproval_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	f := newOwnershipFixture(t)
	for _, tc := range []struct {
		typ, action string
		foreign     uuid.UUID
	}{
		{"risk_acceptance", "accept", f.foreignRisk},
		{"compliance_control", "waive", f.foreignControl},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			before := len(f.repo.requests)
			submit := func(id uuid.UUID) error {
				_, err := f.submit.Execute(context.Background(), f.tenant, uuid.New(), SubmitApprovalInput{
					EntityType: tc.typ, Action: tc.action, EntityID: id.String(), Title: "t",
				})
				return err
			}
			errForeign, errUnknown := submit(tc.foreign), submit(uuid.New())
			if !errors.Is(errForeign, domain.ErrNotFound) || !errors.Is(errUnknown, domain.ErrNotFound) {
				t.Fatalf("expected not found twice, got %v / %v", errForeign, errUnknown)
			}
			if domain.MessageFromError(errForeign) != domain.MessageFromError(errUnknown) ||
				domain.HTTPStatusFromError(errForeign) != domain.HTTPStatusFromError(errUnknown) {
				t.Fatalf("foreign and unknown must answer alike: %v / %v", errForeign, errUnknown)
			}
			if len(f.repo.requests) != before {
				t.Fatal("a refused request must not be stored")
			}
		})
	}
	// Malformed is a validation error, not a not-found.
	_, err := f.submit.Execute(context.Background(), f.tenant, uuid.New(), SubmitApprovalInput{
		EntityType: "risk_acceptance", Action: "accept", EntityID: "not-a-uuid", Title: "t",
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for a malformed id, got %v", err)
	}
}

func TestSubmitApproval_Unauthorized_NoTenantOrNoLookup(t *testing.T) {
	f := newOwnershipFixture(t)
	_, err := f.submit.Execute(context.Background(), uuid.Nil, uuid.New(), SubmitApprovalInput{
		EntityType: "risk_acceptance", Action: "accept", EntityID: f.risk.String(), Title: "t",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for no tenant, got %v", err)
	}
	// Fail closed: a validated type with no ownership lookup wired is refused.
	bare := NewSubmitApprovalRequestUseCase(f.repo, f.repo)
	_, err = bare.Execute(context.Background(), f.tenant, uuid.New(), SubmitApprovalInput{
		EntityType: "risk_acceptance", Action: "accept", EntityID: f.risk.String(), Title: "t",
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden without a lookup, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Workflow approvers
// ---------------------------------------------------------------------------

type approverFixture struct {
	repo                  *fakeApprovalRepo
	tenant, other         uuid.UUID
	member, foreign, gone uuid.UUID
	members               MemberLookup
}

type approverDirectory map[[2]uuid.UUID]bool // value: is active

func (d approverDirectory) GetMember(_ context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	active, ok := d[[2]uuid.UUID{tenantID, userID}]
	if !ok {
		return nil, nil
	}
	return &domain.OrganizationMember{IsActive: active}, nil
}

func newApproverFixture() approverFixture {
	f := approverFixture{
		repo: newFakeApprovalRepo(), tenant: uuid.New(), other: uuid.New(),
		member: uuid.New(), foreign: uuid.New(), gone: uuid.New(),
	}
	f.members = approverDirectory{
		{f.tenant, f.member}: true,
		{f.tenant, f.gone}:   false,
		{f.other, f.foreign}: true,
	}
	return f
}

func approverInput(typ string, ids ...string) WorkflowInput {
	return WorkflowInput{
		Name: "wf " + typ, EntityType: typ, Action: "grant",
		Steps: []domain.WorkflowStep{{Name: "s", ApproverUserIDs: ids}},
	}
}

func TestCreateWorkflow_Approvers_Success(t *testing.T) {
	f := newApproverFixture()
	uc := NewCreateWorkflowUseCase(f.repo).WithMembers(f.members)
	// A member of the tenant, named twice, is accepted; so is a role-only step.
	w, err := uc.Execute(context.Background(), f.tenant, uuid.New(), approverInput("a", f.member.String(), f.member.String()))
	if err != nil || w == nil {
		t.Fatalf("unexpected: %v", err)
	}
	if _, err := uc.Execute(context.Background(), f.tenant, uuid.New(), WorkflowInput{
		Name: "roles", EntityType: "b", Action: "grant", Steps: []domain.WorkflowStep{{Name: "s", ApproverRole: "admin"}},
	}); err != nil {
		t.Fatalf("a role-only step needs no lookup: %v", err)
	}
}

func TestCreateWorkflow_Approvers_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	f := newApproverFixture()
	uc := NewCreateWorkflowUseCase(f.repo).WithMembers(f.members)
	errForeign := func() error {
		_, err := uc.Execute(context.Background(), f.tenant, uuid.New(), approverInput("c", f.member.String(), f.foreign.String()))
		return err
	}()
	_, errUnknown := uc.Execute(context.Background(), f.tenant, uuid.New(), approverInput("c", uuid.NewString()))
	if !errors.Is(errForeign, domain.ErrNotFound) || !errors.Is(errUnknown, domain.ErrNotFound) {
		t.Fatalf("expected not found twice, got %v / %v", errForeign, errUnknown)
	}
	if domain.MessageFromError(errForeign) != domain.MessageFromError(errUnknown) ||
		domain.HTTPStatusFromError(errForeign) != domain.HTTPStatusFromError(errUnknown) {
		t.Fatalf("foreign and unknown must answer alike: %v / %v", errForeign, errUnknown)
	}
	if len(f.repo.workflows) != 0 {
		t.Fatal("a refused workflow must not be stored")
	}
	// Deactivated member and malformed id are validation errors.
	if _, err := uc.Execute(context.Background(), f.tenant, uuid.New(), approverInput("c", f.gone.String())); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for a deactivated approver, got %v", err)
	}
	if _, err := uc.Execute(context.Background(), f.tenant, uuid.New(), approverInput("c", "nope")); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for a malformed approver id, got %v", err)
	}
}

func TestCreateWorkflow_Approvers_Unauthorized(t *testing.T) {
	f := newApproverFixture()
	uc := NewCreateWorkflowUseCase(f.repo).WithMembers(f.members)
	if _, err := uc.Execute(context.Background(), uuid.Nil, uuid.New(), approverInput("d", f.member.String())); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for no tenant, got %v", err)
	}
	// Fail closed when approvers are named and no membership lookup is wired.
	bare := NewCreateWorkflowUseCase(f.repo)
	if _, err := bare.Execute(context.Background(), f.tenant, uuid.New(), approverInput("d", f.member.String())); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden without a lookup, got %v", err)
	}
}

func seedOwnWorkflow(t *testing.T, f approverFixture) *domain.ApprovalWorkflow {
	t.Helper()
	w, err := NewCreateWorkflowUseCase(f.repo).WithMembers(f.members).Execute(context.Background(), f.tenant, uuid.New(), WorkflowInput{
		Name: "orig", EntityType: "policy", Action: "publish", Steps: []domain.WorkflowStep{{Name: "s", ApproverRole: "admin"}},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return w
}

func TestUpdateWorkflow_Approvers_Success(t *testing.T) {
	f := newApproverFixture()
	w := seedOwnWorkflow(t, f)
	uc := NewUpdateWorkflowUseCase(f.repo).WithMembers(f.members)
	got, err := uc.Execute(context.Background(), f.tenant, w.ID, WorkflowInput{
		Steps: []domain.WorkflowStep{{Name: "s", ApproverUserIDs: []string{f.member.String()}}},
	})
	if err != nil || len(got.Steps) != 1 || got.Steps[0].ApproverUserIDs[0] != f.member.String() {
		t.Fatalf("unexpected: %v %+v", err, got)
	}
	// An edit that leaves the steps alone does no approver lookup at all.
	if _, err := NewUpdateWorkflowUseCase(f.repo).Execute(context.Background(), f.tenant, w.ID, WorkflowInput{Name: "renamed"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestUpdateWorkflow_NotFound_ForeignAndUnknownApproverAlike(t *testing.T) {
	f := newApproverFixture()
	w := seedOwnWorkflow(t, f)
	uc := NewUpdateWorkflowUseCase(f.repo).WithMembers(f.members)
	put := func(id uuid.UUID) error {
		_, err := uc.Execute(context.Background(), f.tenant, w.ID, WorkflowInput{
			Steps: []domain.WorkflowStep{{Name: "s", ApproverUserIDs: []string{id.String()}}},
		})
		return err
	}
	errForeign, errUnknown := put(f.foreign), put(uuid.New())
	if !errors.Is(errForeign, domain.ErrNotFound) || !errors.Is(errUnknown, domain.ErrNotFound) {
		t.Fatalf("expected not found twice, got %v / %v", errForeign, errUnknown)
	}
	if domain.MessageFromError(errForeign) != domain.MessageFromError(errUnknown) {
		t.Fatalf("foreign and unknown must answer alike: %v / %v", errForeign, errUnknown)
	}
	stored, _ := f.repo.GetWorkflowByID(context.Background(), w.ID, f.tenant)
	if len(stored.Steps[0].ApproverUserIDs) != 0 {
		t.Fatal("a refused update must not change the stored steps")
	}
}

func TestUpdateWorkflow_Unauthorized(t *testing.T) {
	f := newApproverFixture()
	w := seedOwnWorkflow(t, f)
	uc := NewUpdateWorkflowUseCase(f.repo).WithMembers(f.members)
	if _, err := uc.Execute(context.Background(), uuid.Nil, w.ID, WorkflowInput{Name: "x"}); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for no tenant, got %v", err)
	}
}
