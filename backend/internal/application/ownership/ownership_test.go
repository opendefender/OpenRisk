// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package ownership

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

type fakeMembers struct {
	rows []domain.OrganizationMember
	err  error
}

func (f *fakeMembers) ListMembers(_ context.Context, tenantID uuid.UUID) ([]domain.OrganizationMember, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []domain.OrganizationMember{}
	for _, m := range f.rows {
		if m.OrganizationID == tenantID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeMembers) GetMember(_ context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.rows {
		if f.rows[i].OrganizationID == tenantID && f.rows[i].UserID == userID {
			return &f.rows[i], nil
		}
	}
	return nil, nil
}

type sentNotification struct {
	userID   uuid.UUID
	tenantID uuid.UUID
	subject  string
}

type fakeNotifier struct{ sent []sentNotification }

func (f *fakeNotifier) NotifyInApp(userID, tenantID uuid.UUID, _ domain.NotificationType, subject, _ string, _ *uuid.UUID, _ string) error {
	f.sent = append(f.sent, sentNotification{userID, tenantID, subject})
	return nil
}

func member(tenant, user uuid.UUID, role domain.MemberRole, biz domain.BusinessRoleKey, active bool, email, name string) domain.OrganizationMember {
	return domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tenant, UserID: user,
		Role: role, BusinessRole: biz, IsActive: active,
		User: &domain.User{ID: user, Email: email, FullName: name},
	}
}

