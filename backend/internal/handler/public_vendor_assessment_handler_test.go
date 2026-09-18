// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/tprm"
	"github.com/opendefender/openrisk/internal/domain"
)

// deadAssessmentStore resolves no token: every request is a dead link.
type deadAssessmentStore struct{ lookups []string }

func (s *deadAssessmentStore) CreateAssessment(context.Context, *domain.VendorAssessment, *domain.VendorAssessmentToken) error {
	return nil
}
func (s *deadAssessmentStore) GetAssessment(context.Context, uuid.UUID, uuid.UUID) (*domain.VendorAssessment, error) {
	return nil, nil
}
func (s *deadAssessmentStore) ListAssessmentsByVendor(context.Context, uuid.UUID, uuid.UUID) ([]domain.VendorAssessment, error) {
	return nil, nil
}
func (s *deadAssessmentStore) RevokeAssessment(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, nil
}
func (s *deadAssessmentStore) IssueToken(context.Context, *domain.VendorAssessmentToken) (bool, error) {
	return false, nil
}
func (s *deadAssessmentStore) FindTokenByHash(_ context.Context, hash string) (*domain.VendorAssessmentToken, error) {
	s.lookups = append(s.lookups, hash)
	return nil, nil
}
func (s *deadAssessmentStore) SaveAnswers(context.Context, uuid.UUID, uuid.UUID, []domain.VendorAssessmentItem, time.Time) (bool, error) {
	return false, nil
}
func (s *deadAssessmentStore) SubmitAssessment(context.Context, uuid.UUID, uuid.UUID, []domain.VendorAssessmentItem, domain.JSONMap, domain.VendorAssessmentScoring, time.Time) (bool, error) {
	return false, nil
}

type denyAfter struct {
	limit int
	seen  map[string]int
}

func (d *denyAfter) IsAllowed(key string, _ int, _ time.Duration) bool {
	d.seen[key]++
	return d.seen[key] <= d.limit
}

func publicApp(store tprm.AssessmentStore, limiter *denyAfter) *fiber.App {
	h := NewPublicVendorAssessmentHandler(tprm.AssessmentDeps{Assessments: store}, limiter)
	app := fiber.New()
	app.Get("/api/v1/public/vendor-assessment", h.Get)
	app.Put("/api/v1/public/vendor-assessment/answers", h.SaveAnswers)
	app.Post("/api/v1/public/vendor-assessment/submit", h.Submit)
	return app
}

func TestPublicVendorAssessmentHandler_DeadLinkIs404WithNoStoreAndNoReferrer(t *testing.T) {
	store := &deadAssessmentStore{}
	app := publicApp(store, &denyAfter{limit: 1000, seen: map[string]int{}})
	token := "tok_that_names_nothing_ABCDEFGHIJKLMNOPQRSTUV"

	req := httptest.NewRequest("GET", "/api/v1/public/vendor-assessment", nil)
	req.Header.Set(VendorAssessmentTokenHeader, token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"))
	assert.NotContains(t, string(body), token, "the token is never echoed")
	require.Len(t, store.lookups, 1)
	assert.Equal(t, domain.HashVendorAssessmentToken(token), store.lookups[0], "only the hash reaches the store")
}

func TestPublicVendorAssessmentHandler_TheTokenInTheQueryStringIsIgnored(t *testing.T) {
	store := &deadAssessmentStore{}
	app := publicApp(store, &denyAfter{limit: 1000, seen: map[string]int{}})

	req := httptest.NewRequest("GET", "/api/v1/public/vendor-assessment?token=some-token", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 404, resp.StatusCode)
	assert.Empty(t, store.lookups, "no header, no lookup: a token in a URL is not a credential here")
}

func TestPublicVendorAssessmentHandler_PerTokenAndPerSubmitLimitsAnswer429(t *testing.T) {
	limiter := &denyAfter{limit: 1, seen: map[string]int{}}
	store := &deadAssessmentStore{}
	app := publicApp(store, limiter)

	send := func(method, path string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(VendorAssessmentTokenHeader, "same-token")
		resp, err := app.Test(req)
		require.NoError(t, err)
		return resp.StatusCode
	}

	assert.Equal(t, 404, send("GET", "/api/v1/public/vendor-assessment"))
	assert.Equal(t, 429, send("GET", "/api/v1/public/vendor-assessment"), "the second request with the same token is over the limit")

	for k := range limiter.seen {
		assert.False(t, strings.Contains(k, "same-token"), "the counter store never holds the token: %s", k)
	}
}

func TestPrefixedRateLimitBackend_KeepsCountersApart(t *testing.T) {
	shared := &denyAfter{limit: 1, seen: map[string]int{}}
	login := PrefixedRateLimitBackend{Prefix: "login:", Inner: shared}
	questionnaire := PrefixedRateLimitBackend{Prefix: "questionnaire:", Inner: shared}

	assert.True(t, login.IsAllowed("203.0.113.7", 1, time.Minute))
	assert.True(t, questionnaire.IsAllowed("203.0.113.7", 1, time.Minute), "the same IP on another limiter has its own budget")
	assert.False(t, login.IsAllowed("203.0.113.7", 1, time.Minute))
}
