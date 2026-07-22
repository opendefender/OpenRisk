// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// fakeMemberRepo is an in-memory, tenant-scoped MemberRepository for tests.
type fakeMemberRepo struct {
	members map[string]*domain.OrganizationMember // key = tenant|user
	updated *domain.OrganizationMember
}

func key(t, u uuid.UUID) string { return t.String() + "|" + u.String() }

func (r *fakeMemberRepo) GetMember(_ context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	return r.members[key(tenantID, userID)], nil
}
func (r *fakeMemberRepo) ListMembers(_ context.Context, tenantID uuid.UUID) ([]domain.OrganizationMember, error) {
	var out []domain.OrganizationMember
	for _, m := range r.members {
		if m.OrganizationID == tenantID {
			out = append(out, *m)
		}
	}
	return out, nil
}
func (r *fakeMemberRepo) UpdateMember(_ context.Context, m *domain.OrganizationMember) error {
	r.updated = m
	r.members[key(m.OrganizationID, m.UserID)] = m
	return nil
}

func newFixture() (*fakeMemberRepo, uuid.UUID, uuid.UUID) {
	tenant := uuid.New()
	user := uuid.New()
	repo := &fakeMemberRepo{members: map[string]*domain.OrganizationMember{}}
	repo.members[key(tenant, user)] = &domain.OrganizationMember{
		ID:             uuid.New(),
		OrganizationID: tenant,
		UserID:         user,
		Role:           domain.RoleUser,
		IsActive:       true,
		User:           &domain.User{Email: "analyst@acme.io", FullName: "A Nalyst"},
	}
	return repo, tenant, user
}

func TestAssignBusinessRole_Success(t *testing.T) {
	repo, tenant, user := newFixture()
	uc := NewAssignBusinessRoleUseCase(repo)

	view, err := uc.Execute(context.Background(), tenant, AssignBusinessRoleInput{
		TargetUserID: user,
		BusinessRole: domain.BusinessRoleRSSI,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.BusinessRole != domain.BusinessRoleRSSI {
		t.Fatalf("business role not set, got %q", view.BusinessRole)
	}
	if !hasStr(view.Permissions, "vulnerabilities:read") {
		t.Fatalf("expected RSSI permissions resolved, got %v", view.Permissions)
	}
	if repo.updated == nil || repo.updated.BusinessRole != domain.BusinessRoleRSSI {
		t.Fatal("repository was not updated")
	}
}

func TestAssignBusinessRole_UnknownRole(t *testing.T) {
	repo, tenant, user := newFixture()
	uc := NewAssignBusinessRoleUseCase(repo)
	_, err := uc.Execute(context.Background(), tenant, AssignBusinessRoleInput{
		TargetUserID: user, BusinessRole: "wizard",
	})
	assertValidation(t, err)
}

func TestAssignBusinessRole_NotFound(t *testing.T) {
	repo, tenant, _ := newFixture()
	uc := NewAssignBusinessRoleUseCase(repo)
	_, err := uc.Execute(context.Background(), tenant, AssignBusinessRoleInput{
		TargetUserID: uuid.New(), BusinessRole: domain.BusinessRoleViewer,
	})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Err != domain.ErrNotFound {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestAssignBusinessRole_CrossTenantIsolation(t *testing.T) {
	repo, _, user := newFixture()
	uc := NewAssignBusinessRoleUseCase(repo)
	// Query the same user under a DIFFERENT tenant → must not resolve.
	_, err := uc.Execute(context.Background(), uuid.New(), AssignBusinessRoleInput{
		TargetUserID: user, BusinessRole: domain.BusinessRoleViewer,
	})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Err != domain.ErrNotFound {
		t.Fatalf("cross-tenant lookup should be not-found, got %v", err)
	}
}

func TestAssignBusinessRole_RootProtected(t *testing.T) {
	repo, tenant, user := newFixture()
	repo.members[key(tenant, user)].Role = domain.RoleRoot
	uc := NewAssignBusinessRoleUseCase(repo)
	_, err := uc.Execute(context.Background(), tenant, AssignBusinessRoleInput{
		TargetUserID: user, BusinessRole: domain.BusinessRoleViewer,
	})
	assertValidation(t, err)
}

func TestAssignBusinessRole_ClearAndDowngrade(t *testing.T) {
	repo, tenant, user := newFixture()
	// Start as admin, downgrade to user and give a scoped role in one call.
	repo.members[key(tenant, user)].Role = domain.RoleAdmin
	uc := NewAssignBusinessRoleUseCase(repo)
	view, err := uc.Execute(context.Background(), tenant, AssignBusinessRoleInput{
		TargetUserID: user, BusinessRole: domain.BusinessRoleAuditor, MemberRole: domain.RoleUser,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.OrgRole != domain.RoleUser {
		t.Fatalf("expected downgrade to user, got %q", view.OrgRole)
	}
	// Auditor is not wildcard.
	if hasStr(view.Permissions, "*") {
		t.Fatal("downgraded auditor should not have wildcard")
	}
}

func TestAssignBusinessRole_InvalidMemberRole(t *testing.T) {
	repo, tenant, user := newFixture()
	uc := NewAssignBusinessRoleUseCase(repo)
	_, err := uc.Execute(context.Background(), tenant, AssignBusinessRoleInput{
		TargetUserID: user, BusinessRole: domain.BusinessRoleViewer, MemberRole: "superuser",
	})
	assertValidation(t, err)
}

func TestListMembers_ResolvesPermissions(t *testing.T) {
	repo, tenant, user := newFixture()
	repo.members[key(tenant, user)].BusinessRole = domain.BusinessRoleRiskManager
	uc := NewListMembersUseCase(repo)
	views, err := uc.Execute(context.Background(), tenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 member, got %d", len(views))
	}
	if views[0].Email != "analyst@acme.io" {
		t.Fatalf("user not preloaded, got %q", views[0].Email)
	}
	if !hasStr(views[0].Permissions, "risks:delete") {
		t.Fatalf("risk manager should resolve risks:delete, got %v", views[0].Permissions)
	}
}

func assertValidation(t *testing.T, err error) {
	t.Helper()
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Err != domain.ErrValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func hasStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
