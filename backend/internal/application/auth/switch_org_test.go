// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// memberStub is a MembershipRepository backed by an in-memory list.
type memberStub struct {
	byOrg      map[uuid.UUID]*domain.OrganizationMember // keyed by org id
	active     []*domain.OrganizationMember
	defaultOrg *uuid.UUID
}

func (m *memberStub) ListActiveMemberships(_ context.Context, _ uuid.UUID) ([]*domain.OrganizationMember, error) {
	return m.active, nil
}
func (m *memberStub) GetOrganizationMember(_ context.Context, _ uuid.UUID, orgID uuid.UUID) (*domain.OrganizationMember, error) {
	return m.byOrg[orgID], nil
}
func (m *memberStub) GetByID(_ context.Context, uid uuid.UUID) (*domain.User, error) {
	return &domain.User{ID: uid, DefaultOrgID: m.defaultOrg}, nil
}

func newSwitchTokenManager(t *testing.T) *coreauth.TokenManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
			expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
			created_at DATETIME, updated_at DATETIME
		)`).Error)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tm := coreauth.NewTokenManager(db, &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey})
	// Authorizing org resolver: membership was already validated by the use case.
	tm.SetOrgSessionResolver(func(_ context.Context, _ uuid.UUID, orgID uuid.UUID) (*coreauth.SessionClaims, error) {
		return &coreauth.SessionClaims{TenantID: orgID, OrgRoles: map[uuid.UUID]string{orgID: "admin"}, Permissions: []string{"*"}}, nil
	})
	return tm
}

func TestSwitchOrganization_Success(t *testing.T) {
	userID, orgA, orgB := uuid.New(), uuid.New(), uuid.New()
	stub := &memberStub{byOrg: map[uuid.UUID]*domain.OrganizationMember{
		orgB: {UserID: userID, OrganizationID: orgB, Role: domain.RoleAdmin, IsActive: true,
			Organization: &domain.Organization{ID: orgB, Name: "Org B", Slug: "org-b"}},
	}, defaultOrg: &orgA}

	uc := NewSwitchOrganizationUseCase(stub, newSwitchTokenManager(t))
	out, err := uc.Execute(context.Background(), SwitchOrganizationInput{UserID: userID, TargetOrgID: orgB})
	require.NoError(t, err)
	require.NotNil(t, out.TokenPair)
	require.NotEmpty(t, out.TokenPair.AccessToken)
	require.Equal(t, domain.RoleAdmin, out.Role)
	require.Equal(t, "Org B", out.Organization.Name)
}

func TestSwitchOrganization_NonMember_Forbidden(t *testing.T) {
	userID, orgB := uuid.New(), uuid.New()
	stub := &memberStub{byOrg: map[uuid.UUID]*domain.OrganizationMember{}} // no membership

	uc := NewSwitchOrganizationUseCase(stub, newSwitchTokenManager(t))
	_, err := uc.Execute(context.Background(), SwitchOrganizationInput{UserID: userID, TargetOrgID: orgB})
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrForbidden, appErr.Err)
}

func TestSwitchOrganization_InactiveMember_Forbidden(t *testing.T) {
	userID, orgB := uuid.New(), uuid.New()
	stub := &memberStub{byOrg: map[uuid.UUID]*domain.OrganizationMember{
		orgB: {UserID: userID, OrganizationID: orgB, Role: domain.RoleUser, IsActive: false},
	}}

	uc := NewSwitchOrganizationUseCase(stub, newSwitchTokenManager(t))
	_, err := uc.Execute(context.Background(), SwitchOrganizationInput{UserID: userID, TargetOrgID: orgB})
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrForbidden, appErr.Err, "a deactivated member must be rejected identically to a non-member")
}

func TestSwitchOrganization_MissingOrg_Validation(t *testing.T) {
	uc := NewSwitchOrganizationUseCase(&memberStub{}, newSwitchTokenManager(t))
	_, err := uc.Execute(context.Background(), SwitchOrganizationInput{UserID: uuid.New()})
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Err)
}

func TestListOrganizations_MarksDefault(t *testing.T) {
	userID, orgA, orgB := uuid.New(), uuid.New(), uuid.New()
	stub := &memberStub{
		active: []*domain.OrganizationMember{
			{UserID: userID, OrganizationID: orgA, Role: domain.RoleAdmin, IsActive: true,
				Organization: &domain.Organization{ID: orgA, Name: "Org A", Slug: "org-a"}},
			{UserID: userID, OrganizationID: orgB, Role: domain.RoleUser, IsActive: true,
				Organization: &domain.Organization{ID: orgB, Name: "Org B", Slug: "org-b"}},
		},
		defaultOrg: &orgB,
	}
	uc := NewListOrganizationsUseCase(stub)
	rows, err := uc.Execute(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byID := map[uuid.UUID]MembershipSummary{}
	for _, r := range rows {
		byID[r.OrganizationID] = r
	}
	require.False(t, byID[orgA].IsDefault)
	require.True(t, byID[orgB].IsDefault)
	require.Equal(t, "Org A", byID[orgA].Name)
}
