// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #913 — GET /risks?view=, the item `trend`, and GET /risks/summary, over the
// real handler and repository (sqlite: the Postgres-only trend SQL has its own
// DATABASE_URL-gated test, and degrades to a null trend here).

func (h *tenantRiskApp) createScored(t *testing.T, title string, probability, impact float64) string {
	t.Helper()
	status, body := h.do(t, http.MethodPost, "/api/v1/risks", map[string]any{
		"title": title, "description": "fixture", "impact": impact, "probability": probability,
	})
	require.Equal(t, http.StatusCreated, status, "%v", body)
	id, _ := body["id"].(string)
	require.NotEmpty(t, id)
	return id
}

func itemTitles(body map[string]any) []string {
	items, _ := body["items"].([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		m, _ := it.(map[string]any)
		out = append(out, m["title"].(string))
	}
	return out
}

func TestRiskSummary_Handler_Success(t *testing.T) {
	h := newTenantRiskApp(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	h.as(tenantA)
	h.createScored(t, "A critical", 1, 10) // 10 -> critical
	h.createScored(t, "A low", 0.1, 1)     // 0.1 -> low

	status, body := h.do(t, http.MethodGet, "/api/v1/risks/summary", nil)
	require.Equal(t, http.StatusOK, status, "%v", body)

	views := body["views"].(map[string]any)
	assert.EqualValues(t, 2, views["all"])
	assert.EqualValues(t, 1, views["hot"])
	assert.EqualValues(t, 0, views["accepted"])
	assert.EqualValues(t, 0, views["review_due"])
	for _, k := range []string{"all", "mine", "hot", "review_due", "accepted"} {
		assert.Contains(t, views, k, "every view is present, even at zero")
	}

	bands := body["by_criticality"].(map[string]any)
	assert.EqualValues(t, 1, bands["critical"])
	assert.EqualValues(t, 1, bands["low"])
	assert.EqualValues(t, 0, bands["high"])

	active := body["active"].(map[string]any)
	assert.EqualValues(t, 2, active["count"])
	assert.NotNil(t, active["ale_xaf"], "no plan gate wired: the figure is served")

	// Bands follow the selected view and the search.
	_, body = h.do(t, http.MethodGet, "/api/v1/risks/summary?view=hot", nil)
	bands = body["by_criticality"].(map[string]any)
	assert.EqualValues(t, 1, bands["critical"])
	assert.EqualValues(t, 0, bands["low"], "a low risk is not in the hot view")
	// ...but the view counts ignore it.
	assert.EqualValues(t, 2, body["views"].(map[string]any)["all"])

	// Another tenant starts from zero.
	h.as(tenantB)
	_, body = h.do(t, http.MethodGet, "/api/v1/risks/summary", nil)
	assert.EqualValues(t, 0, body["views"].(map[string]any)["all"])
	assert.EqualValues(t, 0, body["active"].(map[string]any)["count"])
}

func TestRiskSummary_Handler_NotFound(t *testing.T) {
	// An empty register is a summary of zeros, and the route is not shadowed by
	// /risks/:id (which would answer 400 "Invalid UUID").
	h := newTenantRiskApp(t)
	h.as(uuid.New())
	status, body := h.do(t, http.MethodGet, "/api/v1/risks/summary", nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.EqualValues(t, 0, body["active"].(map[string]any)["count"])

	status, body = h.do(t, http.MethodGet, "/api/v1/risks/summary?view=bogus", nil)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "validation_failed", body["error"])
}

func TestRiskSummary_Handler_Unauthorized(t *testing.T) {
	h := newTenantRiskApp(t)
	h.as(uuid.New())
	h.createScored(t, "A critical", 1, 10)

	h.as(uuid.Nil)
	status, body := h.do(t, http.MethodGet, "/api/v1/risks/summary", nil)
	assert.Equal(t, http.StatusUnauthorized, status, "a nil tenant is a 401, never all tenants: %v", body)
	assert.NotContains(t, body, "views", "nothing leaks in the refusal")
}

func TestRiskSummary_Handler_CrossTenant(t *testing.T) {
	h := newTenantRiskApp(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	h.as(tenantB)
	for i := 0; i < 3; i++ {
		h.createScored(t, "B secret", 1, 10)
	}

	h.as(tenantA)
	h.createScored(t, "A only", 0.1, 1)
	for _, path := range []string{
		"/api/v1/risks/summary",
		"/api/v1/risks/summary?view=hot",
	} {
		status, body := h.do(t, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, status, path)
		assert.EqualValues(t, 1, body["views"].(map[string]any)["all"], "%s must count only tenant A", path)
		assert.EqualValues(t, 0, body["by_criticality"].(map[string]any)["critical"], "%s must not see B's critical risks", path)
		assert.EqualValues(t, 1, body["active"].(map[string]any)["count"], path)
	}
}

func TestRiskList_View(t *testing.T) {
	h := newTenantRiskApp(t)
	h.as(uuid.New())
	h.createScored(t, "crit", 1, 10)
	h.createScored(t, "low", 0.1, 1)

	status, body := h.do(t, http.MethodGet, "/api/v1/risks?view=hot", nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, []string{"crit"}, itemTitles(body))
	assert.EqualValues(t, 1, body["total"])

	_, body = h.do(t, http.MethodGet, "/api/v1/risks?view=all", nil)
	assert.Len(t, itemTitles(body), 2)
	_, body = h.do(t, http.MethodGet, "/api/v1/risks", nil)
	assert.Len(t, itemTitles(body), 2, "no view means all")

	// Every item carries the trend key; null means no snapshot read.
	items := body["items"].([]any)
	for _, it := range items {
		m := it.(map[string]any)
		v, present := m["trend"]
		assert.True(t, present, "trend is always present on a list item")
		assert.Nil(t, v)
	}

	// Unknown view: 400 validation_failed, not a silent "all".
	status, body = h.do(t, http.MethodGet, "/api/v1/risks?view=nope", nil)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "validation_failed", body["error"])

	// accepted / review_due are empty here, and filtering does not error.
	for _, v := range []string{"accepted", "review_due"} {
		status, body = h.do(t, http.MethodGet, "/api/v1/risks?view="+v, nil)
		require.Equal(t, http.StatusOK, status, v)
		assert.Empty(t, itemTitles(body), v)
	}

	// Sorting on next_review_at is accepted (NULLs last).
	status, _ = h.do(t, http.MethodGet, "/api/v1/risks?sort_by=next_review_at&sort_dir=asc", nil)
	assert.Equal(t, http.StatusOK, status)
}

func TestRiskList_ViewMine(t *testing.T) {
	h := newTenantRiskApp(t)
	tenant := uuid.New()
	me := uuid.New()
	h.as(tenant)
	// Creating assigns the creator, so each risk is made by its own user.
	*h.user = me
	h.createScored(t, "mine", 1, 10)
	*h.user = uuid.New()
	h.createScored(t, "someone else's", 1, 10)
	*h.user = me

	status, body := h.do(t, http.MethodGet, "/api/v1/risks?view=mine", nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, []string{"mine"}, itemTitles(body))

	_, body = h.do(t, http.MethodGet, "/api/v1/risks?mine=true", nil)
	assert.Equal(t, []string{"mine"}, itemTitles(body), "view=mine is identical to mine=true")

	_, body = h.do(t, http.MethodGet, "/api/v1/risks/summary", nil)
	assert.EqualValues(t, 1, body["views"].(map[string]any)["mine"])
	assert.EqualValues(t, 2, body["views"].(map[string]any)["all"])

	// Someone else's session sees nothing under "mine".
	*h.user = uuid.New()
	_, body = h.do(t, http.MethodGet, "/api/v1/risks?view=mine", nil)
	assert.Empty(t, itemTitles(body))
}

// "summary" must be mounted before /risks/:id in the composition root, or it
// parses as a uuid and 400s; and it carries the same read permission as the list.
func TestRiskSummaryRoute_MountedBeforeByIDWithReadPermission(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	src := string(raw)

	summary := strings.Index(src, `protected.Get("/risks/summary",`)
	byID := strings.Index(src, `protected.Get("/risks/:id",`)
	require.NotEqual(t, -1, summary, "GET /risks/summary must be mounted")
	require.NotEqual(t, -1, byID)
	assert.Less(t, summary, byID, "/risks/summary must be registered before /risks/:id")

	rest := src[summary:]
	end := strings.Index(rest, "riskHandler.GetRiskSummary)")
	require.NotEqual(t, -1, end)
	assert.Contains(t, rest[:end], `middleware.RequirePermission("risks:read")`)
}
