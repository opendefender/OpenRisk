// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

// #1520: role change and removal answered 200 for a user with no row in the
// caller's tenant. Foreign and unknown ids are now one 404 with one body.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/internal/testsupport/registrationdb"
)

func userMutationApp(caller, tenant uuid.UUID, db *gorm.DB) *fiber.App {
	h := NewRBACUserHandler(service.NewUserService(db), service.NewRoleService(db), service.NewTenantService(db))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenantID", tenant)
		c.Locals("userID", caller)
		return c.Next()
	})
	app.Patch("/rbac/users/:user_id/role", h.ChangeUserRole)
	app.Delete("/rbac/users/:user_id", h.RemoveUserFromTenant)
	return app
}

func userMutationCall(t *testing.T, app *fiber.App, method, path string, body any) (int, string) {
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
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(b)
}

type userMutationFixture struct {
	db                                   *gorm.DB
	orgA, orgB                           uuid.UUID
	adminA, adminA2, memberA, viewerA, b uuid.UUID
	viewerRole, analystRole              domain.RoleEnhanced
}

func newUserMutationFixture(t *testing.T) *userMutationFixture {
	t.Helper()
	db, admin := registrationdb.Open(t, true)
	f := &userMutationFixture{db: db, orgA: uuid.New(), orgB: uuid.New(),
		adminA: uuid.New(), adminA2: uuid.New(), memberA: uuid.New(), viewerA: uuid.New(), b: uuid.New()}
	f.viewerRole = domain.RoleEnhanced{ID: uuid.New(), Name: "Viewer", Level: domain.RoleLevelViewer, IsPredefined: true, IsActive: true}
	f.analystRole = domain.RoleEnhanced{ID: uuid.New(), Name: "Analyst", Level: domain.RoleLevelAnalyst, IsPredefined: true, IsActive: true}
	require.NoError(t, db.Create(&f.viewerRole).Error)
	require.NoError(t, db.Create(&f.analystRole).Error)
	for _, ut := range []domain.UserTenant{
		{UserID: f.adminA, TenantID: f.orgA, RoleID: admin.ID},
		{UserID: f.adminA2, TenantID: f.orgA, RoleID: admin.ID},
		{UserID: f.memberA, TenantID: f.orgA, RoleID: f.viewerRole.ID},
		{UserID: f.viewerA, TenantID: f.orgA, RoleID: f.viewerRole.ID},
		{UserID: f.b, TenantID: f.orgB, RoleID: f.viewerRole.ID},
	} {
		require.NoError(t, db.Create(&ut).Error)
	}
	return f
}

func (f *userMutationFixture) roleOf(t *testing.T, user, tenant uuid.UUID) (uuid.UUID, int64) {
	t.Helper()
	var rows []domain.UserTenant
	require.NoError(t, f.db.Where("user_id = ? AND tenant_id = ?", user, tenant).Find(&rows).Error)
	if len(rows) == 0 {
		return uuid.Nil, 0
	}
	return rows[0].RoleID, int64(len(rows))
}

func TestChangeUserRole_Success(t *testing.T) {
	f := newUserMutationFixture(t)
	app := userMutationApp(f.adminA, f.orgA, f.db)
	status, _ := userMutationCall(t, app, "PATCH", "/rbac/users/"+f.memberA.String()+"/role",
		map[string]string{"role_id": f.analystRole.ID.String()})
	assert.Equal(t, fiber.StatusOK, status)
	role, _ := f.roleOf(t, f.memberA, f.orgA)
	assert.Equal(t, f.analystRole.ID, role)
}

func TestChangeUserRole_NotFound(t *testing.T) {
	f := newUserMutationFixture(t)
	app := userMutationApp(f.adminA, f.orgA, f.db)
	body := map[string]string{"role_id": f.analystRole.ID.String()}
	foreignStatus, foreignBody := userMutationCall(t, app, "PATCH", "/rbac/users/"+f.b.String()+"/role", body)
	unknownStatus, unknownBody := userMutationCall(t, app, "PATCH", "/rbac/users/"+uuid.New().String()+"/role", body)
	assert.Equal(t, fiber.StatusNotFound, foreignStatus)
	assert.Equal(t, foreignStatus, unknownStatus)
	assert.Equal(t, foreignBody, unknownBody)
	role, _ := f.roleOf(t, f.b, f.orgB)
	assert.Equal(t, f.viewerRole.ID, role, "B's membership must be unchanged")
}

func TestChangeUserRole_Unauthorized(t *testing.T) {
	f := newUserMutationFixture(t)
	app := userMutationApp(f.viewerA, f.orgA, f.db)
	status, _ := userMutationCall(t, app, "PATCH", "/rbac/users/"+f.memberA.String()+"/role",
		map[string]string{"role_id": f.analystRole.ID.String()})
	assert.Equal(t, fiber.StatusForbidden, status)
	role, _ := f.roleOf(t, f.memberA, f.orgA)
	assert.Equal(t, f.viewerRole.ID, role)
}

func TestRemoveUserFromTenant_Success(t *testing.T) {
	f := newUserMutationFixture(t)
	app := userMutationApp(f.adminA, f.orgA, f.db)
	status, _ := userMutationCall(t, app, "DELETE", "/rbac/users/"+f.memberA.String(), nil)
	assert.Equal(t, fiber.StatusOK, status)
	_, n := f.roleOf(t, f.memberA, f.orgA)
	assert.EqualValues(t, 0, n)

	// A same-tenant admin still cannot be removed.
	status, _ = userMutationCall(t, app, "DELETE", "/rbac/users/"+f.adminA2.String(), nil)
	assert.Equal(t, fiber.StatusBadRequest, status)
	_, n = f.roleOf(t, f.adminA2, f.orgA)
	assert.EqualValues(t, 1, n)
}

func TestRemoveUserFromTenant_NotFound(t *testing.T) {
	f := newUserMutationFixture(t)
	app := userMutationApp(f.adminA, f.orgA, f.db)
	foreignStatus, foreignBody := userMutationCall(t, app, "DELETE", "/rbac/users/"+f.b.String(), nil)
	unknownStatus, unknownBody := userMutationCall(t, app, "DELETE", "/rbac/users/"+uuid.New().String(), nil)
	assert.Equal(t, fiber.StatusNotFound, foreignStatus)
	assert.Equal(t, foreignStatus, unknownStatus)
	assert.Equal(t, foreignBody, unknownBody)
	_, n := f.roleOf(t, f.b, f.orgB)
	assert.EqualValues(t, 1, n, "B's membership must survive")
}

func TestRemoveUserFromTenant_Unauthorized(t *testing.T) {
	f := newUserMutationFixture(t)
	app := userMutationApp(f.viewerA, f.orgA, f.db)
	status, _ := userMutationCall(t, app, "DELETE", "/rbac/users/"+f.memberA.String(), nil)
	assert.Equal(t, fiber.StatusForbidden, status)
	_, n := f.roleOf(t, f.memberA, f.orgA)
	assert.EqualValues(t, 1, n)
}
