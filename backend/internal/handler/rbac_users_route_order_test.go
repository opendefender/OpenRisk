// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// #1519: Fiber matches in registration order, so GET /rbac/users/:user_id
// registered first swallowed "stats" and answered 400 for every caller.

// The composition root must register the static path first.
func TestRBACUsersRoutes_StatsIsMountedBeforeUserID(t *testing.T) {
	src := compositionRoot(t)
	stats := strings.Index(src, `rbacUsers.Get("/stats", rbacUserHandler.GetTenantUserStats)`)
	byID := strings.Index(src, `rbacUsers.Get("/:user_id", rbacUserHandler.GetUser)`)
	require.GreaterOrEqual(t, stats, 0, "GET /rbac/users/stats is not mounted")
	require.GreaterOrEqual(t, byID, 0, "GET /rbac/users/:user_id is not mounted")
	require.Less(t, stats, byID, "the static /stats must be registered before /:user_id")
}

// The behaviour the order buys, on a router built the same way.
func TestRBACUsersRoutes_OrderDecidesWhichHandlerAnswers(t *testing.T) {
	build := func(statsFirst bool) *fiber.App {
		app := fiber.New()
		g := app.Group("/rbac/users")
		stats := func(c *fiber.Ctx) error { return c.SendString("stats") }
		byID := func(c *fiber.Ctx) error { return c.SendString("user:" + c.Params("user_id")) }
		if statsFirst {
			g.Get("/stats", stats)
			g.Get("/:user_id", byID)
		} else {
			g.Get("/:user_id", byID)
			g.Get("/stats", stats)
		}
		return app
	}
	body := func(app *fiber.App, path string) string {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		defer resp.Body.Close()
		b := make([]byte, 64)
		n, _ := resp.Body.Read(b)
		return string(b[:n])
	}
	const id = "3f2b1c9e-6a54-4d0b-9c1e-0b1b5a7d2e11"

	ok := build(true)
	require.Equal(t, "stats", body(ok, "/rbac/users/stats"))
	require.Equal(t, "user:"+id, body(ok, "/rbac/users/"+id))

	require.Equal(t, "user:stats", body(build(false), "/rbac/users/stats"), "the old order shadows stats")
}
