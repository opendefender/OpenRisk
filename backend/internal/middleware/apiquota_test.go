// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestLoadQuotaConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_IP_PER_MINUTE", "")
		t.Setenv("RATE_LIMIT_TENANT_PER_MINUTE", "")

		cfg := LoadQuotaConfig()
		if cfg.IPRequestsPerMinute != defaultIPRequestsPerMinute {
			t.Errorf("ip: got %d, want %d", cfg.IPRequestsPerMinute, defaultIPRequestsPerMinute)
		}
		if cfg.TenantRequestsPerMinute != defaultTenantRequestsPerMinute {
			t.Errorf("tenant: got %d, want %d", cfg.TenantRequestsPerMinute, defaultTenantRequestsPerMinute)
		}
	})

	t.Run("overrides and disable", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_IP_PER_MINUTE", "42")
		t.Setenv("RATE_LIMIT_TENANT_PER_MINUTE", "0")

		cfg := LoadQuotaConfig()
		if cfg.IPRequestsPerMinute != 42 {
			t.Errorf("ip: got %d, want 42", cfg.IPRequestsPerMinute)
		}
		if cfg.TenantRequestsPerMinute != 0 {
			t.Errorf("tenant: got %d, want 0 (disabled)", cfg.TenantRequestsPerMinute)
		}
	})

	t.Run("garbage falls back to defaults", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_IP_PER_MINUTE", "lots")
		t.Setenv("RATE_LIMIT_TENANT_PER_MINUTE", "-5")

		cfg := LoadQuotaConfig()
		if cfg.IPRequestsPerMinute != defaultIPRequestsPerMinute {
			t.Errorf("ip: got %d, want default", cfg.IPRequestsPerMinute)
		}
		if cfg.TenantRequestsPerMinute != defaultTenantRequestsPerMinute {
			t.Errorf("tenant: got %d, want default", cfg.TenantRequestsPerMinute)
		}
	})
}

func TestIPRateLimit_EnforcesBudget(t *testing.T) {
	app := fiber.New()
	app.Use(IPRateLimit(NewRateLimitStore(), QuotaConfig{IPRequestsPerMinute: 2}))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	var got []int
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/x", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		resp.Body.Close()
		got = append(got, resp.StatusCode)
	}

	if got[0] != 200 || got[1] != 200 {
		t.Errorf("first two requests should pass, got %v", got)
	}
	if got[2] != fiber.StatusTooManyRequests {
		t.Errorf("third request should be throttled, got %d", got[2])
	}
}

func TestIPRateLimit_DisabledIsPassthrough(t *testing.T) {
	app := fiber.New()
	app.Use(IPRateLimit(NewRateLimitStore(), QuotaConfig{IPRequestsPerMinute: 0}))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/x", nil)
		resp, _ := app.Test(req)
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("request %d throttled while quota disabled: %d", i+1, resp.StatusCode)
		}
	}
}

// tenantApp mounts TenantRateLimit behind a stub that injects a tenant local,
// mirroring what the real auth middleware does.
func tenantApp(store RateLimitBackend, cfg QuotaConfig) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if raw := c.Get("X-Test-Tenant"); raw != "" {
			if id, err := uuid.Parse(raw); err == nil {
				c.Locals("tenant_id", id)
			}
		}
		return c.Next()
	})
	app.Use(TenantRateLimit(store, cfg))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func callTenant(t *testing.T, app *fiber.App, tenant string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/x", nil)
	if tenant != "" {
		req.Header.Set("X-Test-Tenant", tenant)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// TestTenantRateLimit_IsolatesNoisyNeighbour is the point of the per-tenant
// quota: one tenant burning its budget must not affect anyone else.
func TestTenantRateLimit_IsolatesNoisyNeighbour(t *testing.T) {
	noisy := uuid.NewString()
	quiet := uuid.NewString()

	app := tenantApp(NewRateLimitStore(), QuotaConfig{TenantRequestsPerMinute: 2})

	if got := callTenant(t, app, noisy); got != 200 {
		t.Fatalf("noisy req1: got %d", got)
	}
	if got := callTenant(t, app, noisy); got != 200 {
		t.Fatalf("noisy req2: got %d", got)
	}
	if got := callTenant(t, app, noisy); got != fiber.StatusTooManyRequests {
		t.Errorf("noisy req3: got %d, want 429", got)
	}

	if got := callTenant(t, app, quiet); got != 200 {
		t.Errorf("quiet tenant got %d — one tenant's burst starved another", got)
	}
}

// TestTenantRateLimit_NoTenantPassesThrough: requests without a resolvable
// tenant (unauthenticated, or machine identities like scanner agents) are the
// per-IP limiter's job, not this one's.
func TestTenantRateLimit_NoTenantPassesThrough(t *testing.T) {
	app := tenantApp(NewRateLimitStore(), QuotaConfig{TenantRequestsPerMinute: 1})

	for i := 0; i < 4; i++ {
		if got := callTenant(t, app, ""); got != fiber.StatusOK {
			t.Fatalf("request %d without tenant got %d, want 200", i+1, got)
		}
	}
}

// TestTenantRateLimit_NilTenantPassesThrough guards the uuid.Nil case, which is
// what an unresolved tenant looks like once auth has run.
func TestTenantRateLimit_NilTenantPassesThrough(t *testing.T) {
	app := tenantApp(NewRateLimitStore(), QuotaConfig{TenantRequestsPerMinute: 1})

	for i := 0; i < 3; i++ {
		if got := callTenant(t, app, uuid.Nil.String()); got != fiber.StatusOK {
			t.Fatalf("request %d with nil tenant got %d, want 200", i+1, got)
		}
	}
}

func TestTooManyRequests_SetsRetryAfter(t *testing.T) {
	app := fiber.New()
	app.Use(IPRateLimit(NewRateLimitStore(), QuotaConfig{IPRequestsPerMinute: 1}))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/x", nil)
		resp, _ := app.Test(req)
		defer resp.Body.Close()
		if i == 1 {
			if resp.StatusCode != fiber.StatusTooManyRequests {
				t.Fatalf("expected throttle, got %d", resp.StatusCode)
			}
			if got := resp.Header.Get("Retry-After"); got != "60" {
				t.Errorf("Retry-After: got %q, want \"60\" so clients can back off", got)
			}
		}
	}
}
