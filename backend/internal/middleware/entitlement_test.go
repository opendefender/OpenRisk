// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type levelTestOrg struct {
	plan string
	err  error
}

func (o levelTestOrg) PlanAndRegion(context.Context, uuid.UUID) (string, string, error) {
	return o.plan, "eu", o.err
}

// levelApp mounts RequireLevel in front of a handler that reports whether the
// recorded depth reaches `probe`, so a test sees both the gate and what the
// handler would serve behind it.
func levelApp(svc *entapp.Service, tenant uuid.UUID, f ent.Feature, min, probe ent.Level) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if tenant != uuid.Nil {
			SetContext(c, &RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		}
		return c.Next()
	})
	app.Get("/x", RequireLevel(svc, f, min), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"full": FeatureLevelAtLeast(c, f, probe)})
	})
	return app
}

func doLevel(t *testing.T, app *fiber.App) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body := map[string]any{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body
}

func TestRequireLevel_Success(t *testing.T) {
	// Pro grants financial quantification "on": past the basic gate, and the
	// handler is told it may run the simulation.
	svc := entapp.NewService(levelTestOrg{plan: "pro"})
	app := levelApp(svc, uuid.New(), ent.FeatFinancialQuant, ent.LevelBasic, ent.LevelOn)

	status, body := doLevel(t, app)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, true, body["full"])
}

func TestRequireLevel_Insufficient(t *testing.T) {
	t.Run("feature off on the plan", func(t *testing.T) {
		svc := entapp.NewService(levelTestOrg{plan: "free"})
		app := levelApp(svc, uuid.New(), ent.FeatFinancialQuant, ent.LevelBasic, ent.LevelOn)

		status, body := doLevel(t, app)
		assert.Equal(t, fiber.StatusPaymentRequired, status)
		assert.Equal(t, "upgrade_required", body["code"])
		assert.Equal(t, "financial_quantification", body["feature"])
		assert.Equal(t, "free", body["current_plan"])
		assert.Equal(t, "pro", body["required_plan"])
	})

	t.Run("feature on, depth too shallow", func(t *testing.T) {
		// Pro has automation on; advanced is Business. RequireFeature would let
		// this through — the level gate is what refuses it.
		svc := entapp.NewService(levelTestOrg{plan: "pro"})
		app := levelApp(svc, uuid.New(), ent.FeatAutomation, ent.LevelAdvanced, ent.LevelAdvanced)

		status, body := doLevel(t, app)
		assert.Equal(t, fiber.StatusPaymentRequired, status)
		assert.Equal(t, "automation", body["feature"])
		assert.Equal(t, "pro", body["current_plan"])
		assert.Equal(t, "business", body["required_plan"])
		assert.Contains(t, body["message"], "Business")
	})

	t.Run("past the gate, below the probe", func(t *testing.T) {
		// Pro automation is on: through an "on" gate, but the handler is told
		// the plan does not reach advanced.
		svc := entapp.NewService(levelTestOrg{plan: "pro"})
		app := levelApp(svc, uuid.New(), ent.FeatAutomation, ent.LevelOn, ent.LevelAdvanced)

		status, body := doLevel(t, app)
		assert.Equal(t, fiber.StatusOK, status)
		assert.Equal(t, false, body["full"])
	})
}

func TestRequireLevel_Unauthorized(t *testing.T) {
	svc := entapp.NewService(levelTestOrg{plan: "enterprise"})
	app := levelApp(svc, uuid.Nil, ent.FeatFinancialQuant, ent.LevelBasic, ent.LevelOn)

	status, _ := doLevel(t, app)
	assert.Equal(t, fiber.StatusUnauthorized, status)
}

func TestRequireLevel_ResolverErrorFailsOpen(t *testing.T) {
	// Same posture as RequireFeature: an infrastructure hiccup does not block,
	// and with no depth recorded the handler serves the full feature.
	svc := entapp.NewService(levelTestOrg{err: errors.New("db down")})
	app := levelApp(svc, uuid.New(), ent.FeatFinancialQuant, ent.LevelBasic, ent.LevelOn)

	status, body := doLevel(t, app)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, true, body["full"])
}

func TestRequireLevel_RejectsADescriptorOrOff(t *testing.T) {
	for _, min := range []ent.Level{ent.LevelOff, ent.LevelPriority, ent.Level("99.9")} {
		assert.Panics(t, func() { RequireLevel(nil, ent.FeatSupport, min) }, "min %q", min)
	}
}
