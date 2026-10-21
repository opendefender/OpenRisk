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
	"strings"
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
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #590 — the role an account created here holds is its MEMBERSHIP's role
// (admin | user, plus an optional business role), exactly as an invitation
// grants it; users.role_id is never written.
//
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
	// anonymous drops the session entirely, as if no token were presented.
	anonymous bool
}

func newCreateUserRig(t *testing.T) *createUserRig {
	t.Helper()
	f := newOrgFixture(t)
	sign := newSignInRig(t, f) // organizations + refresh_tokens, and keys

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
		if r.anonymous {
			return c.Next()
		}
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
	// The handler's own guard, reached without the role gate in front of it.
	app.Post("/users-ungated", stamp, handler.CreateUser)
	r.app = app
	return r
}

func (r *createUserRig) create(t *testing.T, body map[string]any) (int, map[string]any) {
	t.Helper()
	return r.post(t, "/users", body)
}

func (r *createUserRig) post(t *testing.T, path string, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
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

func (r *createUserRig) countUsers(t *testing.T, email string) int64 {
	t.Helper()
	var n int64
	if err := r.f.db.Model(&domain.User{}).Where("email = ?", email).Count(&n).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

func (r *createUserRig) membershipOf(t *testing.T, email string) domain.OrganizationMember {
	t.Helper()
	var m domain.OrganizationMember
	if err := r.f.db.Where("organization_id = ? AND user_id IN (SELECT id FROM users WHERE email = ?)", r.f.tenantA, email).First(&m).Error; err != nil {
		t.Fatalf("membership missing: %v", err)
	}
	return m
}

func TestCreateUser_Success_AccountSignsInToTheCallersOrganization(t *testing.T) {
	r := newCreateUserRig(t)

	body := newAccount("prov1@bank.io", "user")
	body["business_role"] = "security_analyst"
	status, resp := r.create(t, body)
	if status != http.StatusCreated {
		t.Fatalf("POST /users: got %d %v, want 201", status, resp)
	}
	if resp["role"] != "user" || resp["business_role"] != "security_analyst" {
		t.Fatalf("response role=%v business_role=%v, want the membership's user/security_analyst", resp["role"], resp["business_role"])
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
		t.Fatalf("org role = %q, want %q", got, domain.RoleUser)
	}

	m := r.membershipOf(t, "prov1@bank.io")
	if m.Status != domain.MembershipActive || !m.IsActive {
		t.Fatalf("membership status=%q active=%v, want active", m.Status, m.IsActive)
	}
	if m.Role != domain.RoleUser || m.BusinessRole != domain.BusinessRoleSecurityAnalyst {
		t.Fatalf("membership role=%q business=%q, want user/security_analyst", m.Role, m.BusinessRole)
	}

	var u domain.User
	if err := r.f.db.Where("email = ?", "prov1@bank.io").First(&u).Error; err != nil {
		t.Fatalf("user: %v", err)
	}
	if u.RoleID != uuid.Nil {
		t.Fatalf("users.role_id = %s, want it left unwritten: the role lives on the membership", u.RoleID)
	}
}

func TestCreateUser_Success_AdminRoleDropsBusinessRole(t *testing.T) {
	r := newCreateUserRig(t)

	// Case and padding do not matter, and the email is normalised.
	body := newAccount("  Prov6@Bank.IO ", " Admin ")
	body["username"] = "prov6"
	body["business_role"] = "rssi"
	status, resp := r.create(t, body)
	if status != http.StatusCreated {
		t.Fatalf("got %d %v, want 201", status, resp)
	}
	if resp["role"] != "admin" {
		t.Fatalf("response role = %v, want admin", resp["role"])
	}
	if _, present := resp["business_role"]; present {
		t.Fatalf("business_role = %v, want it dropped for an admin", resp["business_role"])
	}
	m := r.membershipOf(t, "prov6@bank.io")
	if m.Role != domain.RoleAdmin || m.BusinessRole != "" {
		t.Fatalf("membership role=%q business=%q, want admin with no business role", m.Role, m.BusinessRole)
	}
}

func TestCreateUser_Success_EmptyRoleIsAPlainMember(t *testing.T) {
	r := newCreateUserRig(t)

	status, resp := r.create(t, newAccount("prov7@bank.io", ""))
	if status != http.StatusCreated {
		t.Fatalf("got %d %v, want 201", status, resp)
	}
	if m := r.membershipOf(t, "prov7@bank.io"); m.Role != domain.RoleUser {
		t.Fatalf("membership role = %q, want user", m.Role)
	}
}

func TestCreateUser_NotFound_ActiveOrganizationNoLongerExists(t *testing.T) {
	r := newCreateUserRig(t)
	gone := uuid.New()
	r.tenant = gone

	status, body := r.create(t, newAccount("prov8@bank.io", "user"))
	if status != http.StatusNotFound {
		t.Fatalf("got %d %v, want 404", status, body)
	}
	if body["error"] != "organization not found" {
		t.Fatalf("error = %v, want it to name the organization", body["error"])
	}
	if r.countUsers(t, "prov8@bank.io") != 0 {
		t.Fatal("a refused request must create nothing")
	}
	var members int64
	r.f.db.Model(&domain.OrganizationMember{}).Where("organization_id = ?", gone).Count(&members)
	if members != 0 {
		t.Fatal("no membership may point at a missing organization")
	}
}

func TestCreateUser_Unauthorized_NoSessionIsRefused(t *testing.T) {
	r := newCreateUserRig(t)
	r.anonymous = true

	// Straight at the handler: its own guard, not the role gate, answers.
	status, _ := r.post(t, "/users-ungated", newAccount("prov9@bank.io", "user"))
	if status != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", status)
	}
	if r.countUsers(t, "prov9@bank.io") != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

func TestCreateUser_Unauthorized_PlainMemberIsRefused(t *testing.T) {
	r := newCreateUserRig(t)
	r.orgRole = string(domain.RoleUser)

	status, _ := r.create(t, newAccount("prov3@bank.io", "user"))
	if status != http.StatusForbidden {
		t.Fatalf("got %d, want 403", status)
	}
	if r.countUsers(t, "prov3@bank.io") != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

func TestCreateUser_Validation_UnknownRoleNamesTheAcceptedOnes(t *testing.T) {
	r := newCreateUserRig(t)

	// "viewer" was a global role the old contract accepted; it is not an org role.
	for _, role := range []string{"superuser", "viewer", "root"} {
		status, body := r.create(t, newAccount("prov2@bank.io", role))
		if status != http.StatusBadRequest {
			t.Fatalf("role %q: got %d %v, want 400", role, status, body)
		}
		if body["error"] != "role must be one of: admin, user" {
			t.Fatalf("role %q: error = %v, want it to name the accepted roles", role, body["error"])
		}
		accepted, _ := body["accepted_roles"].([]any)
		if len(accepted) != 2 || accepted[0] != "admin" || accepted[1] != "user" {
			t.Fatalf("role %q: accepted_roles = %v, want [admin user]", role, body["accepted_roles"])
		}
	}
	if r.countUsers(t, "prov2@bank.io") != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

func TestCreateUser_Validation_UnknownBusinessRole(t *testing.T) {
	r := newCreateUserRig(t)

	body := newAccount("provA@bank.io", "user")
	body["business_role"] = "chief_wizard"
	status, resp := r.create(t, body)
	if status != http.StatusBadRequest {
		t.Fatalf("got %d %v, want 400", status, resp)
	}
	if resp["error"] != "unknown business role: chief_wizard" {
		t.Fatalf("error = %v, want it to name the unknown business role", resp["error"])
	}
	if r.countUsers(t, "provA@bank.io") != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

func TestCreateUser_Validation_MalformedInputNamesTheField(t *testing.T) {
	r := newCreateUserRig(t)

	cases := []struct {
		name  string
		edit  func(map[string]any)
		cause string
	}{
		{"bad email", func(b map[string]any) { b["email"] = "not-an-email" }, "email must be a valid email address"},
		{"short password", func(b map[string]any) { b["password"] = "short" }, "password must be at least 12 characters"},
		// Long enough for the tag, refused by the platform policy that
		// registration and invitation acceptance apply.
		{"weak password", func(b map[string]any) { b["password"] = "passwordpassword" }, domain.MessageFromError(domain.ValidatePassword("passwordpassword"))},
		{"no full name", func(b map[string]any) { delete(b, "full_name") }, "full_name is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := newAccount("provB@bank.io", "user")
			tc.edit(body)
			status, resp := r.create(t, body)
			if status != http.StatusBadRequest {
				t.Fatalf("got %d %v, want 400", status, resp)
			}
			if resp["error"] != tc.cause {
				t.Fatalf("error = %v, want %q", resp["error"], tc.cause)
			}
		})
	}
	var n int64
	r.f.db.Model(&domain.User{}).Where("username = ?", "provB").Count(&n)
	if n != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

func TestCreateUser_Conflict_EmailTaken(t *testing.T) {
	r := newCreateUserRig(t)

	// admin@a.io is seeded by the fixture; a different case is the same address.
	body := newAccount("Admin@A.io", "user")
	body["username"] = "fresh-name"
	status, resp := r.create(t, body)
	if status != http.StatusConflict {
		t.Fatalf("got %d %v, want 409", status, resp)
	}
	if resp["error"] != "email already exists" {
		t.Fatalf("error = %v, want it to name the email", resp["error"])
	}
	var n int64
	r.f.db.Model(&domain.User{}).Where("username = ?", "fresh-name").Count(&n)
	if n != 0 {
		t.Fatal("a refused request must create nothing")
	}
}

func TestCreateUser_Conflict_UsernameTaken(t *testing.T) {
	r := newCreateUserRig(t)

	if status, resp := r.create(t, newAccount("provC@bank.io", "user")); status != http.StatusCreated {
		t.Fatalf("first create: got %d %v, want 201", status, resp)
	}
	body := newAccount("provC@other.io", "user") // same username "provC"
	status, resp := r.create(t, body)
	if status != http.StatusConflict {
		t.Fatalf("got %d %v, want 409", status, resp)
	}
	if resp["error"] != "username already exists" {
		t.Fatalf("error = %v, want it to name the username", resp["error"])
	}
	if r.countUsers(t, "provc@other.io") != 0 {
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

	status, _ := r.create(t, newAccount("prov4@bank.io", "user"))
	if status != http.StatusForbidden {
		t.Fatalf("got %d, want 403", status)
	}
	if r.countUsers(t, "prov4@bank.io") != 0 {
		t.Fatal("an account with no organization must not be created")
	}
}

// The account and its membership are written together (rule 7). If the
// membership cannot be written, the account must not be left behind, and the
// caller is told which step failed without seeing the database error.
func TestCreateUser_MembershipFailureLeavesNoAccount(t *testing.T) {
	r := newCreateUserRig(t)
	if err := r.f.db.Exec(`CREATE TRIGGER no_members BEFORE INSERT ON organization_members
		BEGIN SELECT RAISE(ABORT, 'membership refused'); END`).Error; err != nil {
		t.Fatalf("trigger: %v", err)
	}

	status, body := r.create(t, newAccount("prov5@bank.io", "user"))
	if status != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", status)
	}
	msg, _ := body["error"].(string)
	if msg != "could not create the account and its membership" {
		t.Fatalf("error = %q, want it to name the failed stage", msg)
	}
	if strings.Contains(msg, "membership refused") {
		t.Fatal("the raw database error leaked to the client")
	}
	if r.countUsers(t, "prov5@bank.io") != 0 {
		t.Fatal("the account was written without its membership")
	}
}
