// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	authapp "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #807 — nothing done in the context of organization A may change whether a
// person can sign in to organization B, or what they can do there.
//
// The routes that broke this (PATCH /users/:id/status, PATCH /users/:id/role,
// DELETE /users/:id) are gone; authz_route_coverage_test.go keeps them gone.
// What remains for A's administrator is the membership route, and these tests
// drive it for real, then sign the person in through the real login use case
// against the same database.
// ---------------------------------------------------------------------------

// verbatimHasher treats the stored hash as "hashed:<password>", like the rest
// of the suite's fakes.
type verbatimHasher struct{}

func (verbatimHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }
func (verbatimHasher) Verify(h, p string) bool       { return h == "hashed:"+p }

const dualPassword = "Ardoise-Lanterne-42"

type signInRig struct {
	login *authapp.LoginUseCase
	keys  *authpkg.RSAKeys
}

// newSignInRig adds what login needs on top of the membership fixture: the
// organizations the memberships point at, and a refresh-token store.
func newSignInRig(t *testing.T, f *orgFixture) *signInRig {
	t.Helper()
	for _, ddl := range []string{
		`CREATE TABLE organizations (id TEXT PRIMARY KEY)`,
		`CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
			expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
			created_at DATETIME, updated_at DATETIME)`,
	} {
		if err := f.db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	if err := sqliteschema.Reconcile(f.db, "organizations", &domain.Organization{}); err != nil {
		t.Fatalf("reconcile organizations: %v", err)
	}
	for _, o := range []domain.Organization{
		{ID: f.tenantA, Name: "Banque Atlantique", Slug: "banque-atlantique", IsActive: true},
		{ID: f.tenantB, Name: "Other Corp", Slug: "other-corp", IsActive: true},
	} {
		o := o
		if err := f.db.Create(&o).Error; err != nil {
			t.Fatalf("seed organization: %v", err)
		}
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	keys := &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}
	login := authapp.NewLoginUseCase(repository.NewGormUserRepository(f.db), coreauth.NewTokenManager(f.db, keys), verbatimHasher{})
	return &signInRig{login: login, keys: keys}
}

// session signs the person in and returns the organization and the claims the
// minted access token actually carries. A refused sign-in fails the test.
func (r *signInRig) session(t *testing.T, email string) (*domain.Organization, *authpkg.Claims) {
	t.Helper()
	out, err := r.login.Execute(context.Background(), authapp.LoginInput{Email: email, Password: dualPassword})
	if err != nil {
		t.Fatalf("sign-in of %s refused: %v", email, err)
	}
	if out.TokenPair == nil {
		t.Fatalf("sign-in of %s issued no session", email)
	}
	claims, err := authpkg.ValidateAccessToken(r.keys, out.TokenPair.AccessToken, nil)
	if err != nil {
		t.Fatalf("minted token does not validate: %v", err)
	}
	return out.Organization, claims
}

// seedDualMember creates one person with an active membership in A (their
// default organization, the worst case) and in B, with a different role in each.
func seedDualMember(t *testing.T, f *orgFixture) (user *domain.User, inA, inB *domain.OrganizationMember) {
	t.Helper()
	user = &domain.User{
		ID: uuid.New(), Email: "dual@both.io", Username: "dual", FullName: "Dual Member",
		IsActive: true, Password: "hashed:" + dualPassword, DefaultOrgID: &f.tenantA,
	}
	if err := f.db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	mk := func(org uuid.UUID, role domain.MemberRole, business domain.BusinessRoleKey) *domain.OrganizationMember {
		m := &domain.OrganizationMember{
			ID: uuid.New(), OrganizationID: org, UserID: user.ID, Role: role, BusinessRole: business,
			Status: domain.MembershipActive, IsActive: true,
			JoinedAt: f.now, CreatedAt: f.now, UpdatedAt: f.now,
		}
		if err := f.db.Create(m).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
		return m
	}
	inA = mk(f.tenantA, domain.RoleUser, "")
	inB = mk(f.tenantB, domain.RoleAdmin, "")
	return user, inA, inB
}

func TestCrossTenant_AdminOfAWithdrawingAccessLeavesBUntouched(t *testing.T) {
	for _, status := range []domain.MembershipStatus{domain.MembershipDeactivated, domain.MembershipRevoked} {
		t.Run(string(status), func(t *testing.T) {
			f := newOrgFixture(t)
			rig := newSignInRig(t, f)
			user, inA, inB := seedDualMember(t, f)

			// Before: the default organization is A.
			org, claims := rig.session(t, user.Email)
			if org.ID != f.tenantA || claims.TenantID != f.tenantA {
				t.Fatalf("precondition: expected a session in A, got %s", org.ID)
			}

			// A's administrator withdraws access in A.
			f.mustCall(t, "admin@a.io", http.MethodPut,
				"/api/v1/organization/members/"+inA.ID.String()+"/status",
				map[string]any{"status": string(status)}, http.StatusOK)

			// After: the person still signs in, and lands in B with B's role.
			org, claims = rig.session(t, user.Email)
			if org.ID != f.tenantB || claims.TenantID != f.tenantB {
				t.Fatalf("expected a session in B after A withdrew access, got %s", claims.TenantID)
			}
			if got := claims.OrgRoles[f.tenantB]; got != string(domain.RoleAdmin) {
				t.Fatalf("role in B = %q, want %q", got, domain.RoleAdmin)
			}
			if _, leaked := claims.OrgRoles[f.tenantA]; leaked {
				t.Fatal("the session in B must carry no role in A")
			}
			t.Logf("after A set %s: sign-in lands in B (%s) with org_roles=%v", status, claims.TenantID, claims.OrgRoles)

			// The account and B's membership are byte-for-byte what they were.
			var storedUser domain.User
			f.db.First(&storedUser, "id = ?", user.ID)
			if !storedUser.IsActive {
				t.Fatal("users.is_active was changed by a membership action in A")
			}
			var storedB domain.OrganizationMember
			f.db.First(&storedB, "id = ?", inB.ID)
			if storedB.Role != inB.Role || storedB.Status != domain.MembershipActive || !storedB.IsActive {
				t.Fatalf("B membership changed: role=%s status=%s active=%v", storedB.Role, storedB.Status, storedB.IsActive)
			}
			var storedA domain.OrganizationMember
			f.db.First(&storedA, "id = ?", inA.ID)
			if storedA.Status != status || storedA.IsActive {
				t.Fatalf("A membership should be %s, got %s active=%v", status, storedA.Status, storedA.IsActive)
			}
		})
	}
}

// The fallback opens nothing that was closed: a person whose only membership
// was withdrawn is still refused.
func TestCrossTenant_SoleMembershipWithdrawnStillRefusesSignIn(t *testing.T) {
	f := newOrgFixture(t)
	rig := newSignInRig(t, f)
	user, inA, inB := seedDualMember(t, f)
	for _, m := range []struct {
		actor string
		id    uuid.UUID
	}{{"admin@a.io", inA.ID}, {"admin@b.io", inB.ID}} {
		f.mustCall(t, m.actor, http.MethodPut, "/api/v1/organization/members/"+m.id.String()+"/status",
			map[string]any{"status": string(domain.MembershipRevoked)}, http.StatusOK)
	}
	out, err := rig.login.Execute(context.Background(), authapp.LoginInput{Email: user.Email, Password: dualPassword})
	if err == nil || out != nil {
		t.Fatalf("a person with no active membership must not get a session, got %+v", out)
	}
	if !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("unexpected refusal: %v", err)
	}
}

