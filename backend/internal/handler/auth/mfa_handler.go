// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// MFAHandler handles MFA endpoints
type MFAHandler struct {
	setupUseCase    *auth.SetupMFAUseCase
	verifyUseCase   *auth.VerifyMFAUseCase
	challengeUseCase *auth.ChallengeMFAUseCase
	disableUseCase  *auth.DisableMFAUseCase
}

// NewMFAHandler creates a new MFA handler
func NewMFAHandler(
	setupUseCase *auth.SetupMFAUseCase,
	verifyUseCase *auth.VerifyMFAUseCase,
	challengeUseCase *auth.ChallengeMFAUseCase,
	disableUseCase *auth.DisableMFAUseCase,
) *MFAHandler {
	return &MFAHandler{
		setupUseCase:    setupUseCase,
		verifyUseCase:   verifyUseCase,
		challengeUseCase: challengeUseCase,
		disableUseCase:  disableUseCase,
	}
}

// SetupMFARequest represents MFA setup request
type SetupMFARequest struct {
	Email string `json:"email" validate:"required,email"`
}

// SetupMFAResponse represents MFA setup response
type SetupMFAResponse struct {
	Secret      string   `json:"secret"`
	QRCode      string   `json:"qr_code"`
	BackupCodes []string `json:"backup_codes"`
}

// HandleSetupMFA handles POST /auth/mfa/setup
func (h *MFAHandler) HandleSetupMFA(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id")
	tenantIDStr := c.Locals("tenant_id")

	if userIDStr == nil || tenantIDStr == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user_id",
		})
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid tenant_id",
		})
	}

	var req SetupMFARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email required",
		})
	}

	input := auth.SetupMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Email:    req.Email,
	}

	output, err := h.setupUseCase.Execute(c.Context(), input)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(SetupMFAResponse{
		Secret:      output.Secret,
		QRCode:      output.QRCode,
		BackupCodes: output.BackupCodes,
	})
}

// VerifyMFARequest represents MFA verification request
type VerifyMFARequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// VerifyMFAResponse represents MFA verification response
type VerifyMFAResponse struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

// HandleVerifyMFA handles POST /auth/mfa/verify
func (h *MFAHandler) HandleVerifyMFA(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id")
	tenantIDStr := c.Locals("tenant_id")

	if userIDStr == nil || tenantIDStr == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user_id",
		})
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid tenant_id",
		})
	}

	var req VerifyMFARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	input := auth.VerifyMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     req.Code,
	}

	output, err := h.verifyUseCase.Execute(c.Context(), input)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(VerifyMFAResponse{
		Verified: output.Verified,
		Message:  output.Message,
	})
}

// ChallengeMFARequest represents MFA challenge request
type ChallengeMFARequest struct {
	Code string `json:"code" validate:"required"`
}

// ChallengeMFAResponse represents MFA challenge response
type ChallengeMFAResponse struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

// HandleChallengeMFA handles POST /auth/mfa/challenge
// Used after login with MFA_REQUIRED token
func (h *MFAHandler) HandleChallengeMFA(c *fiber.Ctx) error {
	// This would be called with a temporary MFA token
	// For now, require user_id in context
	userIDStr := c.Locals("user_id")
	tenantIDStr := c.Locals("tenant_id")

	if userIDStr == nil || tenantIDStr == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user_id",
		})
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid tenant_id",
		})
	}

	var req ChallengeMFARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	input := auth.ChallengeMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     req.Code,
	}

	output, err := h.challengeUseCase.Execute(c.Context(), input)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(ChallengeMFAResponse{
		Verified: output.Verified,
		Message:  output.Message,
	})
}

// DisableMFARequest represents MFA disable request
type DisableMFARequest struct {
	Password string `json:"password" validate:"required"`
}

// DisableMFAResponse represents MFA disable response
type DisableMFAResponse struct {
	Message string `json:"message"`
}

// HandleDisableMFA handles POST /auth/mfa/disable
func (h *MFAHandler) HandleDisableMFA(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id")
	tenantIDStr := c.Locals("tenant_id")

	if userIDStr == nil || tenantIDStr == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user_id",
		})
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid tenant_id",
		})
	}

	var req DisableMFARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	input := auth.DisableMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Password: req.Password,
	}

	output, err := h.disableUseCase.Execute(c.Context(), input)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(DisableMFAResponse{
		Message: output.Message,
	})
}

// handleError converts domain errors to HTTP responses
func handleError(c *fiber.Ctx, err error) error {
	appErr := err.(*domain.AppError)

	switch appErr.Code {
	case domain.ErrNotFoundCode:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": appErr.Message,
		})
	case domain.ErrForbiddenCode:
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": appErr.Message,
		})
	case domain.ErrConflictCode:
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": appErr.Message,
		})
	case domain.ErrValidationCode:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": appErr.Message,
		})
	case domain.ErrUnauthorizedCode:
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": appErr.Message,
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}
}
