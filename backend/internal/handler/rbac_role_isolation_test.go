// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// Regression tests for GHSA-6m8w-33j5-ff68.
//
// The /rbac/roles/:role_id routes checked that the caller administers their own
// tenant, then loaded and changed the role by its ID alone. An administrator of
// tenant A who knew the UUID of a custom role of tenant B could read it, rename
// it, deactivate it and add or remove its permissions.
//
// These tests drive the real handler and services over a real Fiber app, with
// the caller's tenant set the way middleware.Protected sets it, and check both
// the status code and the database afterwards: a 404 that still wrote the row
// would not be a fix.

type rbacRoleFixture struct {
	db         *gorm.DB
	tenantA    uuid.UUID
	tenantB    uuid.UUID
	adminA     uuid.UUID // administrator of tenant A only
	viewerA    uuid.UUID // viewer of tenant A
	predefined domain.RoleEnhanced
	ownRole    domain.RoleEnhanced // custom role of tenant A
	victimRole domain.RoleEnhanced // custom role of tenant B
	permission domain.PermissionDB
}

func newRBACRoleFixture(t *testing.T) *rbacRoleFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Hand-written keys only; sqliteschema adds the model's other columns.
	require.NoError(t, db.Exec(`CREATE TABLE roles (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "roles", &domain.RoleEnhanced{}))
	require.NoError(t, db.Exec(`CREATE TABLE permissions (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "permissions", &domain.PermissionDB{}))
	require.NoError(t, db.Exec(`CREATE TABLE role_permissions (role_id TEXT, permission_id TEXT, PRIMARY KEY (role_id, permission_id))`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "role_permissions", &domain.RolePermission{}))
	require.NoError(t, db.Exec(`CREATE TABLE user_tenants (user_id TEXT, tenant_id TEXT, PRIMARY KEY (user_id, tenant_id))`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "user_tenants", &domain.UserTenant{}))

	f := &rbacRoleFixture{
		db:      db,
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		adminA:  uuid.New(),
		viewerA: uuid.New(),
	}
	f.predefined = domain.RoleEnhanced{ID: uuid.New(), Name: "Admin", Level: domain.RoleLevelAdmin, IsPredefined: true, IsActive: true}
	viewer := domain.RoleEnhanced{ID: uuid.New(), Name: "Viewer", Level: domain.RoleLevelViewer, IsPredefined: true, IsActive: true}
	f.ownRole = domain.RoleEnhanced{ID: uuid.New(), TenantID: f.tenantA, Name: "Auditors A", Level: domain.RoleLevelViewer, IsActive: true}
	f.victimRole = domain.RoleEnhanced{ID: uuid.New(), TenantID: f.tenantB, Name: "Auditors B", Level: domain.RoleLevelViewer, IsActive: true}
	for _, r := range []*domain.RoleEnhanced{&f.predefined, &viewer, &f.ownRole, &f.victimRole} {
		require.NoError(t, db.Create(r).Error)
	}

	f.permission = domain.PermissionDB{ID: uuid.New(), Resource: "risk", Action: "delete"}
	require.NoError(t, db.Create(&f.permission).Error)

	require.NoError(t, db.Create(&domain.UserTenant{UserID: f.adminA, TenantID: f.tenantA, RoleID: f.predefined.ID}).Error)
	require.NoError(t, db.Create(&domain.UserTenant{UserID: f.viewerA, TenantID: f.tenantA, RoleID: viewer.ID}).Error)
	return f
}

// app mounts the role routes as main.go does, behind a stub that sets the
// caller's identity: a member of tenant A.
func (f *rbacRoleFixture) app(caller uuid.UUID) *fiber.App {
	h := NewRBACRoleHandler(service.NewRoleService(f.db), service.NewPermissionService(), service.NewUserService(f.db))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenantID", f.tenantA)
		c.Locals("userID", caller)
		return c.Next()
	})
	app.Get("/rbac/roles/:role_id", h.GetRole)
	app.Patch("/rbac/roles/:role_id", h.UpdateRole)
	app.Delete("/rbac/roles/:role_id", h.DeleteRole)
	app.Get("/rbac/roles/:role_id/permissions", h.GetRolePermissions)
	app.Post("/rbac/roles/:role_id/permissions", h.AssignPermissionToRole)
	app.Delete("/rbac/roles/:role_id/permissions", h.RemovePermissionFromRole)
	return app
}

