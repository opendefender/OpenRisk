// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// SwitchHandler serves the organization switcher: listing the orgs a user may
// operate in, and issuing a session scoped to a chosen one.
//
// The switch is always server-authorized. The caller proves who they are with an
// existing session (this group is mounted under the protected middleware), and
// the server independently re-checks active membership of the target org before
// minting anything. Changing a tenant id in a token, cookie, or body cannot move
// a user into an org they are not an active member of.
type SwitchHandler struct {
	list    *appauth.ListOrganizationsUseCase
	switch_ *appauth.SwitchOrganizationUseCase
	audit   *coreauth.AuditService
}

// NewSwitchHandler builds the handler.
func NewSwitchHandler(
	list *appauth.ListOrganizationsUseCase,
	switchUC *appauth.SwitchOrganizationUseCase,
	audit *coreauth.AuditService,
) *SwitchHandler {
	return &SwitchHandler{list: list, switch_: switchUC, audit: audit}
}

// ListOrganizations returns the organizations the authenticated user may switch
// into (their active memberships).
func (h *SwitchHandler) ListOrganizations(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil || mwCtx.UserID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	orgs, err := h.list.Execute(c.UserContext(), mwCtx.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list organizations"})
	}
	return c.JSON(fiber.Map{"organizations": orgs, "total": len(orgs)})
}

// SwitchOrganizationRequest is the switch body.
type SwitchOrganizationRequest struct {
	OrganizationID string `json:"organization_id" validate:"required,uuid"`
}

// SwitchOrganization mints a session scoped to the target organization.
func (h *SwitchHandler) SwitchOrganization(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil || mwCtx.UserID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	var req SwitchOrganizationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	targetOrg, err := uuid.Parse(req.OrganizationID)
	if err != nil || targetOrg == uuid.Nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid organization id"})
	}

	result, err := h.switch_.Execute(c.UserContext(), appauth.SwitchOrganizationInput{
		UserID:            mwCtx.UserID,
		TargetOrgID:       targetOrg,
		DeviceFingerprint: c.Get("X-Device-Fingerprint"),
		IP:                c.IP(),
		UserAgent:         c.Get("User-Agent"),
	})
	if err != nil {
		h.audit_(c, &mwCtx.UserID, &targetOrg, false, strptr("switch_denied"))
		if appErr, ok := err.(*domain.AppError); ok {
			switch appErr.Err {
			case domain.ErrForbidden:
				// A non-member and a deactivated member get the same 403 so the
				// endpoint cannot be used to enumerate orgs.
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": appErr.Message})
			case domain.ErrValidation:
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": appErr.Message})
			}
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to switch organization"})
	}

	// Re-issue the HttpOnly session cookies for the new org so a browser client
	// carries the new tenant context without ever touching the raw tokens.
	csrfToken, cErr := middleware.IssueSessionCookies(
		c,
		result.TokenPair.AccessToken,
		result.TokenPair.RefreshToken,
		coreauth.AccessTokenTTL,
		coreauth.RefreshTokenTTL,
	)
	if cErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not establish session"})
	}

	h.audit_(c, &mwCtx.UserID, &targetOrg, true, nil)

	resp := fiber.Map{
		"token_pair":    result.TokenPair,
		"organization":  result.Organization,
		"role":          result.Role,
		"business_role": result.BusinessRole,
		"csrf_token":    csrfToken,
	}
	return c.JSON(resp)
}

// audit_ records a switch_org event (tenant = the TARGET org) when audit is wired.
func (h *SwitchHandler) audit_(c *fiber.Ctx, userID, targetOrg *uuid.UUID, success bool, reason *string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.LogFiber(c, userID, targetOrg, coreauth.AuditActionSwitchOrg, success, reason)
}
