// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

func safeGetUUIDGamification(c *fiber.Ctx, key string) string {
	val := c.Locals(key)
	if val == nil {
		return ""
	}
	if u, ok := val.(string); ok {
		return u
	}
	// Add other checks if necessary, but returning string representation
	return fmt.Sprintf("%v", val)
}

func GetMyGamificationProfile(c *fiber.Ctx) error {
	// Safe type extraction — no more panic on type assertion
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	userID := claims.Sub.String()
	tenantID := safeGetUUIDGamification(c, "tenant_id")

	svc := service.NewGamificationService()
	stats, err := svc.GetUserStats(userID, tenantID)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate stats"})
	}

	return c.JSON(stats)
}
