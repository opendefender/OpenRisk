// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// MFAPolicyHandler serves Settings → Security → "Force MFA after N days"
// (OR26-03).
//
// Reading is open to any authenticated member: everyone subject to a deadline
// deserves to see what it is. Writing is admin-only, enforced by the route
// guard — the use case does not know about roles, only about tenants and bounds.
type MFAPolicyHandler struct {
	get    *appauth.GetMFAPolicyUseCase
	update *appauth.UpdateMFAPolicyUseCase
}

// NewMFAPolicyHandler wires the use cases.
func NewMFAPolicyHandler(get *appauth.GetMFAPolicyUseCase, update *appauth.UpdateMFAPolicyUseCase) *MFAPolicyHandler {
	return &MFAPolicyHandler{get: get, update: update}
}

func policyTenant(c *fiber.Ctx) uuid.UUID {
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		return mwCtx.OrganizationID
	}
	return uuid.Nil
}

func policyActor(c *fiber.Ctx) uuid.UUID {
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		return mwCtx.UserID
	}
	return uuid.Nil
}

// Get godoc
// @Summary Read the organization's MFA grace policy
// @Tags Authentication
// @Produce json
// @Security BearerAuth
// @Success 200 {object} auth.MFAPolicyView
// @Router /security/mfa-policy [get]
func (h *MFAPolicyHandler) Get(c *fiber.Ctx) error {
	view, err := h.get.Execute(c.UserContext(), policyTenant(c))
	if err != nil {
		return c.Status(domain.HTTPStatusFromError(err)).JSON(fiber.Map{"error": domain.MessageFromError(err)})
	}
	return c.JSON(view)
}

type updateMFAPolicyRequest struct {
	// A pointer so "field absent" is distinguishable from "set it to zero" —
	// zero is a meaningful value here (enrolment required immediately), and a
	// bare int would silently turn a partial save into the strictest setting.
	GraceDays *int `json:"grace_days"`
}

// Update godoc
// @Summary Set the organization's MFA grace policy
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} auth.MFAPolicyView
// @Failure 400 {object} fiber.Map
// @Router /security/mfa-policy [put]
func (h *MFAPolicyHandler) Update(c *fiber.Ctx) error {
	var req updateMFAPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.GraceDays == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "grace_days is required"})
	}

	view, err := h.update.Execute(c.UserContext(), appauth.UpdateMFAPolicyInput{
		TenantID:  policyTenant(c),
		ActorID:   policyActor(c),
		GraceDays: *req.GraceDays,
	})
	if err != nil {
		return c.Status(domain.HTTPStatusFromError(err)).JSON(fiber.Map{"error": domain.MessageFromError(err)})
	}
	return c.JSON(view)
}
