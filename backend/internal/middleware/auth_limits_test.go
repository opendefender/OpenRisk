// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// authLimitedApp mounts the five auth limiters the way main.go does, over ONE
// shared store — the production shape, where every limiter talks to the same
// Redis.
func authLimitedApp() *fiber.App {
	return authLimitedAppWith(AuthLimitConfig{})
}

func authLimitedAppWith(cfg AuthLimitConfig) *fiber.App {
	l := NewAuthLimiters(NewRateLimitStore(), cfg)
	ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app := fiber.New()
	app.Post("/auth/login", l.Login, ok)
	app.Post("/auth/register", l.Register, ok)
	app.Post("/auth/password/forgot", l.Reset, ok)
	app.Post("/auth/password/reset", l.Reset, ok)
	app.Post("/auth/password/check", l.Check, ok)
	app.Post("/auth/password/change", l.Reauth, ok)
	app.Post("/auth/mfa/challenge", l.MFAChallenge, ok)
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

// #588: the production budgets are a contract. The end-to-end switch turns the
// limiters off; it never changes these numbers.
func TestAuthRateLimit_ProductionBudgetsUnchanged(t *testing.T) {
	cases := []struct {
		name   string
		limit  int
		window time.Duration
		want   int
		wantW  time.Duration
	}{
		{"login", AuthLoginLimit, AuthLoginWindow, 15, 5 * time.Minute},
		{"register", AuthRegisterLimit, AuthRegisterWindow, 10, 15 * time.Minute},
		{"reset", AuthResetLimit, AuthResetWindow, 15, 5 * time.Minute},
		{"check", AuthCheckLimit, AuthCheckWindow, 120, 5 * time.Minute},
		{"reauth", AuthReauthLimit, AuthReauthWindow, 15, 5 * time.Minute},
		{"mfa challenge", AuthMFAChallengeLimit, AuthMFAChallengeWindow, 20, 5 * time.Minute},
	}
	for _, c := range cases {
		if c.limit != c.want || c.window != c.wantW {
			t.Errorf("%s: %d per %s, want %d per %s", c.name, c.limit, c.window, c.want, c.wantW)
		}
	}
}

// keyRecorder records the keys a limiter asks for and allows everything.
type keyRecorder struct{ keys []string }

func (k *keyRecorder) IsAllowed(key string, _ int, _ time.Duration) bool {
	k.keys = append(k.keys, key)
	return true
}

// The key prefixes are part of the production contract too: renaming one resets
// the live Redis counters on deploy.
func TestAuthRateLimit_KeyPrefixesUnchanged(t *testing.T) {
	rec := &keyRecorder{}
	l := NewAuthLimiters(rec, AuthLimitConfig{})
	ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app := fiber.New()
	app.Post("/login", l.Login, ok)
	app.Post("/register", l.Register, ok)
	app.Post("/reset", l.Reset, ok)
	app.Post("/check", l.Check, ok)
	app.Post("/reauth", l.Reauth, ok)
	app.Post("/mfa", l.MFAChallenge, ok)

	want := map[string]string{
		"/login":    "auth-login:",
		"/register": "auth-register:",
		"/reset":    "auth-reset:",
		"/check":    "auth-check:",
		"/reauth":   "auth-reauth:",
		"/mfa":      "mfa-challenge-ip:",
	}
	for path, prefix := range want {
		rec.keys = nil
		post(t, app, path)
		if len(rec.keys) != 1 || !strings.HasPrefix(rec.keys[0], prefix) {
			t.Errorf("%s: keys %v, want one key with prefix %q", path, rec.keys, prefix)
		}
	}
}

func TestAuthRateLimit_MFAChallengeHasItsOwnBudget(t *testing.T) {
	app := authLimitedApp()

	for i := 1; i <= AuthMFAChallengeLimit; i++ {
		if got := post(t, app, "/auth/mfa/challenge"); got != fiber.StatusOK {
			t.Fatalf("challenge %d: got %d", i, got)
		}
	}
	if got := post(t, app, "/auth/mfa/challenge"); got != fiber.StatusTooManyRequests {
		t.Fatalf("challenge %d: got %d, want 429", AuthMFAChallengeLimit+1, got)
	}
	if got := post(t, app, "/auth/login"); got != fiber.StatusOK {
		t.Errorf("login after the challenge budget ran out: got %d, want 200", got)
	}
}

func TestAuthRateLimit_DisabledIsPassthrough(t *testing.T) {
	app := authLimitedAppWith(AuthLimitConfig{Disabled: true})

	for _, path := range []string{"/auth/login", "/auth/register", "/auth/password/forgot", "/auth/password/reset", "/auth/password/check", "/auth/password/change", "/auth/mfa/challenge"} {
		for i := 1; i <= AuthCheckLimit+1; i++ {
			if got := post(t, app, path); got != fiber.StatusOK {
				t.Fatalf("%s request %d while disabled: got %d, want 200", path, i, got)
			}
		}
	}
}

func TestLoadAuthLimitConfig(t *testing.T) {
	cases := []struct {
		name     string
		appEnv   string
		disabled string
		want     bool
	}{
		{"default in test", "test", "", false},
		{"switched off in test", "test", "true", true},
		{"switched off in development", "development", "1", true},
		{"explicit false", "test", "false", false},
		{"garbage keeps the limiters", "test", "please", false},
		// The switch must never reach production, including an install that
		// forgot APP_ENV, which IsProductionEnv treats as production.
		{"ignored in production", "production", "true", false},
		{"ignored when APP_ENV is unset", "", "true", false},
		{"ignored for an unknown APP_ENV", "staging", "true", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("APP_ENV", c.appEnv)
			t.Setenv("RATE_LIMIT_AUTH_DISABLED", c.disabled)
			if got := LoadAuthLimitConfig().Disabled; got != c.want {
				t.Errorf("Disabled = %v, want %v", got, c.want)
			}
		})
	}
}