func rbacRoleCall(t *testing.T, app *fiber.App, method, path string, body any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func (f *rbacRoleFixture) reload(t *testing.T, id uuid.UUID) domain.RoleEnhanced {
	t.Helper()
	var r domain.RoleEnhanced
	require.NoError(t, f.db.First(&r, "id = ?", id).Error)
	return r
}

func (f *rbacRoleFixture) linkCount(t *testing.T, roleID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.db.Model(&domain.RolePermission{}).Where("role_id = ?", roleID).Count(&n).Error)
	return n
}

func TestRBACRoleRoutes_Success(t *testing.T) {
	f := newRBACRoleFixture(t)
	app := f.app(f.adminA)
	own := "/rbac/roles/" + f.ownRole.ID.String()
	perm := map[string]string{"permission_id": f.permission.ID.String()}

	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "GET", own, nil))
	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "GET", own+"/permissions", nil))
	// A predefined role is shared by every tenant, so it stays readable.
	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "GET", "/rbac/roles/"+f.predefined.ID.String(), nil))

	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "POST", own+"/permissions", perm))
	require.EqualValues(t, 1, f.linkCount(t, f.ownRole.ID))
	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "DELETE", own+"/permissions", perm))
	require.EqualValues(t, 0, f.linkCount(t, f.ownRole.ID))

	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "PATCH", own,
		map[string]any{"name": "Renamed", "description": "d", "is_active": true}))
	require.Equal(t, "Renamed", f.reload(t, f.ownRole.ID).Name)

	// A partial PATCH changes only what it sends, and can deactivate.
	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "PATCH", own, map[string]any{"is_active": false}))
	partial := f.reload(t, f.ownRole.ID)
	require.Equal(t, "Renamed", partial.Name)
	require.Equal(t, "d", partial.Description)
	require.False(t, partial.IsActive)
	require.Equal(t, fiber.StatusBadRequest, rbacRoleCall(t, app, "PATCH", own, map[string]any{"name": ""}))
	require.Equal(t, fiber.StatusBadRequest, rbacRoleCall(t, app, "PATCH", own, map[string]any{"name": "   "}))
	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "PATCH", own, map[string]any{"is_active": true}))

	require.Equal(t, fiber.StatusOK, rbacRoleCall(t, app, "DELETE", own, nil))
	require.False(t, f.reload(t, f.ownRole.ID).IsActive)
}

func TestRBACRoleRoutes_HeldRoleConflict(t *testing.T) {
	f := newRBACRoleFixture(t)
	app := f.app(f.adminA)
	held := domain.RoleEnhanced{ID: uuid.New(), TenantID: f.tenantA, Name: "Held", Level: domain.RoleLevelViewer, IsActive: true}
	require.NoError(t, f.db.Create(&held).Error)
	require.NoError(t, f.db.Create(&domain.UserTenant{UserID: uuid.New(), TenantID: f.tenantA, RoleID: held.ID}).Error)
	path := "/rbac/roles/" + held.ID.String()

	// Deactivating or deleting a role someone holds would leave them its rights.
	require.Equal(t, fiber.StatusConflict, rbacRoleCall(t, app, "PATCH", path, map[string]any{"is_active": false}))
	require.Equal(t, fiber.StatusConflict, rbacRoleCall(t, app, "DELETE", path, nil))
	require.True(t, f.reload(t, held.ID).IsActive)
}

