// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

	userID := claims.ID.String()
	tenantID := safeGetUUIDGamification(c, "tenant_id")

	svc := service.NewGamificationService()
	stats, err := svc.GetUserStats(userID, tenantID)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate stats"})
	}

	return c.JSON(stats)
}
