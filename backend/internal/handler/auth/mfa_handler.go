// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// MFAHandler exposes the /auth/mfa/* endpoints: setup + verify (enrollment, under
// a full session) and challenge (during login, under an MFA_REQUIRED token).
type MFAHandler struct {
	setup     *appauth.SetupMFAUseCase
	verify    *appauth.VerifyMFAUseCase
	disable   *appauth.DisableMFAUseCase
	challenge *appauth.ChallengeMFAUseCase
	tokens    *coreauth.TokenManager
	users     *repository.GormUserRepository
	audit     *coreauth.AuditService
}

// NewMFAHandler wires the MFA use cases + token manager.
func NewMFAHandler(
	setup *appauth.SetupMFAUseCase,
	verify *appauth.VerifyMFAUseCase,
	disable *appauth.DisableMFAUseCase,
	challenge *appauth.ChallengeMFAUseCase,
	tokens *coreauth.TokenManager,
	users *repository.GormUserRepository,
	audit *coreauth.AuditService,
) *MFAHandler {
	return &MFAHandler{setup: setup, verify: verify, disable: disable, challenge: challenge, tokens: tokens, users: users, audit: audit}
}

func ctxUUID(c *fiber.Ctx, key string) uuid.UUID {
	if v, ok := c.Locals(key).(uuid.UUID); ok {
		return v
	}
	if s, ok := c.Locals(key).(string); ok {
		if id, err := uuid.Parse(s); err == nil {
			return id
		}
	}
	return uuid.Nil
}

// Setup begins MFA enrollment: returns the TOTP secret, a QR code and 8 backup
// codes. Requires a full authenticated session.
func (h *MFAHandler) Setup(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	email := ""
	if u, err := h.users.GetByID(c.UserContext(), userID); err == nil && u != nil {
		email = u.Email
	}

	out, err := h.setup.Execute(c.UserContext(), appauth.SetupMFAInput{UserID: userID, TenantID: tenantID, Email: email})
	if err != nil {
		return mapAuthError(c, err)
	}
	if h.audit != nil {
		_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaSetup, true, nil)
	}
	return c.JSON(out)
}

// Verify activates MFA after the user confirms a TOTP code from their app.
func (h *MFAHandler) Verify(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	out, err := h.verify.Execute(c.UserContext(), appauth.VerifyMFAInput{UserID: userID, TenantID: tenantID, Code: req.Code})
	if err != nil {
		return mapAuthError(c, err)
	}
	return c.JSON(out)
}

// Disable turns MFA off for the current user.
func (h *MFAHandler) Disable(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		Password string `json:"password"`
	}
	_ = c.BodyParser(&req)

	out, err := h.disable.Execute(c.UserContext(), appauth.DisableMFAInput{UserID: userID, TenantID: tenantID, Password: req.Password})
	if err != nil {
		return mapAuthError(c, err)
	}
	return c.JSON(out)
}

// Challenge is the second leg of an MFA login. It is reached with an
// MFA_REQUIRED token (validated by MFATokenMiddleware, which populates user_id /
// tenant_id). On a valid TOTP or backup code it mints the real access+refresh
// pair — identical to a password-only login.
func (h *MFAHandler) Challenge(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid MFA session"})
	}

	var req struct {
		Code              string `json:"code"`
		DeviceFingerprint string `json:"device_fingerprint,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if _, err := h.challenge.Execute(c.UserContext(), appauth.ChallengeMFAInput{UserID: userID, TenantID: tenantID, Code: req.Code}); err != nil {
		if h.audit != nil {
			reason := "invalid MFA code"
			_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaVerify, false, &reason)
		}
		return mapAuthError(c, err)
	}

	fp := req.DeviceFingerprint
	if fp == "" {
		fp = c.Get("X-Device-Fingerprint")
	}
	pair, err := h.tokens.IssueSession(c.UserContext(), userID, fp)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue session"})
	}

	if h.audit != nil {
		_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaVerify, true, nil)
	}
	return c.JSON(LoginResponse{TokenPair: pair})
}

// mapAuthError maps typed domain errors to HTTP status codes.
func mapAuthError(c *fiber.Ctx, err error) error {
	if appErr, ok := err.(*domain.AppError); ok {
		switch appErr.Err {
		case domain.ErrValidation:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": appErr.Message})
		case domain.ErrConflict:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": appErr.Message})
		case domain.ErrNotFound:
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": appErr.Message})
		case domain.ErrForbidden:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": appErr.Message})
		}
	}
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
}
