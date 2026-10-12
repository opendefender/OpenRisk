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
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// PUT /analytics/financial/appetite (#904).

type memAppetite map[uuid.UUID]float64

func (m memAppetite) SetOrganizationRiskAppetite(_ context.Context, id uuid.UUID, v float64) error {
	if _, ok := m[id]; !ok {
		return domain.NewNotFoundError("organization", id)
	}
	m[id] = v
	return nil
}

func appetiteApp(store memAppetite, tenant uuid.UUID) *fiber.App {
	h := NewFinancialAnalyticsHandler(nil).WithAppetiteUseCase(risk.NewSetRiskAppetiteUseCase(store))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if tenant != uuid.Nil {
			middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		}
		return c.Next()
	})
	app.Put("/analytics/financial/appetite", h.SetAppetite)
	return app
}

func putAppetite(t *testing.T, app *fiber.App, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("PUT", "/analytics/financial/appetite", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestSetAppetiteHandler_Success(t *testing.T) {
	tenant := uuid.New()
	store := memAppetite{tenant: 0}
	code, body := putAppetite(t, appetiteApp(store, tenant), `{"appetite_xaf": 80000000}`)
	if code != 200 {
		t.Fatalf("want 200, got %d %s", code, body)
	}
	var out struct {
		AppetiteXAF float64 `json:"appetite_xaf"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	if out.AppetiteXAF != 80_000_000 || store[tenant] != 80_000_000 {
		t.Fatalf("appetite not stored: body %s, store %v", body, store[tenant])
	}
}

func TestSetAppetiteHandler_NotFound(t *testing.T) {
	code, _ := putAppetite(t, appetiteApp(memAppetite{}, uuid.New()), `{"appetite_xaf": 1000}`)
	if code != 404 {
		t.Fatalf("want 404, got %d", code)
	}
}

func TestSetAppetiteHandler_Unauthorized(t *testing.T) {
	store := memAppetite{}
	code, _ := putAppetite(t, appetiteApp(store, uuid.Nil), `{"appetite_xaf": 1000}`)
	if code != 403 {
		t.Fatalf("want 403 without an organization, got %d", code)
	}
}

func TestSetAppetiteHandler_Validation(t *testing.T) {
	tenant := uuid.New()
	app := appetiteApp(memAppetite{tenant: 0}, tenant)
	for _, body := range []string{`{"appetite_xaf": 0}`, `{"appetite_xaf": -3}`, `not json`} {
		if code, _ := putAppetite(t, app, body); code != 400 {
			t.Fatalf("%s: want 400, got %d", body, code)
		}
	}
}
