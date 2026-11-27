// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

// #1518: a root created by registration reaches the legacy /rbac/* routes with
// no restart. Before, it had no tenants row and no user_tenants row, so
// GET /rbac/tenants/{org}, GET /rbac/users/{id} and GET /rbac/users answered
// 403/404 until the next boot's SeedRBAC repaired it.

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/internal/testsupport/registrationdb"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

type rootHasher struct{}

func (rootHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }
func (rootHasher) Verify(h, p string) bool       { return h == "hashed:"+p }

func TestRegisteredRoot_ReachesItsOwnRBACRoutes(t *testing.T) {
	db, admin := registrationdb.Open(t, true)
	// GET /rbac/tenants/{id} also counts risks and mitigations.
	for _, m := range []struct {
		table string
		model any
	}{{"risks", &domain.Risk{}}, {"mitigations", &domain.Mitigation{}}} {
		require.NoError(t, db.Exec(`CREATE TABLE `+m.table+` (id TEXT PRIMARY KEY)`).Error)
		require.NoError(t, sqliteschema.Reconcile(db, m.table, m.model))
	}

	uc := appauth.NewRegisterUseCase(repository.NewGormUserRepository(db),
		repository.NewGormOrganizationRepository(db), nil, rootHasher{}).
		WithAccounts(repository.NewGormRegistrationRepository(db))
	out, err := uc.Execute(context.Background(), appauth.RegisterInput{
		Email: "root@alpha.test", Password: "Ancre-Vitrail7-Cobalt", FullName: "Root", CompanyName: "Alpha SARL",
	})
	require.NoError(t, err)

	userSvc := service.NewUserService(db)
	tenantSvc := service.NewTenantService(db)
	users := NewRBACUserHandler(userSvc, service.NewRoleService(db), tenantSvc)
	tenants := NewRBACTenantHandler(tenantSvc, userSvc)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		// What middleware.Protected sets for the freshly registered root.
		c.Locals("tenantID", out.Organization.ID)
		c.Locals("userID", out.User.ID)
		return c.Next()
	})
	app.Get("/rbac/tenants/:tenant_id", tenants.GetTenant)
	app.Get("/rbac/users/:user_id", users.GetUser)
	app.Get("/rbac/users", users.ListUsers)

	get := func(path string) (int, []byte) {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil), 5000)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, b
	}

	code, _ := get("/rbac/tenants/" + out.Organization.ID.String())
	assert.Equal(t, fiber.StatusOK, code)
	code, _ = get("/rbac/users/" + out.User.ID.String())
	assert.Equal(t, fiber.StatusOK, code)

	code, body := get("/rbac/users")
	require.Equal(t, fiber.StatusOK, code)
	var list struct {
		Users []domain.UserTenant `json:"users"`
	}
	require.NoError(t, json.Unmarshal(body, &list))
	require.Len(t, list.Users, 1)
	assert.Equal(t, out.User.ID, list.Users[0].UserID)
	assert.Equal(t, admin.ID, list.Users[0].RoleID)
	require.NotNil(t, list.Users[0].Role)
	assert.Equal(t, "Admin", list.Users[0].Role.Name)
}
