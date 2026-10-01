// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// #831 — an organization's decision about a member ends that member's sessions
// in that organization only. The membership route is driven for real, with the
// real TokenManager as the revoker, against live refresh tokens in A and B.

func TestMembershipSessions_WithdrawingAccessInAKeepsTheSessionInB(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		body   map[string]any
		reason string
	}{
		{"deactivate", "/status", map[string]any{"status": string(domain.MembershipDeactivated)}, "access withdrawn"},
		{"revoke", "/status", map[string]any{"status": string(domain.MembershipRevoked)}, "access withdrawn"},
		{"role change", "/role", map[string]any{"role": string(domain.RoleAdmin)}, "claims must be re-derived"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrgFixture(t)
			tm := newSessionManager(t, f)
			f.svc.WithSessionRevoker(tm)

			user, inA, _ := seedTwoOrgMember(t, f)
			ctx := context.Background()
			pairA, err := tm.GenerateTokenPair(ctx, user.ID, f.tenantA, nil, nil, nil, coreauth.DeviceContext{})
			if err != nil {
				t.Fatalf("session in A: %v", err)
			}
			pairB, err := tm.GenerateTokenPair(ctx, user.ID, f.tenantB, nil, nil, nil, coreauth.DeviceContext{})
			if err != nil {
				t.Fatalf("session in B: %v", err)
			}

			f.mustCall(t, "admin@a.io", http.MethodPut,
				"/api/v1/organization/members/"+inA.ID.String()+tc.path, tc.body, http.StatusOK)

			if _, err := tm.RefreshTokenPair(ctx, pairA.RefreshToken, coreauth.DeviceContext{}); err == nil {
				t.Fatalf("the A session must end after %s (%s)", tc.name, tc.reason)
			}
			next, err := tm.RefreshTokenPair(ctx, pairB.RefreshToken, coreauth.DeviceContext{})
			if err != nil {
				t.Fatalf("the B session must survive a %s in A, refresh failed: %v", tc.name, err)
			}
			t.Logf("after %s in A: A refresh refused, B refresh ok (new token issued: %v)", tc.name, next.AccessToken != "")
		})
	}
}

// Criterion 3: an account-level event still ends every session. Password
// change and reset call RevokeAllUserTokens (change_password.go,
// password_reset.go); this pins what that call does across organizations.
func TestMembershipSessions_AccountLevelRevocationStillEndsEverySession(t *testing.T) {
	f := newOrgFixture(t)
	tm := newSessionManager(t, f)
	user, _, _ := seedTwoOrgMember(t, f)
	ctx := context.Background()

	pairA, _ := tm.GenerateTokenPair(ctx, user.ID, f.tenantA, nil, nil, nil, coreauth.DeviceContext{})
	pairB, _ := tm.GenerateTokenPair(ctx, user.ID, f.tenantB, nil, nil, nil, coreauth.DeviceContext{})

	if err := tm.RevokeAllUserTokens(ctx, user.ID); err != nil {
		t.Fatalf("revoke all: %v", err)
	}
	for name, p := range map[string]*coreauth.TokenPair{"A": pairA, "B": pairB} {
		if _, err := tm.RefreshTokenPair(ctx, p.RefreshToken, coreauth.DeviceContext{}); err == nil {
			t.Errorf("an account-level revocation must end the %s session too", name)
		}
	}
}

// newSessionManager builds a TokenManager over the fixture's database. Its org
// resolver re-checks the membership the way the composition root's
// resolveSessionForOrg does: an inactive or missing membership refuses.
func newSessionManager(t *testing.T, f *orgFixture) *coreauth.TokenManager {
	t.Helper()
	if err := f.db.Exec(`CREATE TABLE refresh_tokens (
		id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL,
		family_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
		device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
		expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
		created_at DATETIME, updated_at DATETIME)`).Error; err != nil {
		t.Fatalf("create refresh_tokens: %v", err)
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	tm := coreauth.NewTokenManager(f.db, &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey})
	tm.SetOrgSessionResolver(func(ctx context.Context, userID, orgID uuid.UUID) (*coreauth.SessionClaims, error) {
		var m domain.OrganizationMember
		if err := f.db.WithContext(ctx).Where("user_id = ? AND organization_id = ? AND is_active = ?", userID, orgID, true).First(&m).Error; err != nil {
			return nil, errors.New("no active membership")
		}
		return &coreauth.SessionClaims{TenantID: orgID, OrgRoles: map[uuid.UUID]string{orgID: string(m.Role)}}, nil
	})
	return tm
}

// seedTwoOrgMember creates one person with an active membership in A and in B.
func seedTwoOrgMember(t *testing.T, f *orgFixture) (*domain.User, *domain.OrganizationMember, *domain.OrganizationMember) {
	t.Helper()
	user := &domain.User{ID: uuid.New(), Email: "dual@both.io", Username: "dual", FullName: "Dual Member", IsActive: true}
	if err := f.db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	mk := func(org uuid.UUID, role domain.MemberRole) *domain.OrganizationMember {
		m := &domain.OrganizationMember{
			ID: uuid.New(), OrganizationID: org, UserID: user.ID, Role: role,
			Status: domain.MembershipActive, IsActive: true,
			JoinedAt: f.now, CreatedAt: f.now, UpdatedAt: f.now,
		}
		if err := f.db.Create(m).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
		return m
	}
	return user, mk(f.tenantA, domain.RoleUser), mk(f.tenantB, domain.RoleAdmin)
}
