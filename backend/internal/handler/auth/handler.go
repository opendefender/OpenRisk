// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// Handler handles authentication endpoints
type Handler struct {
	loginUseCase    *auth.LoginUseCase
	registerUseCase *auth.RegisterUseCase
	refreshUseCase  *auth.RefreshTokenUseCase
	logoutUseCase   *auth.LogoutUseCase
	passwordHasher  auth.PasswordHasher
	audit           *coreauth.AuditService // L7: full-fidelity auth audit trail (optional)
}

// NewHandler creates a new auth handler
func NewHandler(
	loginUseCase *auth.LoginUseCase,
	registerUseCase *auth.RegisterUseCase,
	refreshUseCase *auth.RefreshTokenUseCase,
	logoutUseCase *auth.LogoutUseCase,
	passwordHasher auth.PasswordHasher,
	audit *coreauth.AuditService,
) *Handler {
	return &Handler{
		loginUseCase:    loginUseCase,
		registerUseCase: registerUseCase,
		refreshUseCase:  refreshUseCase,
		logoutUseCase:   logoutUseCase,
		passwordHasher:  passwordHasher,
		audit:           audit,
	}
}

// logAudit records an auth event when an audit service is wired (nil-safe).
func (h *Handler) logAudit(c *fiber.Ctx, userID, tenantID *uuid.UUID, action coreauth.AuditAction, success bool, failureReason *string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.LogFiber(c, userID, tenantID, action, success, failureReason)
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email             string `json:"email" validate:"required,email"`
	Password          string `json:"password" validate:"required"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	User         *domain.User         `json:"user"`
	TokenPair    *coreauth.TokenPair  `json:"token_pair"`
	Organization *domain.Organization `json:"organization"`
}

// Login godoc
// @Summary User login
// @Description Authenticate user and return JWT tokens
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} fiber.Map
// @Failure 401 {object} fiber.Map
// @Router /auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Basic validation
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	// Device fingerprint may arrive in the body or (more naturally) as a header.
	if req.DeviceFingerprint == "" {
		req.DeviceFingerprint = c.Get("X-Device-Fingerprint")
	}

	result, err := h.loginUseCase.Execute(c.UserContext(), auth.LoginInput{
		Email:             req.Email,
		Password:          req.Password,
		DeviceFingerprint: req.DeviceFingerprint,
	})

	if err != nil {
		reason := "authentication failed"
		h.logAudit(c, nil, nil, coreauth.AuditActionLogin, false, &reason)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication failed",
		})
	}

	// L4 — MFA gate: password verified but a second factor is required. No session
	// is issued; the client must POST /auth/mfa/challenge with the mfa_token.
	if result.MFARequired {
		var tenantID *uuid.UUID
		if result.Organization != nil {
			tenantID = &result.Organization.ID
		}
		h.logAudit(c, &result.User.ID, tenantID, coreauth.AuditActionLogin, true, strptr("mfa_required"))
		return c.JSON(fiber.Map{
			"mfa_required": true,
			"mfa_token":    result.MFAToken,
			"user_id":      result.User.ID,
		})
	}

	var tenantID *uuid.UUID
	if result.Organization != nil {
		tenantID = &result.Organization.ID
	}
	h.logAudit(c, &result.User.ID, tenantID, coreauth.AuditActionLogin, true, nil)

	return c.JSON(LoginResponse{
		User:         result.User,
		TokenPair:    result.TokenPair,
		Organization: result.Organization,
	})
}

func strptr(s string) *string { return &s }

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Email       string `json:"email" validate:"required,email"`
	Username    string `json:"username" validate:"required,min=3,max=50"`
	Password    string `json:"password" validate:"required,min=8"`
	FullName    string `json:"full_name" validate:"required"`
	CompanyName string `json:"company_name" validate:"required"`
}

// RegisterResponse represents the registration response
type RegisterResponse struct {
	User         *domain.User         `json:"user"`
	Organization *domain.Organization `json:"organization"`
	Message      string               `json:"message"`
}

// Register godoc
// @Summary User registration
// @Description Register a new user and create organization
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration data"
// @Success 201 {object} RegisterResponse
// @Failure 400 {object} fiber.Map
// @Failure 409 {object} fiber.Map
// @Router /auth/register [post]
func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	result, err := h.registerUseCase.Execute(c.UserContext(), auth.RegisterInput{
		Email:       req.Email,
		Username:    req.Username,
		Password:    req.Password,
		FullName:    req.FullName,
		CompanyName: req.CompanyName,
	})

	if err != nil {
		if domainErr, ok := err.(*domain.AppError); ok {
			switch domainErr.Err {
			case domain.ErrValidation:
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": domainErr.Message,
				})
			case domain.ErrConflict:
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{
					"error": domainErr.Message,
				})
			}
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Registration failed",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(RegisterResponse{
		User:         result.User,
		Organization: result.Organization,
		Message:      result.Message,
	})
}

// RefreshTokenRequest represents the refresh token request body
type RefreshTokenRequest struct {
	RefreshToken      string `json:"refresh_token" validate:"required"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty"`
}

// RefreshTokenResponse represents the refresh token response
type RefreshTokenResponse struct {
	TokenPair *coreauth.TokenPair `json:"token_pair"`
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Refresh access token using refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} fiber.Map
// @Failure 401 {object} fiber.Map
// @Router /auth/refresh [post]
func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.DeviceFingerprint == "" {
		req.DeviceFingerprint = c.Get("X-Device-Fingerprint")
	}

	result, err := h.refreshUseCase.Execute(c.UserContext(), auth.RefreshTokenInput{
		RefreshToken:      req.RefreshToken,
		DeviceFingerprint: req.DeviceFingerprint,
	})

	if err != nil {
		reason := "token refresh failed"
		h.logAudit(c, nil, nil, coreauth.AuditActionRefresh, false, &reason)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token refresh failed",
		})
	}

	h.logAudit(c, nil, nil, coreauth.AuditActionRefresh, true, nil)
	return c.JSON(RefreshTokenResponse{
		TokenPair: result.TokenPair,
	})
}

// LogoutRequest represents the logout request body
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Logout godoc
// @Summary User logout
// @Description Revoke refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body LogoutRequest true "Refresh token to revoke"
// @Success 200 {object} fiber.Map
// @Failure 400 {object} fiber.Map
// @Failure 401 {object} fiber.Map
// @Router /auth/logout [post]
func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	err := h.logoutUseCase.Execute(c.UserContext(), auth.LogoutInput{
		RefreshToken: req.RefreshToken,
	})

	if err != nil {
		reason := "logout failed"
		h.logAudit(c, nil, nil, coreauth.AuditActionLogout, false, &reason)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Logout failed",
		})
	}

	h.logAudit(c, nil, nil, coreauth.AuditActionLogout, true, nil)
	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}

// Me godoc
// @Summary Get current user profile
// @Description Get profile of currently authenticated user
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.User
// @Failure 401 {object} fiber.Map
// @Router /auth/me [get]
func (h *Handler) Me(c *fiber.Ctx) error {
	// Get user from context (set by auth middleware)
	userID := c.Locals("user_id")
	if userID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// In a real implementation, you would fetch the user from the repository
	// For now, return a placeholder
	return c.JSON(fiber.Map{
		"id":    userID,
		"email": "user@example.com", // This should come from the database
	})
}
