// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	scoreapp "github.com/opendefender/openrisk/internal/application/score"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

type scoreStubRisks struct {
	byTenant map[uuid.UUID]map[string]int
}

func (s scoreStubRisks) CountRisksByCriticality(_ context.Context, tenantID uuid.UUID) (map[string]int, error) {
	if c, ok := s.byTenant[tenantID]; ok {
		return c, nil
	}
	return map[string]int{}, nil
}

type scoreStubRisk struct{}

func (scoreStubRisk) GetByID(_ context.Context, id, _ uuid.UUID) (*domain.Risk, error) {
	return nil, domain.NewNotFoundError("risk", id)
}

func setupScoreApp(risks scoreStubRisks, tenant uuid.UUID) *fiber.App {
	app := fiber.New()
	h := NewScoreHandler(scoreapp.New().WithRiskCounts(risks).WithRisk(scoreStubRisk{}))
	app.Use(func(c *fiber.Ctx) error {
		if tenant != uuid.Nil {
			middleware.SetContext(c, &middleware.RequestContext{OrganizationID: tenant, UserID: uuid.New()})
		}
		return c.Next()
	})
	app.Get("/api/v1/score", h.GetScore)
	return app
}

func TestScoreHandler_Success(t *testing.T) {
	tenant := uuid.New()
	app := setupScoreApp(scoreStubRisks{byTenant: map[uuid.UUID]map[string]int{
		tenant: {"critical": 3, "high": 2},
	}}, tenant)

	status, body := doGet(t, app, "/api/v1/score?scope=tenant")
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, true, body["measured"])
	require.IsType(t, float64(0), body["value"])
	require.Greater(t, body["value"].(float64), 0.0)
	require.NotNil(t, body["band"])
}

// #287 — on an empty tenant the wire carries null, never a default 0 or 100.
func TestScoreHandler_EmptyTenantReturnsNull(t *testing.T) {
	tenant := uuid.New()
	// Another tenant has critical risks; they must not leak into this score.
	app := setupScoreApp(scoreStubRisks{byTenant: map[uuid.UUID]map[string]int{
		uuid.New(): {"critical": 9},
	}}, tenant)

	status, body := doGet(t, app, "/api/v1/score?scope=tenant")
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, false, body["measured"])
	require.Equal(t, "score.unmeasured.no_data", body["reason_i18n_key"])
	for _, k := range []string{"value", "band", "inherent", "residual"} {
		v, present := body[k]
		require.True(t, present, "%s missing", k)
		require.Nil(t, v, "%s must be null on an empty tenant", k)
	}
}

func TestScoreHandler_NotFound(t *testing.T) {
	app := setupScoreApp(scoreStubRisks{}, uuid.New())
	status, _ := doGet(t, app, "/api/v1/score?scope=risk&id="+uuid.New().String())
	require.Equal(t, fiber.StatusNotFound, status)
}

func TestScoreHandler_Unauthorized(t *testing.T) {
	app := setupScoreApp(scoreStubRisks{}, uuid.Nil)
	status, _ := doGet(t, app, "/api/v1/score?scope=tenant")
	require.Equal(t, fiber.StatusUnauthorized, status)
}
