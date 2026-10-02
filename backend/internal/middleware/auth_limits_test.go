// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// authLimitedApp mounts the five auth limiters the way main.go does, over ONE
// shared store — the production shape, where every limiter talks to the same
// Redis.
func authLimitedApp() *fiber.App {
	l := NewAuthLimiters(NewRateLimitStore())
	ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app := fiber.New()
	app.Post("/auth/login", l.Login, ok)
	app.Post("/auth/register", l.Register, ok)
	app.Post("/auth/password/forgot", l.Reset, ok)
	app.Post("/auth/password/reset", l.Reset, ok)
	app.Post("/auth/password/check", l.Check, ok)
	app.Post("/auth/password/change", l.Reauth, ok)
	return app
}

func post(t *testing.T, app *fiber.App, path string) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, path, nil))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return resp.StatusCode
}

// #688 reproduction 1: the strength meter, called while a person types, used to
// spend the budget login and sign-up share, and lock them out of both.
func TestAuthRateLimit_PasswordCheckDoesNotConsumeLoginBudget(t *testing.T) {
	app := authLimitedApp()

	for i := 1; i <= AuthCheckLimit; i++ {
		if got := post(t, app, "/auth/password/check"); got != fiber.StatusOK {
			t.Fatalf("check %d: got %d", i, got)
		}
	}
	if got := post(t, app, "/auth/password/check"); got != fiber.StatusTooManyRequests {
		t.Fatalf("check %d: got %d, want 429", AuthCheckLimit+1, got)
	}

	for _, path := range []string{"/auth/login", "/auth/register", "/auth/password/forgot", "/auth/password/change"} {
		if got := post(t, app, path); got != fiber.StatusOK {
			t.Errorf("%s after the check budget ran out: got %d, want 200", path, got)
		}
	}
}

func TestAuthRateLimit_EachPurposeHasItsOwnBudget(t *testing.T) {
	app := authLimitedApp()

	for i := 0; i < AuthLoginLimit; i++ {
		post(t, app, "/auth/login")
	}
	if got := post(t, app, "/auth/login"); got != fiber.StatusTooManyRequests {
		t.Fatalf("login past its budget: got %d, want 429", got)
	}
	// Login exhausted; nothing else is.
	for _, path := range []string{"/auth/register", "/auth/password/forgot", "/auth/password/check", "/auth/password/change"} {
		if got := post(t, app, path); got != fiber.StatusOK {
			t.Errorf("%s after login ran out: got %d, want 200", path, got)
		}
	}

	// Forgot and reset are one purpose and share one budget.
	for i := 0; i < AuthResetLimit; i++ {
		post(t, app, "/auth/password/forgot")
	}
	if got := post(t, app, "/auth/password/reset"); got != fiber.StatusTooManyRequests {
		t.Errorf("reset after forgot spent the reset budget: got %d, want 429", got)
	}
}

func TestPrefixedBackend_KeepsCountersApart(t *testing.T) {
	shared := NewRateLimitStore()
	a := PrefixedBackend{Prefix: "a:", Inner: shared}
	b := PrefixedBackend{Prefix: "b:", Inner: shared}
	if !a.IsAllowed("1.2.3.4", 1, 60e9) || a.IsAllowed("1.2.3.4", 1, 60e9) {
		t.Fatal("a: want one allowed then refused")
	}
	if !b.IsAllowed("1.2.3.4", 1, 60e9) {
		t.Fatal("b must not see a's count")
	}
}
