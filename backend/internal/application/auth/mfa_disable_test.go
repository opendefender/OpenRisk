// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #754 — disabling MFA re-proves the password, is refused for roles that
// require MFA, and removes the secret and the backup codes together.

type disableHasher struct{}

func (disableHasher) Hash(p string) (string, error)  { return "hashed:" + p, nil }
func (disableHasher) Verify(hash, plain string) bool { return hash == "hashed:"+plain }

type disableUsers struct {
	users   map[uuid.UUID]*domain.User
	members map[uuid.UUID]*domain.OrganizationMember
	err     error
}

func (u *disableUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	return u.users[id], nil
}

func (u *disableUsers) GetOrganizationMember(_ context.Context, userID, _ uuid.UUID) (*domain.OrganizationMember, error) {
	if u.err != nil {
		return nil, u.err
	}
	return u.members[userID], nil
}

type disableMailer struct {
	to, locale string
	calls      int
}

func (m *disableMailer) SendMFADisabled(_ context.Context, to, _, locale string) error {
	m.to, m.locale = to, locale
	m.calls++
	return nil
}

const disablePassword = "Ancre-Vitrail7-Cobalt"

type disableFixture struct {
	uc      *DisableMFAUseCase
	repo    *MockMFARepository
	users   *disableUsers
	mailer  *disableMailer
	inApp   []string
	user    *domain.User
	tenant  uuid.UUID
	codeKey string
}

func newDisableFixture(t *testing.T, role domain.MemberRole, business domain.BusinessRoleKey) *disableFixture {
	t.Helper()
	tenant := uuid.New()
	user := &domain.User{ID: uuid.New(), Email: "rssi@banque.cm", FullName: "R. Ssi", IsActive: true, Password: "hashed:" + disablePassword}
	users := &disableUsers{
		users: map[uuid.UUID]*domain.User{user.ID: user},
		members: map[uuid.UUID]*domain.OrganizationMember{user.ID: {
			UserID: user.ID, OrganizationID: tenant, Role: role, BusinessRole: business,
		}},
	}
	repo := NewMockMFARepository()
	require.NoError(t, repo.CreateMFASecret(context.Background(), &domain.MFASecret{UserID: user.ID, TenantID: tenant, IsVerified: true}))
	require.NoError(t, repo.SaveBackupCodes(context.Background(), []*domain.MFABackupCode{{UserID: user.ID, TenantID: tenant, CodeHash: "h"}}))

	mailer := &disableMailer{}
	orgRoles, businessRoles := domain.DefaultMFAPrivilegeRoles()
	uc := NewDisableMFAUseCase(repo, users, disableHasher{}).
		RequireMFAForRoles(orgRoles, businessRoles).
		WithMailer(mailer)
	f := &disableFixture{uc: uc, repo: repo, users: users, mailer: mailer, user: user, tenant: tenant,
		codeKey: user.ID.String() + ":" + tenant.String()}
	uc.WithInAppNotifier(func(_ context.Context, tenantID, userID uuid.UUID, subject, _ string) {
		if tenantID == f.tenant && userID == f.user.ID {
			f.inApp = append(f.inApp, subject)
		}
	})
	return f
}

func (f *disableFixture) run(password, roleHint string) error {
	_, err := f.uc.Execute(context.Background(), DisableMFAInput{
		UserID: f.user.ID, TenantID: f.tenant, Password: password, OrgRoleHint: roleHint, Locale: "en",
	})
	return err
}

func (f *disableFixture) assertStillEnrolled(t *testing.T) {
	t.Helper()
	secret, _ := f.repo.GetMFASecret(context.Background(), f.user.ID, f.tenant)
	assert.NotNil(t, secret, "the secret must survive a refused disable")
	assert.Len(t, f.repo.codes[f.codeKey], 1, "the backup codes must survive a refused disable")
	assert.Zero(t, f.mailer.calls, "no notice for a disable that did not happen")
	assert.Empty(t, f.inApp, "no in-app notice for a disable that did not happen")
}

func TestDisableMFA_Success(t *testing.T) {
	f := newDisableFixture(t, domain.RoleUser, "")

	require.NoError(t, f.run(disablePassword, "user"))

	secret, _ := f.repo.GetMFASecret(context.Background(), f.user.ID, f.tenant)
	assert.Nil(t, secret)
	assert.Empty(t, f.repo.codes[f.codeKey])
	assert.Equal(t, 1, f.mailer.calls)
	assert.Equal(t, "rssi@banque.cm", f.mailer.to)
	assert.Equal(t, "en", f.mailer.locale)
	assert.Equal(t, []string{"Two-factor authentication turned off"}, f.inApp)
}

func TestDisableMFA_NotFound(t *testing.T) {
	f := newDisableFixture(t, domain.RoleUser, "")
	require.NoError(t, f.repo.DisableMFA(context.Background(), f.user.ID, f.tenant))

	err := f.run(disablePassword, "")

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr), "got %v", err)
	assert.ErrorIs(t, appErr.Err, domain.ErrNotFound)
	assert.Zero(t, f.mailer.calls)
}

func TestDisableMFA_Unauthorized(t *testing.T) {
	for name, password := range map[string]string{"wrong password": "guess-1234", "missing password": ""} {
		t.Run(name, func(t *testing.T) {
			f := newDisableFixture(t, domain.RoleUser, "")
			assert.ErrorIs(t, f.run(password, ""), ErrMFADisablePasswordIncorrect)
			f.assertStillEnrolled(t)
		})
	}
}

func TestDisableMFA_UnauthorizedWithoutSession(t *testing.T) {
	f := newDisableFixture(t, domain.RoleUser, "")
	_, err := f.uc.Execute(context.Background(), DisableMFAInput{TenantID: f.tenant, Password: disablePassword})

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr), "got %v", err)
	assert.ErrorIs(t, appErr.Err, domain.ErrUnauthorized)
	f.assertStillEnrolled(t)
}

func TestDisableMFA_RoleRequiringMFAIsRefused(t *testing.T) {
	cases := map[string]struct {
		role     domain.MemberRole
		business domain.BusinessRoleKey
		hint     string
	}{
		"admin":                           {domain.RoleAdmin, "", ""},
		"root":                            {domain.RoleRoot, "", ""},
		"security officer as org user":    {domain.RoleUser, domain.BusinessRoleRSSI, ""},
		"token says admin, row says user": {domain.RoleUser, "", "admin"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newDisableFixture(t, tc.role, tc.business)
			assert.ErrorIs(t, f.run(disablePassword, tc.hint), ErrMFARequiredByRole)
			f.assertStillEnrolled(t)
		})
	}
}

func TestDisableMFA_MembershipLookupFailureFailsClosed(t *testing.T) {
	f := newDisableFixture(t, domain.RoleUser, "")
	f.users.err = errors.New("db down")

	require.Error(t, f.run(disablePassword, ""))
	f.assertStillEnrolled(t)
}

func TestDisableMFA_NoRolePolicyAllowsAnAdmin(t *testing.T) {
	f := newDisableFixture(t, domain.RoleAdmin, "")
	// MFA_REQUIRED_ROLES="" — the deployment made MFA optional for everyone.
	f.uc.RequireMFAForRoles(nil, nil)

	require.NoError(t, f.run(disablePassword, "admin"))
}

func TestDisableMFA_AccountWithoutLocalPasswordIsRefused(t *testing.T) {
	f := newDisableFixture(t, domain.RoleUser, "")
	f.user.Password = ""

	assert.ErrorIs(t, f.run("", ""), ErrNoLocalPassword)
	f.assertStillEnrolled(t)
}
