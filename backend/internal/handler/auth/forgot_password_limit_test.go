// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// saturatedResets reports every address as already at its hourly cap, so the
// request use case refuses before touching anything else.
type saturatedResets struct {
	appauth.PasswordResetRepository
}

func (saturatedResets) CountRecentByEmailHash(context.Context, string, time.Time) (int64, error) {
	return domain.PasswordResetMaxPerHour, nil
}

type noResetUsers struct{}

func (noResetUsers) GetByEmail(context.Context, string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (noResetUsers) GetByID(context.Context, uuid.UUID) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (noResetUsers) Update(context.Context, *domain.User) error { return nil }

// #1318: the per-address reset cap answered 429 with the wait in the body only;
// an HTTP client reads the header.
func TestForgotPassword_RateLimitedAnswers429WithRetryAfter(t *testing.T) {
	h := NewPasswordHandler(appauth.NewRequestPasswordResetUseCase(noResetUsers{}, saturatedResets{}, nil),
		nil, nil, "https://openrisk.test", nil)
	app := fiber.New()
	app.Post("/auth/password/forgot", h.ForgotPassword)

	req := httptest.NewRequest(http.MethodPost, "/auth/password/forgot", strings.NewReader(`{"email":"a@b.io"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", resp.StatusCode)
	}
	want := strconv.Itoa(int(domain.PasswordResetWindow.Seconds()))
	if got := resp.Header.Get("Retry-After"); got != want {
		t.Fatalf("Retry-After %q, want %q", got, want)
	}
}
