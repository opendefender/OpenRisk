// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// patRepoFake stores tokens in memory with the GORM repository's predicates.
type patRepoFake struct{ rows []*domain.PersonalAccessToken }

func (r *patRepoFake) Create(_ context.Context, t *domain.PersonalAccessToken) error {
	t.ID = uuid.New()
	r.rows = append(r.rows, t)
	return nil
}
func (r *patRepoFake) GetByTokenHash(_ context.Context, h string) (*domain.PersonalAccessToken, error) {
	for _, t := range r.rows {
		if t.TokenHash == h {
			return t, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (r *patRepoFake) ListByOwner(context.Context, uuid.UUID, uuid.UUID) ([]*domain.PersonalAccessToken, error) {
	return nil, nil
}
func (r *patRepoFake) UpdateLastUsed(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *patRepoFake) DeleteByOwner(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// patTestServer mounts the production chain: PATMiddleware, then the RS256 JWT
// gate, then a protected route that echoes the resolved tenant and permissions.
// members maps user → the organizations they are an active member of, with
// their permissions there; the resolver refuses any other pair, as
// resolveSessionClaimsForOrg does.
func patTestServer(t *testing.T, svc *auth.PersonalAccessTokenService, members map[uuid.UUID]map[uuid.UUID][]string) *fiber.App {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := &authpkg.RSAKeys{PrivateKey: pk, PublicKey: &pk.PublicKey}

	resolve := func(_ context.Context, uid, orgID uuid.UUID) (*auth.SessionClaims, error) {
		perms, ok := members[uid][orgID]
		if !ok {
			return nil, fmt.Errorf("not an active member")
		}
		return &auth.SessionClaims{TenantID: orgID, Permissions: perms}, nil
	}

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(PATMiddleware(svc, resolve))
	api.Use(Protected(keys, nil))
	api.Get("/risks", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"tenant_id": c.Locals("tenant_id"), "permissions": c.Locals("permissions")})
	})
	return app
}

func callRisks(t *testing.T, app *fiber.App, bearer string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/risks", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(body, &out)
	return resp.StatusCode, out
}

// Criterion 1 of #782: the value Settings hands the user authenticates a real
// protected route, and the same call without it is refused.
func TestPAT_AuthenticatesProtectedRoute(t *testing.T) {
	ctx := context.Background()
	svc := auth.NewPersonalAccessTokenService(&patRepoFake{})
	user, tenant := uuid.New(), uuid.New()
	app := patTestServer(t, svc, map[uuid.UUID]map[uuid.UUID][]string{
		user: {tenant: {"risks:read", "assets:read"}},
	})

	_, raw, err := svc.CreateToken(ctx, user, tenant, "CI/CD", "", []string{"*"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	status, body := callRisks(t, app, raw)
	if status != fiber.StatusOK {
		t.Fatalf("with token: status %d, body %v", status, body)
	}
	if body["tenant_id"] != tenant.String() {
		t.Errorf("tenant_id = %v, want %s", body["tenant_id"], tenant)
	}
	// "*" on a non-admin member means exactly the member's permissions.
	perms, _ := body["permissions"].([]interface{})
	if len(perms) != 2 {
		t.Errorf("permissions = %v, want the owner's two", body["permissions"])
	}

	if status, _ := callRisks(t, app, ""); status != fiber.StatusUnauthorized {
		t.Errorf("without token: status %d, want 401", status)
	}
	if status, _ := callRisks(t, app, raw+"0"); status != fiber.StatusUnauthorized {
		t.Errorf("with a tampered token: status %d, want 401", status)
	}
}

// Criterion 4 of #782: a token minted in tenant A acts in A only. Its owner
// being a member of B too does not let it reach B, and once the owner leaves A
// the token stops working instead of falling back to another organization.
func TestPAT_CrossTenant(t *testing.T) {
	ctx := context.Background()
	svc := auth.NewPersonalAccessTokenService(&patRepoFake{})
	user, tenantA, tenantB := uuid.New(), uuid.New(), uuid.New()
	members := map[uuid.UUID]map[uuid.UUID][]string{
		user: {tenantA: {"*"}, tenantB: {"*"}},
	}
	app := patTestServer(t, svc, members)

	_, raw, err := svc.CreateToken(ctx, user, tenantA, "A", "", []string{"*"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	status, body := callRisks(t, app, raw)
	if status != fiber.StatusOK || body["tenant_id"] != tenantA.String() {
		t.Fatalf("got %d tenant %v, want 200 in tenant A", status, body["tenant_id"])
	}

	delete(members[user], tenantA)
	if status, body := callRisks(t, app, raw); status != fiber.StatusUnauthorized {
		t.Fatalf("owner left A: got %d tenant %v, want 401", status, body["tenant_id"])
	}
}
