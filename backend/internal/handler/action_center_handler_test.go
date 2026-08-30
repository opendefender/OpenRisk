// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	actioncenterapp "github.com/opendefender/openrisk/internal/application/actioncenter"
	"github.com/opendefender/openrisk/internal/domain"
)

type acStubRepo struct {
	role  domain.BusinessRoleKey
	risks []domain.Risk
}

func (s *acStubRepo) BusinessRoleFor(_, _ uuid.UUID) (domain.BusinessRoleKey, error) {
	return s.role, nil
}
func (s *acStubRepo) OverdueMitigations(_ uuid.UUID, _ time.Time, _ int) ([]domain.Mitigation, error) {
	return nil, nil
}
func (s *acStubRepo) CriticalRisksWithoutActiveMitigation(_ uuid.UUID, _ float64, _ int) ([]domain.Risk, error) {
	return s.risks, nil
}
func (s *acStubRepo) PendingApprovals(_ uuid.UUID, _ int) ([]domain.ApprovalRequest, error) {
	return nil, nil
}
func (s *acStubRepo) OpenIncidents(_ uuid.UUID, _ int) ([]domain.Incident, error) { return nil, nil }
func (s *acStubRepo) ExpiringEvidence(_ uuid.UUID, _ time.Time, _ int) ([]domain.Evidence, error) {
	return nil, nil
}
func (s *acStubRepo) OverdueRemediationPlans(_ uuid.UUID, _ time.Time, _ int) ([]domain.RemediationPlan, error) {
	return nil, nil
}

// setupActionCenterApp mounts the route behind a stand-in for the auth
// middleware. `authenticated=false` populates nothing, which is exactly what the
// real stack leaves behind when a request arrives without a valid token.
func setupActionCenterApp(repo actioncenterapp.Repository, authenticated bool) *fiber.App {
	app := fiber.New()
	h := NewActionCenterHandler(actioncenterapp.NewUseCase(repo))
	app.Use(func(c *fiber.Ctx) error {
		if authenticated {
			c.Locals("user_id", uuid.New())
			c.Locals("tenant_id", uuid.New())
		}
		return c.Next()
	})
	app.Get("/api/v1/action-center", h.GetActionCenter)
	return app
}

func doGet(t *testing.T, app *fiber.App, url string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", url, nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	return resp.StatusCode, body
}

// Criterion 8.
func TestActionCenterHandler_Unauthorized(t *testing.T) {
	app := setupActionCenterApp(&acStubRepo{}, false)
	status, body := doGet(t, app, "/api/v1/action-center")
	require.Equal(t, fiber.StatusUnauthorized, status)
	require.Equal(t, "unauthorized", body["error"])
}

// Criterion 1 and 7.
func TestActionCenterHandler_EnvelopeShape(t *testing.T) {
	app := setupActionCenterApp(&acStubRepo{role: domain.BusinessRoleRiskManager}, true)
	status, body := doGet(t, app, "/api/v1/action-center")

	require.Equal(t, fiber.StatusOK, status)
	require.Contains(t, body, "data")
	require.Contains(t, body, "generated_at")
	require.Contains(t, body, "limit")
	require.Contains(t, body, "offset")

	// An empty Action Center must serialise as [] — a null here would make the
	// frontend's .map() throw on the happy path of having nothing to do.
	data, ok := body["data"].([]any)
	require.True(t, ok, "data must be a JSON array, got %T", body["data"])
	require.Len(t, data, 0)

	_, err := time.Parse(time.RFC3339, body["generated_at"].(string))
	require.NoError(t, err, "generated_at must be RFC3339")
}

// Criterion 6.
func TestActionCenterHandler_Paging(t *testing.T) {
	repo := &acStubRepo{role: domain.BusinessRoleRiskManager}
	for i := 0; i < 3; i++ {
		repo.risks = append(repo.risks, domain.Risk{
			ID: uuid.New(), TenantID: uuid.New(), Title: "r", Score: 9,
		})
	}
	app := setupActionCenterApp(repo, true)

	t.Run("defaults to 20", func(t *testing.T) {
		status, body := doGet(t, app, "/api/v1/action-center")
		require.Equal(t, fiber.StatusOK, status)
		require.Equal(t, float64(20), body["limit"])
		require.Equal(t, float64(0), body["offset"])
	})

	t.Run("clamps above 100", func(t *testing.T) {
		_, body := doGet(t, app, "/api/v1/action-center?limit=5000")
		require.Equal(t, float64(100), body["limit"])
	})

	t.Run("honours a limit in range", func(t *testing.T) {
		_, body := doGet(t, app, "/api/v1/action-center?limit=2")
		require.Equal(t, float64(2), body["limit"])
		require.Len(t, body["data"].([]any), 2)
	})

	t.Run("rejects a negative offset", func(t *testing.T) {
		status, body := doGet(t, app, "/api/v1/action-center?offset=-1")
		require.Equal(t, fiber.StatusBadRequest, status)
		require.Equal(t, "invalid offset", body["error"])
	})

	t.Run("offset past the end is an empty page, not an error", func(t *testing.T) {
		status, body := doGet(t, app, "/api/v1/action-center?offset=999")
		require.Equal(t, fiber.StatusOK, status)
		require.Len(t, body["data"].([]any), 0)
	})
}

// The response must carry the fields the frontend issue (#430) builds against.
func TestActionCenterHandler_ItemContract(t *testing.T) {
	repo := &acStubRepo{
		role:  domain.BusinessRoleRiskManager,
		risks: []domain.Risk{{ID: uuid.New(), TenantID: uuid.New(), Title: "Critical risk", Score: 9.4}},
	}
	app := setupActionCenterApp(repo, true)
	_, body := doGet(t, app, "/api/v1/action-center")

	items := body["data"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)

	for _, k := range []string{
		"id", "type", "title", "subject_resource_type", "subject_resource_id",
		"deep_link", "due_at", "category_rank", "tenant_id",
	} {
		require.Contains(t, item, k, "ActionItem must expose %q", k)
	}
	require.Equal(t, string(actioncenterapp.ItemTypeCriticalRisk), item["type"])
	require.Equal(t, float64(actioncenterapp.RankCriticalRisk), item["category_rank"])
	require.Contains(t, item["deep_link"], "/risks?drawer=risk&entity=")
}
