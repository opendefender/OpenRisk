// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
	"github.com/stretchr/testify/assert"
)

// #783 under D-078: Free does almost everything, capped by count. The route
// caps below are mounted in main.go (see handler/hosted_plan_routes_test.go);
// each must admit a plan under its cap and refuse it at the cap, with the
// numbers. Threat intel is the one remaining plan gate until its preview
// quota ships.

// gateApp mounts a gate in front of a handler that answers 200.
func gateApp(tenant uuid.UUID, gate fiber.Handler) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if tenant != uuid.Nil {
			SetContext(c, &RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		}
		return c.Next()
	})
	app.Get("/x", gate, func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	return app
}

type fixedUsage map[ent.LimitKey]int

func (u fixedUsage) Count(_ context.Context, _ uuid.UUID, k ent.LimitKey) (int, error) {
	return u[k], nil
}

// routeCaps are the caps mounted as middleware, with the D-078 grid per plan
// (-1 = unlimited).
var routeCaps = []struct {
	key  ent.LimitKey
	caps map[string]int
}{
	{ent.LimitUsers, map[string]int{"free": 2, "pro": 10, "business": 50, "enterprise": -1}},
	{ent.LimitFrameworks, map[string]int{"free": 1, "pro": 3, "business": 10, "enterprise": -1}},
	{ent.LimitScanSources, map[string]int{"free": 1, "pro": 10, "business": -1, "enterprise": -1}},
	{ent.LimitApprovalWorkflows, map[string]int{"free": 1, "pro": 3, "business": -1, "enterprise": -1}},
}

func TestPlanCaps_Success(t *testing.T) {
	for _, rc := range routeCaps {
		for plan, limit := range rc.caps {
			used := limit - 1
			if limit < 0 {
				used = 500
			}
			svc := entapp.NewService(levelTestOrg{plan: plan}).WithUsage(fixedUsage{rc.key: used})
			status, _ := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, rc.key)))
			assert.Equal(t, fiber.StatusOK, status, "%s on %s with %d used", rc.key, plan, used)
		}
	}
}

func TestPlanCaps_Unauthorized(t *testing.T) {
	for _, rc := range routeCaps {
		for plan, limit := range rc.caps {
			if limit < 0 {
				continue
			}
			svc := entapp.NewService(levelTestOrg{plan: plan}).WithUsage(fixedUsage{rc.key: limit})
			status, body := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, rc.key)))
			assert.Equal(t, fiber.StatusPaymentRequired, status, "%s on %s at %d", rc.key, plan, limit)
			assert.Equal(t, "limit_reached", body["code"])
			assert.Equal(t, string(rc.key), body["limit_key"])
			assert.EqualValues(t, limit, body["limit"])
			assert.EqualValues(t, limit, body["used"])
		}
	}
}

func TestPlanCaps_NotFound(t *testing.T) {
	// An unreadable plan fails OPEN, like every entitlement gate.
	for _, rc := range routeCaps {
		svc := entapp.NewService(levelTestOrg{err: domain.ErrNotFound}).WithUsage(fixedUsage{rc.key: 99})
		status, _ := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, rc.key)))
		assert.Equal(t, fiber.StatusOK, status, "%s", rc.key)
	}
}

func TestThreatIntelGate_Success(t *testing.T) {
	for _, plan := range []string{"business", "enterprise"} {
		svc := entapp.NewService(levelTestOrg{plan: plan})
		status, _ := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, ent.FeatCTI)))
		assert.Equal(t, fiber.StatusOK, status, plan)
	}
}

func TestThreatIntelGate_Unauthorized(t *testing.T) {
	for _, plan := range []string{"free", "pro"} {
		svc := entapp.NewService(levelTestOrg{plan: plan})
		status, body := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, ent.FeatCTI)))
		assert.Equal(t, fiber.StatusPaymentRequired, status, plan)
		assert.Equal(t, "upgrade_required", body["code"])
		assert.Equal(t, "cti", body["feature"])
		assert.Equal(t, "business", body["required_plan"])
	}
}

func TestThreatIntelGate_NotFound(t *testing.T) {
	svc := entapp.NewService(levelTestOrg{err: domain.ErrNotFound})
	status, _ := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, ent.FeatCTI)))
	assert.Equal(t, fiber.StatusOK, status)
}

// TestFrameworkCap_Message keeps the French copy the 402 carries.
func TestFrameworkCap_Message(t *testing.T) {
	svc := entapp.NewService(levelTestOrg{plan: "free"}).WithUsage(fixedUsage{ent.LimitFrameworks: 1})
	_, body := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, ent.LimitFrameworks)))
	assert.Contains(t, body["message"], "référentiels actifs")
}
