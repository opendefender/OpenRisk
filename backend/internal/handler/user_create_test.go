// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	authapp "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #870 — POST /users created an account and a membership but no
// default_org_id, and login resolves the organization from default_org_id: the
// account could never sign in. These tests create the account through the real
// handler behind its real gate, then sign it in through the real login use
// case with the real Argon2id hasher, against the same database.
// ---------------------------------------------------------------------------

const createdPassword = "Granit-Boussole-870"

type createUserRig struct {
	f     *orgFixture
	app   *fiber.App
	login *authapp.LoginUseCase
	keys  *authpkg.RSAKeys
	// orgRole is the caller's role in tenant A, as the session stamps it.
	orgRole string
	// tenant is the caller's active organization; uuid.Nil means none.
	tenant uuid.UUID
	// extraRoles are roles the session holds in other organizations.
	extraRoles map[uuid.UUID]string
}

func newCreateUserRig(t *testing.T) *createUserRig {
	t.Helper()
	f := newOrgFixture(t)
	sign := newSignInRig(t, f) // organizations + refresh_tokens, and keys

	if err := f.db.Exec(`CREATE TABLE roles (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	if err := sqliteschema.Reconcile(f.db, "roles", &domain.Role{}); err != nil {
		t.Fatalf("reconcile roles: %v", err)
	}
	for _, name := range []string{"Admin", "Manager", "Analyst", "Viewer"} {
		if err := f.db.Create(&domain.Role{ID: uuid.New(), Name: name}).Error; err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}

	// CreateUser reads the package-level database handle.
	orig := database.DB
	database.DB = f.db
	t.Cleanup(func() { database.DB = orig })

	r := &createUserRig{f: f, keys: sign.keys, orgRole: "admin", tenant: f.tenantA}
	r.login = authapp.NewLoginUseCase(
		repository.NewGormUserRepository(f.db),
		coreauth.NewTokenManager(f.db, sign.keys),
		coreauth.NewConfiguredArgon2idPasswordHasher(),
	)

	app := fiber.New()
	stamp := func(c *fiber.Ctx) error {
		claims := &authpkg.Claims{Sub: uuid.New(), TenantID: r.tenant, OrgRoles: map[uuid.UUID]string{}}
		for org, role := range r.extraRoles {
			claims.OrgRoles[org] = role
		}
		if r.tenant != uuid.Nil {
			claims.OrgRoles[r.tenant] = r.orgRole
		}
		c.Locals("user", claims)
		c.Locals("org_roles", claims.OrgRoles)
		c.Locals("tenant_id", claims.TenantID)
		return c.Next()
	}
	// The same gate as main.go: adminRole := middleware.RequireRole("admin").
	app.Post("/users", stamp, middleware.RequireRole("admin"), handler.CreateUser)
	r.app = app
	return r
}

func (r *createUserRig) create(t *testing.T, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	res, err := r.app.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	_ = json.Unmarshal(out, &decoded)
	return res.StatusCode, decoded
}

func newAccount(email, role string) map[string]any {
	return map[string]any{
		"email": email, "username": email[:5], "full_name": "Provisioned Account",
		"password": createdPassword, "role": role,
	}
}

func TestCreateUser_Success_AccountSignsInToTheCallersOrganization(t *testing.T) {
	r := newCreateUserRig(t)

	// "viewer" in lower case, as the input's comment always documented.
	status, body := r.create(t, newAccount("prov1@bank.io", "viewer"))
	if status != http.StatusCreated {
		t.Fatalf("POST /users: got %d %v, want 201", status, body)
	}

	out, err := r.login.Execute(context.Background(), authapp.LoginInput{Email: "prov1@bank.io", Password: createdPassword})
	if err != nil {
		t.Fatalf("the account POST /users created cannot sign in: %v", err)
	}
	if out.TokenPair == nil || out.Organization == nil {
		t.Fatalf("sign-in issued no session: %+v", out)
	}
	if out.Organization.ID != r.f.tenantA {
		t.Fatalf("signed in to %s, want the creator's organization %s", out.Organization.ID, r.f.tenantA)
	}
	claims, err := authpkg.ValidateAccessToken(r.keys, out.TokenPair.AccessToken, nil)
	if err != nil {
		t.Fatalf("minted token does not validate: %v", err)
	}
	if got := claims.OrgRoles[r.f.tenantA]; got != string(domain.RoleUser) {
		t.Fatalf("org role = %q, want %q: the global role must not grant an org role", got, domain.RoleUser)
	}

	var m domain.OrganizationMember
	if err := r.f.db.Where("organization_id = ? AND user_id IN (SELECT id FROM users WHERE email = ?)", r.f.tenantA, "prov1@bank.io").First(&m).Error; err != nil {
		t.Fatalf("membership missing: %v", err)
	}
	if m.Status != domain.MembershipActive || !m.IsActive {
		t.Fatalf("membership status=%q active=%v, want active", m.Status, m.IsActive)
	}
}

func TestCreateUser_NotFound_UnknownRoleNamesTheAcceptedOnes(t *testing.T) {
	r := newCreateUserRig(t)

	status, body := r.create(t, newAccount("prov2@bank.io", "superuser"))
	if status != http.StatusNotFound {
		t.Fatalf("got %d %v, want 404", status, body)
	}
	accepted, _ := body["accepted_roles"].([]any)
	if len(accepted) != 4 {
		t.Fatalf("accepted_roles = %v, want the 4 roles", body["accepted_roles"])
	}
	var n int64
	r.f.db.Model(&domain.User{}).Where("email = ?", "prov2@bank.io").Count(&n)
	if n != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

func TestCreateUser_Unauthorized_PlainMemberIsRefused(t *testing.T) {
	r := newCreateUserRig(t)
	r.orgRole = string(domain.RoleUser)

	status, _ := r.create(t, newAccount("prov3@bank.io", "viewer"))
	if status != http.StatusForbidden {
		t.Fatalf("got %d, want 403", status)
	}
	var n int64
	r.f.db.Model(&domain.User{}).Where("email = ?", "prov3@bank.io").Count(&n)
	if n != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

// A session with no active organization used to create an account that
// belonged nowhere. RequireRole already refuses a token with no role at all,
// so this drives the handler's own guard: a root role held elsewhere passes
// the gate, but there is no organization to add the account to.
func TestCreateUser_NoActiveOrganizationCreatesNothing(t *testing.T) {
	r := newCreateUserRig(t)
	r.tenant = uuid.Nil
	r.extraRoles = map[uuid.UUID]string{uuid.New(): "root"}

	status, _ := r.create(t, newAccount("prov4@bank.io", "viewer"))
	if status != http.StatusForbidden {
		t.Fatalf("got %d, want 403", status)
	}
	var n int64
	r.f.db.Model(&domain.User{}).Where("email = ?", "prov4@bank.io").Count(&n)
	if n != 0 {
		t.Fatal("an account with no organization must not be created")
	}
}

// The account and its membership are written together (rule 7). If the
// membership cannot be written, the account must not be left behind.
func TestCreateUser_MembershipFailureLeavesNoAccount(t *testing.T) {
	r := newCreateUserRig(t)
	if err := r.f.db.Exec(`CREATE TRIGGER no_members BEFORE INSERT ON organization_members
		BEGIN SELECT RAISE(ABORT, 'membership refused'); END`).Error; err != nil {
		t.Fatalf("trigger: %v", err)
	}

	status, _ := r.create(t, newAccount("prov5@bank.io", "viewer"))
	if status != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", status)
	}
	var n int64
	r.f.db.Model(&domain.User{}).Where("email = ?", "prov5@bank.io").Count(&n)
	if n != 0 {
		t.Fatal("the account was written without its membership")
	}
}