func TestRBACRoleRoutes_NotFound(t *testing.T) {
	f := newRBACRoleFixture(t)
	app := f.app(f.adminA)
	victim := "/rbac/roles/" + f.victimRole.ID.String()
	perm := map[string]string{"permission_id": f.permission.ID.String()}
	require.NoError(t, f.db.Create(&domain.RolePermission{RoleID: f.victimRole.ID, PermissionID: f.permission.ID}).Error)

	// Every route answers 404 for another tenant's role, as for a missing one.
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", victim, nil},
		{"GET", victim + "/permissions", nil},
		{"PATCH", victim, map[string]any{"name": "pwned", "is_active": false}},
		{"DELETE", victim + "/permissions", perm},
		{"POST", victim + "/permissions", map[string]string{"permission_id": uuid.NewString()}},
		{"DELETE", victim, nil},
		{"GET", "/rbac/roles/" + uuid.NewString(), nil},
	} {
		require.Equal(t, fiber.StatusNotFound, rbacRoleCall(t, app, tc.method, tc.path, tc.body), tc.method+" "+tc.path)
	}

	// And nothing of tenant B changed.
	after := f.reload(t, f.victimRole.ID)
	require.Equal(t, "Auditors B", after.Name)
	require.True(t, after.IsActive)
	require.EqualValues(t, 1, f.linkCount(t, f.victimRole.ID))
}

func TestRBACRoleRoutes_Unauthorized(t *testing.T) {
	f := newRBACRoleFixture(t)
	perm := map[string]string{"permission_id": f.permission.ID.String()}

	// A predefined role is shared by every tenant: changing it from one tenant
	// would change it for all of them.
	admin := f.app(f.adminA)
	shared := "/rbac/roles/" + f.predefined.ID.String()
	require.Equal(t, fiber.StatusForbidden, rbacRoleCall(t, admin, "PATCH", shared, map[string]any{"name": "x", "is_active": false}))
	require.Equal(t, fiber.StatusForbidden, rbacRoleCall(t, admin, "POST", shared+"/permissions", perm))
	require.Equal(t, fiber.StatusForbidden, rbacRoleCall(t, admin, "DELETE", shared+"/permissions", perm))
	require.Equal(t, fiber.StatusForbidden, rbacRoleCall(t, admin, "DELETE", shared, nil))
	require.Equal(t, "Admin", f.reload(t, f.predefined.ID).Name)
	require.True(t, f.reload(t, f.predefined.ID).IsActive)

	// A viewer of tenant A cannot change even tenant A's own roles.
	viewer := f.app(f.viewerA)
	own := "/rbac/roles/" + f.ownRole.ID.String()
	require.Equal(t, fiber.StatusForbidden, rbacRoleCall(t, viewer, "POST", own+"/permissions", perm))
	require.Equal(t, fiber.StatusForbidden, rbacRoleCall(t, viewer, "DELETE", own, nil))
	require.EqualValues(t, 0, f.linkCount(t, f.ownRole.ID))
	require.True(t, f.reload(t, f.ownRole.ID).IsActive)

	// A session with no tenant fails closed.
	noTenant := fiber.New()
	h := NewRBACRoleHandler(service.NewRoleService(f.db), service.NewPermissionService(), service.NewUserService(f.db))
	noTenant.Get("/rbac/roles/:role_id", h.GetRole)
	noTenant.Patch("/rbac/roles/:role_id", h.UpdateRole)
	noTenant.Delete("/rbac/roles/:role_id", h.DeleteRole)
	noTenant.Get("/rbac/roles/:role_id/permissions", h.GetRolePermissions)
	noTenant.Post("/rbac/roles/:role_id/permissions", h.AssignPermissionToRole)
	noTenant.Delete("/rbac/roles/:role_id/permissions", h.RemovePermissionFromRole)
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/rbac/roles/" + f.predefined.ID.String(), nil},
		{"PATCH", own, map[string]any{"name": "x", "is_active": false}},
		{"DELETE", own, nil},
		{"GET", own + "/permissions", nil},
		{"POST", own + "/permissions", perm},
		{"DELETE", own + "/permissions", perm},
	} {
		require.Equal(t, fiber.StatusUnauthorized, rbacRoleCall(t, noTenant, tc.method, tc.path, tc.body), tc.method+" "+tc.path)
	}
	require.Equal(t, "Auditors A", f.reload(t, f.ownRole.ID).Name)
	require.True(t, f.reload(t, f.ownRole.ID).IsActive)
	require.EqualValues(t, 0, f.linkCount(t, f.ownRole.ID))
}
