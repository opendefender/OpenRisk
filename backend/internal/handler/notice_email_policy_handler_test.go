// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

type policyStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]domain.NoticeEmailPolicy
}

func (s *policyStore) GetNoticeEmailPolicy(_ context.Context, tenantID uuid.UUID) (*domain.NoticeEmailPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.rows[tenantID]; ok {
		return &p, nil
	}
	return nil, nil
}

func (s *policyStore) SaveNoticeEmailPolicy(_ context.Context, p *domain.NoticeEmailPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[p.TenantID] = *p
	return nil
}

// noticePolicyApp mounts the two routes as main.go does, behind a fake auth
// step that sets the caller's tenant and role.
func noticePolicyApp(store *policyStore, tenantID uuid.UUID, role string) *fiber.App {
	h := NewNoticeEmailPolicyHandler(
		notificationapp.NewGetNoticeEmailPolicyUseCase(store),
		notificationapp.NewUpdateNoticeEmailPolicyUseCase(store))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if tenantID != uuid.Nil {
			middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: tenantID})
			c.Locals("org_roles", map[uuid.UUID]string{tenantID: role})
		}
		return c.Next()
	})
	app.Get("/notifications/email-policy", h.Get)
	app.Put("/notifications/email-policy", middleware.RequireRole("admin", "root"), h.Update)
	return app
}

func callNoticePolicy(t *testing.T, app *fiber.App, method, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, "/notifications/email-policy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(res.Body)
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out
}

func TestNoticeEmailPolicyHandler_Success(t *testing.T) {
	store := &policyStore{rows: map[uuid.UUID]domain.NoticeEmailPolicy{}}
	tenant := uuid.New()

	code, body := callNoticePolicy(t, noticePolicyApp(store, tenant, "admin"), fiber.MethodPut, `{"window_minutes":240}`)
	require.Equal(t, 200, code)
	require.EqualValues(t, 240, body["window_minutes"])
	require.Equal(t, true, body["configured"])

	// Any member reads it.
	code, body = callNoticePolicy(t, noticePolicyApp(store, tenant, "user"), fiber.MethodGet, "")
	require.Equal(t, 200, code)
	require.EqualValues(t, 240, body["window_minutes"])
	require.Len(t, body["options"], 5)
}

// Never saved: the default, flagged as such.
func TestNoticeEmailPolicyHandler_NotFound(t *testing.T) {
	store := &policyStore{rows: map[uuid.UUID]domain.NoticeEmailPolicy{}}
	code, body := callNoticePolicy(t, noticePolicyApp(store, uuid.New(), "user"), fiber.MethodGet, "")
	require.Equal(t, 200, code)
	require.EqualValues(t, 60, body["window_minutes"])
	require.Equal(t, false, body["configured"])
}

func TestNoticeEmailPolicyHandler_Unauthorized(t *testing.T) {
	store := &policyStore{rows: map[uuid.UUID]domain.NoticeEmailPolicy{}}
	tenant := uuid.New()

	code, _ := callNoticePolicy(t, noticePolicyApp(store, tenant, "user"), fiber.MethodPut, `{"window_minutes":15}`)
	require.Equal(t, 403, code, "a member cannot change it")
	code, _ = callNoticePolicy(t, noticePolicyApp(store, tenant, "analyst"), fiber.MethodPut, `{"window_minutes":15}`)
	require.Equal(t, 403, code)
	require.Empty(t, store.rows)

	code, _ = callNoticePolicy(t, noticePolicyApp(store, uuid.Nil, ""), fiber.MethodGet, "")
	require.Equal(t, 401, code, "no organization in context")
}

func TestNoticeEmailPolicyHandler_RejectsBadInput(t *testing.T) {
	store := &policyStore{rows: map[uuid.UUID]domain.NoticeEmailPolicy{}}
	app := noticePolicyApp(store, uuid.New(), "root")
	for _, body := range []string{`{"window_minutes":45}`, `{"window_minutes":0}`, `{}`, `not json`} {
		code, _ := callNoticePolicy(t, app, fiber.MethodPut, body)
		require.Equal(t, 400, code, body)
	}
	require.Empty(t, store.rows)
}
