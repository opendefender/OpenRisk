// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capacityTestUsage int

func (u capacityTestUsage) Count(context.Context, uuid.UUID, ent.LimitKey) (int, error) {
	return int(u), nil
}

func capacityStatus(t *testing.T, svc *entapp.Service) int {
	t.Helper()
	app := fiber.New()
	tenant := uuid.New()
	app.Use(func(c *fiber.Ctx) error {
		SetContext(c, &RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		return c.Next()
	})
	app.Post("/users", RequireCapacity(svc, ent.LimitUsers), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})
	resp, err := app.Test(httptest.NewRequest("POST", "/users", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// #612 criterion 5: a self-hosted tenant far past Free's user cap still creates.
func TestRequireCapacity_SelfHostedNeverRefuses(t *testing.T) {
	svc := entapp.NewService(levelTestOrg{plan: "free"}).
		WithUsage(capacityTestUsage(1000)).
		WithDeployment(ent.DeploymentSelfHosted)
	assert.Equal(t, fiber.StatusCreated, capacityStatus(t, svc))
}

// The same tenant on the hosted deployment is refused at Free's cap.
func TestRequireCapacity_HostedFreeStillCapped(t *testing.T) {
	svc := entapp.NewService(levelTestOrg{plan: "free"}).
		WithUsage(capacityTestUsage(2)).
		WithDeployment(ent.DeploymentHosted)
	assert.Equal(t, fiber.StatusPaymentRequired, capacityStatus(t, svc))
}
