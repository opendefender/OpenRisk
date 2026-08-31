// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"

	actioncenterapp "github.com/opendefender/openrisk/internal/application/actioncenter"
	"github.com/opendefender/openrisk/internal/middleware"
)

// ActionCenterHandler exposes the contextual Action Center (#429).
type ActionCenterHandler struct {
	useCase *actioncenterapp.UseCase
}

func NewActionCenterHandler(useCase *actioncenterapp.UseCase) *ActionCenterHandler {
	return &ActionCenterHandler{useCase: useCase}
}

// GetActionCenter returns the caller's prioritised outstanding work.
// GET /api/v1/action-center
func (h *ActionCenterHandler) GetActionCenter(c *fiber.Ctx) error {
	caller := callerFromCtx(c)

	limit := c.QueryInt("limit", actioncenterapp.DefaultLimit)
	offset := c.QueryInt("offset", 0)

	// A negative offset is rejected rather than clamped, matching
	// GetNotifications: asking for page -1 is a client bug, and quietly serving
	// page 0 instead hides it.
	if offset < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid offset"})
	}

	result, err := h.useCase.GetActionCenter(caller, limit, offset)
	if err != nil {
		if err == actioncenterapp.ErrUnauthorized {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve action center",
		})
	}

	return c.JSON(result)
}

// callerFromCtx derives the caller's identity from the JWT only. Nothing here
// may come from the query string or the body: the tenant and the user id are
// what every one of the six aggregation queries filters on, so a client-supplied
// value would be a cross-tenant read waiting to happen.
func callerFromCtx(c *fiber.Ctx) actioncenterapp.Caller {
	caller := actioncenterapp.Caller{
		UserID:   safeGetUUID(c, "user_id"),
		TenantID: safeGetUUID(c, "tenant_id"),
	}
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return caller
	}
	if claims.HasPermission("*") {
		caller.IsAdmin = true
	}
	seen := map[string]struct{}{}
	for _, r := range claims.OrgRoles {
		if r == "admin" || r == "root" {
			caller.IsAdmin = true
		}
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		caller.Roles = append(caller.Roles, r)
	}
	return caller
}
