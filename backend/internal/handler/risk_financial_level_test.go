// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/crq"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type planOnlyOrg struct{ plan string }

func (o planOnlyOrg) PlanAndRegion(context.Context, uuid.UUID) (string, string, error) {
	return o.plan, "eu", nil
}

// freeWithBasicFinancial is the matrix #415 option B proposes — Free granted
// financial quantification at basic — which no shipped plan has yet.
func freeWithBasicFinancial(p ent.Plan, f ent.Feature) ent.Level {
	if p == ent.PlanFree && f == ent.FeatFinancialQuant {
		return ent.LevelBasic
	}
	return ent.LevelOf(p, f)
}

// getFinancial creates a risk with explicit drivers and reads its assessment
// through the same gate main.go mounts on GET /risks/:id/financial.
func getFinancial(t *testing.T, svc *entapp.Service) (int, crq.FinancialAssessment) {
	t.Helper()
	app := setupAppWithDB(t, middleware.RequireLevel(svc, ent.FeatFinancialQuant, ent.LevelBasic))

	b, _ := json.Marshal(map[string]interface{}{
		"title": "Ransomware", "description": "d", "impact": 8, "probability": 0.6,
		"sle_xaf": 20_000_000, "aro": 0.5,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode)
	var created domain.Risk
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	fin, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/risks/"+created.ID.String()+"/financial", nil))
	require.NoError(t, err)
	defer fin.Body.Close()
	var a crq.FinancialAssessment
	if fin.StatusCode == fiber.StatusOK {
		require.NoError(t, json.NewDecoder(fin.Body).Decode(&a))
	}
	return fin.StatusCode, a
}

func TestGetRiskFinancial_BasicServesDeterministicALE(t *testing.T) {
	svc := entapp.NewService(planOnlyOrg{plan: "free"}).WithLevelSource(freeWithBasicFinancial)

	status, a := getFinancial(t, svc)
	require.Equal(t, fiber.StatusOK, status)
	assert.Greater(t, a.ALE.XAF, 0.0, "a basic grant still gets a figure")
	assert.Nil(t, a.Distribution, "the Monte-Carlo distribution is the on depth")
	assert.Nil(t, a.Methodology, "no simulation ran, so no simulation methodology")
}

func TestGetRiskFinancial_OnServesTheSimulation(t *testing.T) {
	svc := entapp.NewService(planOnlyOrg{plan: "pro"})

	status, a := getFinancial(t, svc)
	require.Equal(t, fiber.StatusOK, status)
	require.NotNil(t, a.Distribution)
	require.NotNil(t, a.Methodology)
	assert.Greater(t, a.Methodology.Iterations, 0)
}

func TestGetRiskFinancial_ShippedFreeIsRefused(t *testing.T) {
	// The shipped matrix keeps financial quantification off on Free: the basic
	// gate refuses it exactly as RequireFeature did before #613.
	status, _ := getFinancial(t, entapp.NewService(planOnlyOrg{plan: "free"}))
	assert.Equal(t, fiber.StatusPaymentRequired, status)
}