// Criterion 3: users.role_id is not an input to any authorization decision.
// A person whose global role row says admin, but whose membership says user,
// gets a user session and is refused by the admin gate on /users.
func TestCrossTenant_GlobalRoleIDGrantsNothing(t *testing.T) {
	f := newOrgFixture(t)
	rig := newSignInRig(t, f)
	user, _, _ := seedDualMember(t, f)

	if err := f.db.Exec(`CREATE TABLE roles (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	if err := sqliteschema.Reconcile(f.db, "roles", &domain.Role{}); err != nil {
		t.Fatalf("reconcile roles: %v", err)
	}
	adminRole := domain.Role{ID: uuid.New(), Name: "admin"}
	if err := f.db.Create(&adminRole).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if err := f.db.Model(&domain.User{}).Where("id = ?", user.ID).Update("role_id", adminRole.ID).Error; err != nil {
		t.Fatalf("set role_id: %v", err)
	}

	_, claims := rig.session(t, user.Email)
	if got := claims.OrgRoles[f.tenantA]; got != string(domain.RoleUser) {
		t.Fatalf("session role = %q, want the membership's %q", got, domain.RoleUser)
	}
	if claims.HasPermission("*") {
		t.Fatal("users.role_id = admin must not put a wildcard in the session")
	}

	// The /users admin gate, fed exactly what the auth middleware stamps from
	// that token. The handler is a sentinel: reaching it is the failure.
	app := fiber.New()
	stamp := func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		c.Locals("org_roles", claims.OrgRoles)
		c.Locals("permissions", claims.Permissions)
		c.Locals("tenant_id", claims.TenantID)
		return c.Next()
	}
	reached := func(c *fiber.Ctx) error { return c.SendStatus(http.StatusTeapot) }
	gate := middleware.RequireRole("admin")
	app.Get("/users", stamp, gate, reached)
	app.Post("/users", stamp, gate, reached)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		res, err := app.Test(httptest.NewRequest(method, "/users", nil))
		if err != nil {
			t.Fatalf("%s /users: %v", method, err)
		}
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s /users with users.role_id=admin and membership role user: got %d, want 403", method, res.StatusCode)
		}
	}
}
