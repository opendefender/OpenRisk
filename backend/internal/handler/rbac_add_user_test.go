// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

// #1518 review: POST /rbac/users bound ANY user id into the caller's tenant, and
// the stranger then saw that tenant in their own tenant list. The target must be
// an active member of the caller's organisation; anyone else is 404, the same
// for another organisation's user and for an id that does not exist.

import (
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

func addUserApp(t *testing.T, caller, tenant uuid.UUID, db *gorm.DB) *fiber.App {
	t.Helper()
	userSvc := service.NewUserService(db)
	h := NewRBACUserHandler(userSvc, service.NewRoleService(db), service.NewTenantService(db))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenantID", tenant)
		c.Locals("userID", caller)
		return c.Next()
	})
	app.Post("/rbac/users", h.AddUserToTenant)
	return app
}

func TestAddUserToTenant_Success_Unauthorized_NotFound(t *testing.T) {
	db, admin := registrationdb.Open(t, true)
	viewer := domain.RoleEnhanced{ID: uuid.New(), Name: "Viewer", Level: domain.RoleLevelViewer, IsPredefined: true, IsActive: true}
	require.NoError(t, db.Create(&viewer).Error)

	orgA, orgB := uuid.New(), uuid.New()
	adminA, colleague, viewerA, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, m := range []domain.OrganizationMember{
		{ID: uuid.New(), OrganizationID: orgA, UserID: adminA, Role: domain.RoleRoot, IsActive: true},
		{ID: uuid.New(), OrganizationID: orgA, UserID: colleague, Role: domain.RoleUser, IsActive: true},
		{ID: uuid.New(), OrganizationID: orgB, UserID: foreign, Role: domain.RoleRoot, IsActive: true},
	} {
		require.NoError(t, db.Create(&m).Error)
	}
	require.NoError(t, db.Create(&domain.UserTenant{UserID: adminA, TenantID: orgA, RoleID: admin.ID}).Error)
	require.NoError(t, db.Create(&domain.UserTenant{UserID: viewerA, TenantID: orgA, RoleID: viewer.ID}).Error)

	app := addUserApp(t, adminA, orgA, db)
	body := func(u uuid.UUID) map[string]string {
		return map[string]string{"user_id": u.String(), "role_id": viewer.ID.String()}
	}

	// Success: a member of the caller's own organisation.
	assert.Equal(t, fiber.StatusCreated, rbacRoleCall(t, app, "POST", "/rbac/users", body(colleague)))
	var n int64
	require.NoError(t, db.Model(&domain.UserTenant{}).Where("user_id = ? AND tenant_id = ?", colleague, orgA).Count(&n).Error)
	assert.EqualValues(t, 1, n)

	// NotFound: another organisation's user and a random id answer alike, and
	// no row is written.
	assert.Equal(t, fiber.StatusNotFound, rbacRoleCall(t, app, "POST", "/rbac/users", body(foreign)))
	assert.Equal(t, fiber.StatusNotFound, rbacRoleCall(t, app, "POST", "/rbac/users", body(uuid.New())))
	require.NoError(t, db.Model(&domain.UserTenant{}).Where("user_id = ?", foreign).Count(&n).Error)
	assert.EqualValues(t, 0, n, "the foreign user must not see this tenant")

	// Unauthorized: a viewer of the tenant may not add anyone.
	viewerApp := addUserApp(t, viewerA, orgA, db)
	assert.Equal(t, fiber.StatusForbidden, rbacRoleCall(t, viewerApp, "POST", "/rbac/users", body(colleague)))
}
