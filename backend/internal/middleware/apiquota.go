// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Default quotas, per one-minute window.
//
// These are deliberately generous: the goal is to stop floods and noisy-neighbour
// starvation, not to police normal use. A GRC dashboard legitimately fans out a
// dozen calls on load, and the executive dashboard refetches every 60s. Limits
// tight enough to be "strict" would break the product; the strict throttle
// belongs on credential endpoints, which keep their own much lower budget.
const (
	defaultIPRequestsPerMinute     = 300
	defaultTenantRequestsPerMinute = 1200
	quotaWindow                    = time.Minute
)

// QuotaConfig holds the resolved per-IP and per-tenant budgets.
type QuotaConfig struct {
	IPRequestsPerMinute     int
	TenantRequestsPerMinute int
}

// LoadQuotaConfig resolves quotas from the environment.
//
//	RATE_LIMIT_IP_PER_MINUTE      per-IP budget      (0 disables)
//	RATE_LIMIT_TENANT_PER_MINUTE  per-tenant budget  (0 disables)
func LoadQuotaConfig() QuotaConfig {
	cfg := QuotaConfig{
		IPRequestsPerMinute:     defaultIPRequestsPerMinute,
		TenantRequestsPerMinute: defaultTenantRequestsPerMinute,
	}
	if v, ok := positiveIntFromEnv("RATE_LIMIT_IP_PER_MINUTE"); ok {
		cfg.IPRequestsPerMinute = v
	}
	if v, ok := positiveIntFromEnv("RATE_LIMIT_TENANT_PER_MINUTE"); ok {
		cfg.TenantRequestsPerMinute = v
	}
	return cfg
}

func positiveIntFromEnv(key string) (int, bool) {
	raw := os.Getenv(key)
	if raw == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return 0, false
	}
	return parsed, true
}

// IPRateLimit applies a per-IP budget to every request reaching it.
//
// Audit finding F-03: rate limiting covered 2 routes out of 289. Everything else
// — exports, dashboard aggregates, AI endpoints — was unmetered. This is mounted
// broadly so unauthenticated surface is covered too.
//
// The key comes from c.IP(), which only honours a forwarded header for trusted
// proxies (see TrustedProxies / F-04); without that fix this limiter would be
// bypassable by spoofing a header.
func IPRateLimit(store RateLimitBackend, cfg QuotaConfig) fiber.Handler {
	if cfg.IPRequestsPerMinute <= 0 {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return func(c *fiber.Ctx) error {
		if !store.IsAllowed("ip:"+c.IP(), cfg.IPRequestsPerMinute, quotaWindow) {
			return tooManyRequests(c)
		}
		return c.Next()
	}
}

// TenantRateLimit applies a per-tenant budget on top of the per-IP one.
//
// This is the control a multi-tenant SaaS actually needs: without it a single
// tenant — through a runaway integration or a compromised token — can exhaust
// shared database and Redis capacity and degrade every other customer, while
// staying under any per-IP limit by spreading traffic across hosts.
//
// Mount it AFTER the auth middleware, which is what populates the tenant local.
// Requests with no resolvable tenant pass through untouched: they are either
// unauthenticated (already covered by IPRateLimit) or machine identities such as
// scanner agents whose own budget is not tenant-shaped.
func TenantRateLimit(store RateLimitBackend, cfg QuotaConfig) fiber.Handler {
	if cfg.TenantRequestsPerMinute <= 0 {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return func(c *fiber.Ctx) error {
		tenantID, ok := c.Locals("tenant_id").(uuid.UUID)
		if !ok || tenantID == uuid.Nil {
			return c.Next()
		}
		if !store.IsAllowed("tenant:"+tenantID.String(), cfg.TenantRequestsPerMinute, quotaWindow) {
			return tooManyRequests(c)
		}
		return c.Next()
	}
}

func tooManyRequests(c *fiber.Ctx) error {
	c.Set("Retry-After", fmt.Sprintf("%d", int(quotaWindow.Seconds())))
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"error": true,
		"msg":   "Rate limit exceeded",
	})
}
