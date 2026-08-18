// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// This is where the open-core model becomes REAL: the backend itself refuses a
// request the plan does not permit. The frontend only greys and explains — it
// never grants — so a hand-crafted API call cannot bypass the wall.
//
// Denials use HTTP 402 Payment Required with a structured, explaining body (never
// a bare 403), so the client can render a converting upgrade prompt.

// entTenant reads the tenant id from the request context.
func entTenant(c *fiber.Ctx) uuid.UUID {
	if rc := GetContext(c); rc != nil && rc.OrganizationID != uuid.Nil {
		return rc.OrganizationID
	}
	if t, ok := c.Locals("tenantID").(uuid.UUID); ok {
		return t
	}
	return uuid.Nil
}

// RequireFeature refuses the request (402) unless the tenant's effective plan
// grants the feature. On a resolver error it fails OPEN (does not block on an
// infrastructure hiccup) — the same posture as the rest of the codebase.
func RequireFeature(svc *entapp.Service, feature ent.Feature) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if svc == nil {
			return c.Next()
		}
		tenant := entTenant(c)
		if tenant == uuid.Nil {
			return c.Next()
		}
		ok, current, required, err := svc.Allowed(c.UserContext(), tenant, feature)
		if err != nil || ok {
			return c.Next()
		}
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"code":          "upgrade_required",
			"feature":       string(feature),
			"current_plan":  string(current),
			"required_plan": string(required),
			"message":       ent.UpgradeMessage(feature),
			"upgrade_url":   "/settings?tab=billing",
		})
	}
}

// RequireCapacity refuses a create (402) when the tenant is at a plan limit for a
// countable resource. Fails OPEN on a counting error (see application.Capacity).
func RequireCapacity(svc *entapp.Service, key ent.LimitKey) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if svc == nil {
			return c.Next()
		}
		tenant := entTenant(c)
		if tenant == uuid.Nil {
			return c.Next()
		}
		allowed, limit, used, plan, err := svc.Capacity(c.UserContext(), tenant, key)
		if err != nil || allowed {
			return c.Next()
		}
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"code":         "limit_reached",
			"limit_key":    string(key),
			"limit":        limit,
			"used":         used,
			"current_plan": string(plan),
			"message":      ent.LimitMessage(key, limit),
			"upgrade_url":  "/settings?tab=billing",
		})
	}
}
