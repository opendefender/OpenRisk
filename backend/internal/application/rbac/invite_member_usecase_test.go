// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

type fakeUsers struct {
	byEmail map[string]*domain.User
	byUser  map[string]*domain.User
	created []*domain.User
	members []*domain.OrganizationMember
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byEmail: map[string]*domain.User{}, byUser: map[string]*domain.User{}}
}
func (f *fakeUsers) GetByEmail(_ context.Context, e string) (*domain.User, error) {
	return f.byEmail[e], nil
}
func (f *fakeUsers) GetByUsername(_ context.Context, u string) (*domain.User, error) {
	return f.byUser[u], nil
}
func (f *fakeUsers) Create(_ context.Context, u *domain.User) error {
	u.ID = uuid.New()
	f.created = append(f.created, u)
	f.byEmail[u.Email] = u
	f.byUser[u.Username] = u
	return nil
}
func (f *fakeUsers) CreateOrganizationMember(_ context.Context, m *domain.OrganizationMember) error {
	f.members = append(f.members, m)
	return nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }

func appErrSentinel(t *testing.T, err error) error {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if ae, ok := err.(*domain.AppError); ok {
		return ae.Err
	}
	t.Fatalf("expected *domain.AppError, got %T: %v", err, err)
	return nil
}

func TestInviteMember_Success(t *testing.T) {
	users := newFakeUsers()
	uc := NewInviteMemberUseCase(users, fakeHasher{})
	tenant := uuid.New()

	res, err := uc.Execute(context.Background(), tenant, InviteMemberInput{
		Email:        "Awa.Analyst@example.com",
		FullName:     "Awa Analyst",
		BusinessRole: domain.BusinessRoleKey("rssi"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users.created) != 1 || len(users.members) != 1 {
		t.Fatalf("expected 1 user + 1 member, got %d + %d", len(users.created), len(users.members))
	}
	u := users.created[0]
	if u.Email != "awa.analyst@example.com" {
		t.Errorf("email not normalized: %q", u.Email)
	}
	if u.Username == "" || u.Password == "" {
		t.Error("username/password not set")
	}
	if u.DefaultOrgID == nil || *u.DefaultOrgID != tenant {
		t.Error("DefaultOrgID should be the inviting tenant")
	}
	m := users.members[0]
	if m.OrganizationID != tenant || m.Role != domain.RoleUser || m.BusinessRole != "rssi" {
		t.Errorf("membership wrong: org=%v role=%v br=%v", m.OrganizationID, m.Role, m.BusinessRole)
	}
	if res.TempPassword == "" {
		t.Error("temp password should be returned")
	}
	if res.Member.BusinessRole != "rssi" || len(res.Member.Permissions) == 0 {
		t.Error("member view should carry the resolved rssi access")
	}
}

func TestInviteMember_Conflict(t *testing.T) {
	users := newFakeUsers()
	users.byEmail["taken@example.com"] = &domain.User{Email: "taken@example.com"}
	uc := NewInviteMemberUseCase(users, fakeHasher{})

	_, err := uc.Execute(context.Background(), uuid.New(), InviteMemberInput{Email: "taken@example.com", FullName: "X"})
	if appErrSentinel(t, err) != domain.ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
	if len(users.created) != 0 {
		t.Error("no user should be created on conflict")
	}
}

func TestInviteMember_Validation(t *testing.T) {
	uc := NewInviteMemberUseCase(newFakeUsers(), fakeHasher{})
	cases := []InviteMemberInput{
		{Email: "not-an-email", FullName: "X"},                                         // bad email
		{Email: "ok@example.com", FullName: ""},                                        // no name
		{Email: "ok@example.com", FullName: "X", BusinessRole: "does_not_exist"},       // bad preset
		{Email: "ok@example.com", FullName: "X", MemberRole: domain.MemberRole("god")}, // bad role
	}
	for i, in := range cases {
		if _, err := uc.Execute(context.Background(), uuid.New(), in); appErrSentinel(t, err) != domain.ErrValidation {
			t.Errorf("case %d: expected ErrValidation, got %v", i, err)
		}
	}
}
