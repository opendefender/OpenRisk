// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// #783 criteria 2, 8, 10 and 11: the rows the D-078 re-land added on top of
// #1131. A self-hosted install passes every cap and every kept gate (D-040), a
// count that errors admits the write and says so at warn, and the risk, asset,
// AI and vendor rows hold per hosted plan.

// overBusiness is one past the Business cap of each key, or a large count where
// Business is already unlimited — the case the self-hosted set must admit.
var overBusiness = map[ent.LimitKey]int{
	ent.LimitUsers:             51,
	ent.LimitRisks:             10000,
	ent.LimitAssets:            10000,
	ent.LimitIntegrations:      10000,
	ent.LimitFrameworks:        11,
	ent.LimitAutomationRules:   10000,
	ent.LimitScanSources:       10000,
	ent.LimitScansPerMonth:     10000,
	ent.LimitApprovalWorkflows: 10000,
}

func TestSelfHostedCaps_Success(t *testing.T) {
	require.Len(t, overBusiness, len(ent.AllLimits), "every cap key needs a self-hosted case")
	for _, key := range ent.AllLimits {
		// The stored plan is Free: only the deployment flag lifts the caps.
		svc := entapp.NewService(levelTestOrg{plan: "free"}).
			WithUsage(fixedUsage{key: overBusiness[key]}).
			WithDeployment(ent.DeploymentSelfHosted)
		status, _ := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, key)))
		assert.Equal(t, fiber.StatusOK, status, "self-hosted %s at %d", key, overBusiness[key])
	}
}

func TestSelfHostedKeptGates_Success(t *testing.T) {
	for _, f := range []ent.Feature{ent.FeatCTI, ent.FeatAIAdvisor, ent.FeatVendorRisk} {
		svc := entapp.NewService(levelTestOrg{plan: "free"}).WithDeployment(ent.DeploymentSelfHosted)
		status, _ := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, f)))
		assert.Equal(t, fiber.StatusOK, status, "self-hosted %s", f)
	}
}

// TestSelfHostedCaps_Unauthorized: the same counts on a hosted Free tenant are
// refused, so the self-hosted pass above is the flag and not a dead cap.
func TestSelfHostedCaps_Unauthorized(t *testing.T) {
	for _, key := range ent.AllLimits {
		if key == ent.LimitIntegrations {
			continue // counted, not mounted on a route (ROADMAP 17.2)
		}
		svc := entapp.NewService(levelTestOrg{plan: "free"}).WithUsage(fixedUsage{key: overBusiness[key]})
		status, body := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, key)))
		assert.Equal(t, fiber.StatusPaymentRequired, status, "hosted free %s", key)
		assert.Equal(t, "limit_reached", body["code"])
	}
}

type failingUsage struct{}

func (failingUsage) Count(context.Context, uuid.UUID, ent.LimitKey) (int, error) {
	return 0, errors.New("relation \"scan_jobs\" does not exist")
}

// TestCapCountFailure_NotFound: a count that errors admits the request (a COUNT
// hiccup must not block a legitimate write) and logs it at warn with the tenant
// and the key, so a cap that keeps failing open is visible.
func TestCapCountFailure_NotFound(t *testing.T) {
	var buf bytes.Buffer
	svc := entapp.NewService(levelTestOrg{plan: "free"}).
		WithUsage(failingUsage{}).
		WithLogger(zerolog.New(&buf))
	tenant := uuid.New()

	status, _ := doLevel(t, gateApp(tenant, RequireCapacity(svc, ent.LimitScansPerMonth)))
	assert.Equal(t, fiber.StatusOK, status)

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line), buf.String())
	assert.Equal(t, "warn", line["level"])
	assert.Equal(t, tenant.String(), line["tenant_id"])
	assert.Equal(t, string(ent.LimitScansPerMonth), line["limit_key"])
	assert.Equal(t, "free", line["plan"])
}

// The risk and asset caps predate #783; D-078 lowers Free to 30 of each.
var recordCaps = []struct {
	key  ent.LimitKey
	caps map[string]int
}{
	{ent.LimitRisks, map[string]int{"free": 30, "pro": 500, "business": -1, "enterprise": -1}},
	{ent.LimitAssets, map[string]int{"free": 30, "pro": -1, "business": -1, "enterprise": -1}},
}

func TestRecordCaps_Success(t *testing.T) {
	for _, rc := range recordCaps {
		for plan, limit := range rc.caps {
			used := limit - 1
			if limit < 0 {
				used = 10000
			}
			svc := entapp.NewService(levelTestOrg{plan: plan}).WithUsage(fixedUsage{rc.key: used})
			status, _ := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, rc.key)))
			assert.Equal(t, fiber.StatusOK, status, "%s on %s with %d used", rc.key, plan, used)
		}
	}
}

func TestRecordCaps_Unauthorized(t *testing.T) {
	for _, rc := range recordCaps {
		for plan, limit := range rc.caps {
			if limit < 0 {
				continue
			}
			svc := entapp.NewService(levelTestOrg{plan: plan}).WithUsage(fixedUsage{rc.key: limit})
			status, body := doLevel(t, gateApp(uuid.New(), RequireCapacity(svc, rc.key)))
			assert.Equal(t, fiber.StatusPaymentRequired, status, "%s on %s at %d", rc.key, plan, limit)
			assert.EqualValues(t, limit, body["limit"])
			assert.EqualValues(t, limit, body["used"])
		}
	}
}

// keptGates are the plan gates D-078 keeps until their own quotas ship.
var keptGates = []struct {
	feature ent.Feature
	from    string
	open    []string
	closed  []string
}{
	{ent.FeatAIAdvisor, "pro", []string{"pro", "business", "enterprise"}, []string{"free"}},
	{ent.FeatVendorRisk, "business", []string{"business", "enterprise"}, []string{"free", "pro"}},
	{ent.FeatCTI, "business", []string{"business", "enterprise"}, []string{"free", "pro"}},
}

func TestKeptGates_Success(t *testing.T) {
	for _, g := range keptGates {
		for _, plan := range g.open {
			svc := entapp.NewService(levelTestOrg{plan: plan})
			status, _ := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, g.feature)))
			assert.Equal(t, fiber.StatusOK, status, "%s on %s", g.feature, plan)
		}
	}
}

func TestKeptGates_Unauthorized(t *testing.T) {
	for _, g := range keptGates {
		for _, plan := range g.closed {
			svc := entapp.NewService(levelTestOrg{plan: plan})
			status, body := doLevel(t, gateApp(uuid.New(), RequireFeature(svc, g.feature)))
			assert.Equal(t, fiber.StatusPaymentRequired, status, "%s on %s", g.feature, plan)
			assert.Equal(t, "upgrade_required", body["code"])
			assert.Equal(t, string(g.feature), body["feature"])
			assert.Equal(t, g.from, body["required_plan"])
		}
	}
}
