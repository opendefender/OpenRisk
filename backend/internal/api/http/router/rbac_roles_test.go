// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package router_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/api/http/router"
	handlers "github.com/opendefender/openrisk/internal/handler"
)

// buildRBACRoles builds the /rbac/roles routes the way main.go will, with a
// zero-value handler: no database, no Redis. The handler's methods are only
// referenced at mount time, never called by the matrix test.
func buildRBACRoles(t *testing.T, signedInAs string) (*fiber.App, *router.Matrix) {
	t.Helper()
	app := fiber.New()
	api := app.Group("/api/v1")
	if signedInAs != "" {
		// Stands in for middleware.Protected: publishes the org role the way
		// AuthMiddlewareRS256 does.
		api.Use(func(c *fiber.Ctx) error {
			c.Locals("org_roles", map[uuid.UUID]string{uuid.New(): signedInAs})
			return c.Next()
		})
	}
	var mx router.Matrix
	protected := router.New(api, "/api/v1", router.Authenticated, &mx)
	router.RegisterRBACRoleRoutes(protected, &handlers.RBACRoleHandler{})
	return app, &mx
}

func TestRBACRoleRoutes_MatrixIsReadOffTheBuiltRouter(t *testing.T) {
	app, mx := buildRBACRoles(t, "")
	require.NoError(t, router.Verify(app, mx))

	var lines []string
	for _, r := range mx.Rows() {
		lines = append(lines, r.String())
		t.Log(r.String())
	}
	assert.Equal(t, []string{
		"GET    /api/v1/rbac/roles                       authenticated → role(admin)",
		"POST   /api/v1/rbac/roles                       authenticated → role(admin)",
		"GET    /api/v1/rbac/roles/:role_id              authenticated → role(admin)",
		"PATCH  /api/v1/rbac/roles/:role_id              authenticated → role(admin)",
		"DELETE /api/v1/rbac/roles/:role_id              authenticated → role(admin)",
		"GET    /api/v1/rbac/roles/:role_id/permissions  authenticated → role(admin)",
		"POST   /api/v1/rbac/roles/:role_id/permissions  authenticated → role(admin)",
		"DELETE /api/v1/rbac/roles/:role_id/permissions  authenticated → role(admin)",
	}, lines)
}

// The record says role(admin); the built router must refuse a member on every
// one of those routes. This is the existing RequireRole doing the refusing.
func TestRBACRoleRoutes_MemberIsRefusedOnEveryRow(t *testing.T) {
	app, mx := buildRBACRoles(t, "user")
	for _, r := range mx.Rows() {
		path := strings.ReplaceAll(r.Path, ":role_id", uuid.NewString())
		resp, err := app.Test(httptest.NewRequest(r.Method, path, nil))
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusForbidden, resp.StatusCode, r.String())
	}
}

// A route mounted on the Fiber router directly, around the Mounter, has no
// row. Verify must say so, or the matrix could silently under-report.
func TestVerify_RejectsARouteMountedAroundTheMounter(t *testing.T) {
	app, mx := buildRBACRoles(t, "")
	app.Delete("/api/v1/rbac/roles/:role_id/everything", func(c *fiber.Ctx) error { return nil })

	err := router.Verify(app, mx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DELETE /api/v1/rbac/roles/:role_id/everything: mounted 1 time(s), recorded 0")
}