func TestListAssignable_ExcludesOtherTenantsAndDeactivated(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	alice, bob, ghost, stranger := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	repo := &fakeMembers{rows: []domain.OrganizationMember{
		member(tenant, alice, domain.RoleAdmin, "", true, "alice@x.io", "Alice Martin"),
		member(tenant, bob, domain.RoleUser, domain.BusinessRoleKey("risk_manager"), true, "bob@x.io", "Bob Diallo"),
		member(tenant, ghost, domain.RoleUser, "", false, "ghost@x.io", "Ghost Account"),
		member(other, stranger, domain.RoleAdmin, "", true, "stranger@y.io", "Stranger"),
	}}

	res, err := NewListAssignableUseCase(repo).Execute(context.Background(), tenant, ListAssignableInput{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Users) != 2 {
		t.Fatalf("expected alice+bob, got %d: %+v", len(res.Users), res.Users)
	}
	for _, u := range res.Users {
		if u.UserID == ghost {
			t.Fatal("a deactivated account must never be offered as an assignee")
		}
		if u.UserID == stranger {
			t.Fatal("cross-tenant leak: a member of another organization was offered")
		}
	}
	// Sorted by display name: Alice before Bob.
	if res.Users[0].UserID != alice {
		t.Fatalf("expected alphabetical order, got %+v", res.Users)
	}
	if res.Users[0].Initials != "AM" {
		t.Fatalf("initials = %q, want AM", res.Users[0].Initials)
	}
}

func TestListAssignable_PermissionFilter(t *testing.T) {
	tenant := uuid.New()
	admin, rm := uuid.New(), uuid.New()
	repo := &fakeMembers{rows: []domain.OrganizationMember{
		member(tenant, admin, domain.RoleAdmin, "", true, "admin@x.io", "Ada Admin"),
		// viewer holds read-only permissions, so it cannot be given work that
		// needs risks:update.
		member(tenant, rm, domain.RoleUser, domain.BusinessRoleKey("viewer"), true, "v@x.io", "Vic Viewer"),
	}}
	uc := NewListAssignableUseCase(repo)

	// Soft filter: everyone returned, capability flagged.
	soft, err := uc.Execute(context.Background(), tenant, ListAssignableInput{Permission: "risks:update"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(soft.Users) != 2 {
		t.Fatalf("soft filter must return everyone, got %d", len(soft.Users))
	}
	byID := map[uuid.UUID]Assignee{}
	for _, u := range soft.Users {
		byID[u.UserID] = u
	}
	if !byID[admin].CanAct {
		t.Fatal("admin holds the '*' wildcard and must be able to act")
	}
	if byID[rm].CanAct {
		t.Fatal("a read-only viewer must not be reported as able to update risks")
	}

	// Hard filter: only the capable remain.
	hard, err := uc.Execute(context.Background(), tenant, ListAssignableInput{Permission: "risks:update", OnlyCapable: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(hard.Users) != 1 || hard.Users[0].UserID != admin {
		t.Fatalf("hard filter should keep only the admin, got %+v", hard.Users)
	}
}

func TestListAssignable_SearchAndGroups(t *testing.T) {
	tenant := uuid.New()
	repo := &fakeMembers{rows: []domain.OrganizationMember{
		member(tenant, uuid.New(), domain.RoleAdmin, "", true, "ada@x.io", "Ada Admin"),
		member(tenant, uuid.New(), domain.RoleUser, domain.BusinessRoleKey("rssi"), true, "rss@x.io", "Rita Sow"),
		member(tenant, uuid.New(), domain.RoleUser, domain.BusinessRoleKey("rssi"), true, "rs2@x.io", "Rene Sy"),
	}}
	uc := NewListAssignableUseCase(repo)

	found, err := uc.Execute(context.Background(), tenant, ListAssignableInput{Search: "rita"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(found.Users) != 1 || found.Users[0].Email != "rss@x.io" {
		t.Fatalf("search should match on full name, got %+v", found.Users)
	}

	all, err := uc.Execute(context.Background(), tenant, ListAssignableInput{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var rssi *AssignableGroup
	for i := range all.Groups {
		if all.Groups[i].Key == "rssi" {
			rssi = &all.Groups[i]
		}
	}
	if rssi == nil || rssi.Count != 2 {
		t.Fatalf("expected an rssi group of 2, got %+v", all.Groups)
	}
	if rssi.Label == "" || rssi.Label == "rssi" {
		t.Fatalf("group should carry a human label, got %q", rssi.Label)
	}
}

func TestService_Validate_RejectsNonMembersAndDeactivated(t *testing.T) {
	tenant := uuid.New()
	alice, ghost, outsider := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeMembers{rows: []domain.OrganizationMember{
		member(tenant, alice, domain.RoleUser, "", true, "a@x.io", "Alice"),
		member(tenant, ghost, domain.RoleUser, "", false, "g@x.io", "Ghost"),
	}}
	svc := NewService().WithMembers(repo)

	if err := svc.Validate(context.Background(), tenant, domain.OwnershipPatch{Owner: domain.Assign(alice)}); err != nil {
		t.Fatalf("assigning an active member must be allowed: %v", err)
	}
	if err := svc.Validate(context.Background(), tenant, domain.OwnershipPatch{Assignee: domain.Assign(outsider)}); err == nil {
		t.Fatal("a user outside the tenant must be rejected")
	}
	if err := svc.Validate(context.Background(), tenant, domain.OwnershipPatch{Reviewer: domain.Assign(ghost)}); err == nil {
		t.Fatal("a deactivated member must be rejected")
	}
	// Unassigning names nobody, so there is nothing to validate.
	if err := svc.Validate(context.Background(), tenant, domain.OwnershipPatch{Owner: domain.Unassign()}); err != nil {
		t.Fatalf("clearing a slot must not require a membership check: %v", err)
	}
}

func TestService_Validate_WithoutMemberRepoDegradesOpen(t *testing.T) {
	// No member repository wired: validation degrades to "trust the caller"
	// rather than failing every assignment. Documented contract.
	svc := NewService()
	if err := svc.Validate(context.Background(), uuid.New(), domain.OwnershipPatch{Owner: domain.Assign(uuid.New())}); err != nil {
		t.Fatalf("unwired service must not block assignment: %v", err)
	}
}

func TestService_Apply_ReturnsOnlyRealChanges(t *testing.T) {
	tenant := uuid.New()
	alice, bob := uuid.New(), uuid.New()
	repo := &fakeMembers{rows: []domain.OrganizationMember{
		member(tenant, alice, domain.RoleUser, "", true, "a@x.io", "Alice"),
		member(tenant, bob, domain.RoleUser, "", true, "b@x.io", "Bob"),
	}}
	svc := NewService().WithMembers(repo)

	block := domain.Ownership{OwnerID: &alice}
	changes, err := svc.Apply(context.Background(), tenant, &block,
		domain.OwnershipPatch{Owner: domain.Assign(alice), Assignee: domain.Assign(bob)}, alice)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(changes) != 1 || changes[0].Role != domain.RoleAssignee {
		t.Fatalf("re-setting the same owner must not count as a change: %+v", changes)
	}
	if block.AssigneeID == nil || *block.AssigneeID != bob {
		t.Fatal("patch was not applied to the block")
	}
}

func TestService_Apply_RejectionLeavesBlockUntouched(t *testing.T) {
	tenant := uuid.New()
	alice, outsider := uuid.New(), uuid.New()
	repo := &fakeMembers{rows: []domain.OrganizationMember{
		member(tenant, alice, domain.RoleUser, "", true, "a@x.io", "Alice"),
	}}
	svc := NewService().WithMembers(repo)

	block := domain.Ownership{OwnerID: &alice}
	if _, err := svc.Apply(context.Background(), tenant, &block,
		domain.OwnershipPatch{Owner: domain.Assign(outsider)}, alice); err == nil {
		t.Fatal("expected a rejection")
	}
	if block.OwnerID == nil || *block.OwnerID != alice {
		t.Fatal("a rejected patch must not have mutated the entity")
	}
}

func TestService_Notify_SkipsSelfAssignmentAndUnassignment(t *testing.T) {
	tenant := uuid.New()
	actor, bob := uuid.New(), uuid.New()
	notifier := &fakeNotifier{}
	svc := NewService().WithNotifier(notifier)

	svc.Notify(context.Background(), tenant, []domain.OwnershipChange{
		{Role: domain.RoleOwner, To: &actor, Actor: actor},  // assigning yourself
		{Role: domain.RoleAssignee, To: nil, Actor: actor},  // unassigning
		{Role: domain.RoleReviewer, To: &bob, Actor: actor}, // the only real notice
	}, domain.OwnershipSubject{ResourceType: "risk", Title: "Fuite S3", ResourceID: uuid.New()})

	if len(notifier.sent) != 1 {
		t.Fatalf("expected exactly one notification, got %d: %+v", len(notifier.sent), notifier.sent)
	}
	if notifier.sent[0].userID != bob || notifier.sent[0].tenantID != tenant {
		t.Fatalf("notification went to the wrong place: %+v", notifier.sent[0])
	}
	if notifier.sent[0].subject == "" {
		t.Fatal("notification must carry a subject")
	}
}

func TestService_Notify_WithoutNotifierIsSilentNotFatal(t *testing.T) {
	svc := NewService()
	bob := uuid.New()
	// Must not panic — a deployment with no notification centre still assigns.
	svc.Notify(context.Background(), uuid.New(), []domain.OwnershipChange{{Role: domain.RoleOwner, To: &bob}}, domain.OwnershipSubject{})
}

type fakeLookup struct {
	emails map[uuid.UUID]string
	err    error
}

func (f *fakeLookup) EmailsByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if e, ok := f.emails[id]; ok {
			out[id] = e
		}
	}
	return out, nil
}

func TestService_ResolveEmails_BatchesAndDegrades(t *testing.T) {
	alice, bob := uuid.New(), uuid.New()
	r1 := &domain.Risk{Ownership: domain.Ownership{OwnerID: &alice}}
	r2 := &domain.Risk{Ownership: domain.Ownership{OwnerID: &alice, AssigneeID: &bob}}

	svc := NewService().WithUsers(&fakeLookup{emails: map[uuid.UUID]string{alice: "a@x.io", bob: "b@x.io"}})
	svc.ResolveEmails(context.Background(), r1, r2)

	if r1.OwnerEmail != "a@x.io" || r2.AssigneeEmail != "b@x.io" {
		t.Fatalf("emails not resolved: %q / %q", r1.OwnerEmail, r2.AssigneeEmail)
	}

	// A failing lookup degrades the display, it does not fail the read.
	r3 := &domain.Risk{Ownership: domain.Ownership{OwnerID: &alice}}
	NewService().WithUsers(&fakeLookup{err: errors.New("db down")}).ResolveEmails(context.Background(), r3)
	if r3.OwnerEmail != "" {
		t.Fatal("a failed lookup must leave the email empty, not invent one")
	}
	if r3.OwnerID == nil {
		t.Fatal("the id itself must survive so the UI can still render something")
	}
}
