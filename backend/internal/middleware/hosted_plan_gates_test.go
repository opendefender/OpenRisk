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

// The four hosted plan gates of #783 (D-075, ADR 0007), each with the plan that
// opens it. Free is refused all four; the opening plan passes; for the two
// Business gates, Pro is refused too.
var hostedGates = []struct {
	feature ent.Feature
	opensAt string
}{
	{ent.FeatAutomation, "pro"},
	{ent.FeatScanner, "pro"},
	{ent.FeatCTI, "business"},
	{ent.FeatGovernance, "business"},
}

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

func TestHostedPlanGates_Success(t *testing.T) {
	for _, g := range hostedGates {
		for _, plan := range ent.AllPlans {
			if !plan.AtLeast(ent.ParsePlan(g.opensAt)) {
				continue
			}
			svc := entapp.NewService(levelTestOrg{plan: string(plan)})
			status, _ := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, g.feature)))
			assert.Equal(t, fiber.StatusOK, status, "%s on %s", g.feature, plan)
		}
	}
}

func TestHostedPlanGates_Unauthorized(t *testing.T) {
	for _, g := range hostedGates {
		for _, plan := range ent.AllPlans {
			if plan.AtLeast(ent.ParsePlan(g.opensAt)) {
				continue
			}
			svc := entapp.NewService(levelTestOrg{plan: string(plan)})
			status, body := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, g.feature)))
			assert.Equal(t, fiber.StatusPaymentRequired, status, "%s on %s", g.feature, plan)
			assert.Equal(t, "upgrade_required", body["code"])
			assert.Equal(t, string(g.feature), body["feature"])
			assert.Equal(t, string(plan), body["current_plan"])
			assert.Equal(t, g.opensAt, body["required_plan"])
		}
	}
}

func TestHostedPlanGates_NotFound(t *testing.T) {
	// A tenant the resolver cannot read fails OPEN, like every entitlement
	// gate: an infrastructure hiccup must not lock a paying tenant out.
	for _, g := range hostedGates {
		svc := entapp.NewService(levelTestOrg{err: domain.ErrNotFound})
		status, _ := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, g.feature)))
		assert.Equal(t, fiber.StatusOK, status, "%s", g.feature)
	}
}

type frameworkUsage int

func (n frameworkUsage) Count(_ context.Context, _ uuid.UUID, k ent.LimitKey) (int, error) {
	if k == ent.LimitFrameworks {
		return int(n), nil
	}
	return 0, nil
}

// TestFrameworkCap_* drive RequireCapacity on LimitFrameworks, the gate
// mounted on POST /compliance/frameworks (D-076).
func TestFrameworkCap_Success(t *testing.T) {
	for plan, used := range map[string]int{"free": 0, "pro": 2, "business": 9, "enterprise": 500} {
		svc := entapp.NewService(levelTestOrg{plan: plan}).WithUsage(frameworkUsage(used))
		status, _ := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, ent.LimitFrameworks)))
		assert.Equal(t, fiber.StatusOK, status, "%s with %d frameworks", plan, used)
	}
}

func TestFrameworkCap_Unauthorized(t *testing.T) {
	for plan, limit := range map[string]int{"free": 1, "pro": 3, "business": 10} {
		svc := entapp.NewService(levelTestOrg{plan: plan}).WithUsage(frameworkUsage(limit))
		status, body := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, ent.LimitFrameworks)))
		assert.Equal(t, fiber.StatusPaymentRequired, status, "%s at %d", plan, limit)
		assert.Equal(t, "limit_reached", body["code"])
		assert.Equal(t, "frameworks", body["limit_key"])
		assert.EqualValues(t, limit, body["limit"])
		assert.EqualValues(t, limit, body["used"])
		assert.Contains(t, body["message"], "référentiels actifs")
	}
}

func TestFrameworkCap_NotFound(t *testing.T) {
	svc := entapp.NewService(levelTestOrg{err: domain.ErrNotFound}).WithUsage(frameworkUsage(99))
	status, _ := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, ent.LimitFrameworks)))
	assert.Equal(t, fiber.StatusOK, status)
}
