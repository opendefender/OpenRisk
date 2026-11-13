// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/auth"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// patRoleServer mounts the production chain in front of the two role guards:
// PATMiddleware, the RS256 gate, then RequireRole and RoleGuard routes. The
// resolver gives every member the role and permissions in roles/perms, as
// resolveSessionClaimsForOrg does for an administrator.
func patRoleServer(t *testing.T, svc *auth.PersonalAccessTokenService, user, tenant uuid.UUID, role string, perms []string) *fiber.App {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := &authpkg.RSAKeys{PrivateKey: pk, PublicKey: &pk.PublicKey}

	resolve := func(_ context.Context, uid, orgID uuid.UUID) (*auth.SessionClaims, error) {
		if uid != user || orgID != tenant {
			return nil, fmt.Errorf("not an active member")
		}
		return &auth.SessionClaims{
			TenantID:    orgID,
			OrgRoles:    map[uuid.UUID]string{orgID: role},
			Permissions: perms,
		}, nil
	}

	ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(PATMiddleware(svc, resolve))
	api.Use(Protected(keys, nil))
	api.Get("/risks", RequirePermission("risks:read"), ok)
	api.Put("/telemetry", RequireRole("admin", "root"), ok)
	api.Put("/security/mfa-policy", RequireRole("admin", "root"), ok)
	api.Get("/legacy-admin", RoleGuard("admin"), ok)
	return app
}

func patStatus(t *testing.T, app *fiber.App, method, path, bearer string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// GHSA-92r7-7fhw-45gg: a token an administrator scoped to a read permission
// reads what its scope grants and nothing that needs the administrator role.
func TestPATRoles_ScopedAdminTokenFailsRoleGuards(t *testing.T) {
	for _, scopes := range [][]string{{"risks:read"}, {"risks:*"}, {"risks:read", "settings:*"}} {
		t.Run(fmt.Sprint(scopes), func(t *testing.T) {
			svc := auth.NewPersonalAccessTokenService(&patRepoFake{})
			user, tenant := uuid.New(), uuid.New()
			app := patRoleServer(t, svc, user, tenant, "admin", []string{"*"})
			_, raw, err := svc.CreateToken(context.Background(), user, tenant, "ci", "", scopes, nil)
			if err != nil {
				t.Fatal(err)
			}

			if got := patStatus(t, app, "GET", "/api/v1/risks", raw); got != fiber.StatusOK {
				t.Errorf("GET /risks: %d, want 200", got)
			}
			for _, r := range []struct{ method, path string }{
				{"PUT", "/api/v1/telemetry"},
				{"PUT", "/api/v1/security/mfa-policy"},
				{"GET", "/api/v1/legacy-admin"},
			} {
				if got := patStatus(t, app, r.method, r.path, raw); got != fiber.StatusForbidden {
					t.Errorf("%s %s: %d, want 403", r.method, r.path, got)
				}
			}
		})
	}
}

// A root owner's scoped token is refused too: RequireRole's root shortcut reads
// the same org_roles a scoped token no longer carries.
func TestPATRoles_ScopedRootTokenFailsRoleGuards(t *testing.T) {
	svc := auth.NewPersonalAccessTokenService(&patRepoFake{})
	user, tenant := uuid.New(), uuid.New()
	app := patRoleServer(t, svc, user, tenant, "root", []string{"*"})
	_, raw, err := svc.CreateToken(context.Background(), user, tenant, "ci", "", []string{"risks:read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := patStatus(t, app, "PUT", "/api/v1/telemetry", raw); got != fiber.StatusForbidden {
		t.Errorf("PUT /telemetry: %d, want 403", got)
	}
}

// A "*" token is the owner's full delegation: an administrator's keeps the role,
// a member's still does not gain it.
func TestPATRoles_FullAccessTokenKeepsOwnerRole(t *testing.T) {
	cases := []struct {
		role  string
		perms []string
		want  int
	}{
		{"admin", []string{"*"}, fiber.StatusOK},
		{"member", []string{"risks:read"}, fiber.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			svc := auth.NewPersonalAccessTokenService(&patRepoFake{})
			user, tenant := uuid.New(), uuid.New()
			app := patRoleServer(t, svc, user, tenant, tc.role, tc.perms)
			_, raw, err := svc.CreateToken(context.Background(), user, tenant, "full", "", []string{"*"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := patStatus(t, app, "PUT", "/api/v1/telemetry", raw); got != tc.want {
				t.Errorf("RequireRole: %d, want %d", got, tc.want)
			}
			if got := patStatus(t, app, "GET", "/api/v1/legacy-admin", raw); got != tc.want {
				t.Errorf("RoleGuard: %d, want %d", got, tc.want)
			}
		})
	}
}

// The guards refuse a scoped token on their own, even if org_roles were set by
// something other than PATMiddleware, and fail closed when the scopes are missing.
func TestPATRoles_GuardsRefuseScopedTokenWithRoles(t *testing.T) {
	for name, scopes := range map[string]interface{}{
		"scoped":  []string{"risks:read"},
		"missing": nil,
	} {
		t.Run(name, func(t *testing.T) {
			tenant := uuid.New()
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error {
				c.Locals("is_pat", true)
				if scopes != nil {
					c.Locals("token_scopes", scopes)
				}
				c.Locals("org_roles", map[uuid.UUID]string{tenant: "admin"})
				return c.Next()
			})
			ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
			app.Get("/require-role", RequireRole("admin"), ok)
			app.Get("/role-guard", RoleGuard("admin"), ok)

			for _, path := range []string{"/require-role", "/role-guard"} {
				if got := patStatus(t, app, "GET", path, "x"); got != fiber.StatusForbidden {
					t.Errorf("%s: %d, want 403", path, got)
				}
			}
		})
	}
}

func TestDelegatesOwnerRoles(t *testing.T) {
	cases := []struct {
		scopes []string
		want   bool
	}{
		{[]string{"*"}, true},
		{[]string{"risks:read", "*"}, true},
		{[]string{"risks:*"}, false},
		{[]string{"risks:read"}, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := delegatesOwnerRoles(c.scopes); got != c.want {
			t.Errorf("delegatesOwnerRoles(%v) = %v, want %v", c.scopes, got, c.want)
		}
	}
}
