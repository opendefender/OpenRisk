// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

// PasswordHandler serves the unauthenticated password endpoints: strength
// checking, reset request, and reset confirmation.
type PasswordHandler struct {
	request *appauth.RequestPasswordResetUseCase
	confirm *appauth.ConfirmPasswordResetUseCase
	policy  *pwpolicy.Policy
	baseURL string
	audit   *coreauth.AuditService
}

// NewPasswordHandler builds the handler.
func NewPasswordHandler(
	request *appauth.RequestPasswordResetUseCase,
	confirm *appauth.ConfirmPasswordResetUseCase,
	policy *pwpolicy.Policy,
	baseURL string,
	audit *coreauth.AuditService,
) *PasswordHandler {
	return &PasswordHandler{request: request, confirm: confirm, policy: policy, baseURL: baseURL, audit: audit}
}

// ---------------------------------------------------------------------------
// Strength check
// ---------------------------------------------------------------------------

type checkPasswordRequest struct {
	Password string `json:"password"`
	// Email/Name let the server judge the password against the identity it will
	// belong to, the same way registration and reset do.
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// CheckPassword scores a candidate password and explains it.
//
// This exists so the strength meter and the enforced rule are the same rule.
// The SPA also scores locally for instant feedback, but a client-side estimate
// can drift from the server's — and when it does, the user is told "strong"
// and then refused on submit. This endpoint is the authority the meter defers
// to, and it is the same pwpolicy.Policy the write paths call.
//
// It is unauthenticated because it has to work on the registration and reset
// screens, where there is no session. It reveals nothing: the caller already
// knows the password they typed.
func (h *PasswordHandler) CheckPassword(c *fiber.Ctx) error {
	var req checkPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	assessment := h.policy.Assess(c.UserContext(), req.Password, []string{req.Email, req.Name})
	return c.JSON(assessment)
}

// ---------------------------------------------------------------------------
// Reset request
// ---------------------------------------------------------------------------

type forgotPasswordRequest struct {
	Email  string `json:"email"`
	Locale string `json:"locale,omitempty"`
}

// ForgotPassword starts a password reset.
//
// The response is byte-identical whether or not the address has an account, and
// so is the status code. That is the entire security property of this endpoint:
// anything that varies — a different message, a different status, a measurably
// different latency — turns the login screen into an account directory. Mail is
// dispatched asynchronously for the same reason (see authmail.Async).
//
// Rate limiting returns 429, and that does NOT break the property: the counter
// is keyed on the address and incremented for every request, including ones for
// addresses with no account, so both cases hit the cap at the same point.
func (h *PasswordHandler) ForgotPassword(c *fiber.Ctx) error {
	var req forgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	locale := resolveLocale(c, req.Locale)

	if strings.TrimSpace(req.Email) == "" {
		// A missing field is a malformed request, not a failed lookup — it says
		// nothing about any account.
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is required"})
	}

	_, err := h.request.Execute(c.UserContext(), appauth.RequestPasswordResetInput{
		Email:     req.Email,
		Locale:    locale,
		IP:        c.IP(),
		UserAgent: c.Get("User-Agent"),
		BaseURL:   h.baseURL,
	})

	switch {
	case errors.Is(err, appauth.ErrResetRateLimited):
		h.logAuth(c, coreauth.AuditActionPasswordResetRequest, false, strPtr("rate_limited"))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       rateLimitMessage(locale),
			"retry_after": int(domain.PasswordResetWindow.Seconds()),
		})
	case err != nil:
		// Do not surface storage failures differently per branch either.
		h.logAuth(c, coreauth.AuditActionPasswordResetRequest, false, strPtr(err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": genericFailure(locale)})
	}

	h.logAuth(c, coreauth.AuditActionPasswordResetRequest, true, nil)
	return c.JSON(fiber.Map{"message": uniformAcknowledgement(locale)})
}

// ---------------------------------------------------------------------------
// Reset confirmation
// ---------------------------------------------------------------------------

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
	Locale      string `json:"locale,omitempty"`
}

// ResetPassword sets a new password from a reset link.
//
// A policy refusal returns 422 with the full assessment attached, so the SPA can
// render the same actionable guidance the meter shows rather than a bare
// "invalid". A bad token returns 400 with one message covering unknown, expired
// and already-used alike — distinguishing them would confirm a token once
// existed, and with it the account.
func (h *PasswordHandler) ResetPassword(c *fiber.Ctx) error {
	var req resetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	locale := resolveLocale(c, req.Locale)

	out, err := h.confirm.Execute(c.UserContext(), appauth.ConfirmPasswordResetInput{
		Token:       req.Token,
		NewPassword: req.NewPassword,
		Locale:      locale,
	})

	switch {
	case errors.Is(err, appauth.ErrInvalidResetToken):
		h.logAuth(c, coreauth.AuditActionPasswordResetConfirm, false, strPtr("invalid_token"))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": invalidTokenMessage(locale),
			"code":  "invalid_token",
		})

	case err != nil && out != nil && out.Assessment != nil:
		// Policy refusal: hand back the reasons, both renderings, so the client
		// shows what to fix without re-implementing the policy.
		h.logAuth(c, coreauth.AuditActionPasswordResetConfirm, false, strPtr("weak_password"))
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      out.Assessment.Blocking[0].Localised(locale),
			"code":       "weak_password",
			"assessment": out.Assessment,
		})

	case err != nil:
		h.logAuth(c, coreauth.AuditActionPasswordResetConfirm, false, strPtr(err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": genericFailure(locale)})
	}

	h.logAuth(c, coreauth.AuditActionPasswordResetConfirm, true, nil)
	return c.JSON(fiber.Map{
		"message":          resetSuccessMessage(locale),
		"sessions_revoked": out.SessionsRevoked,
	})
}

// ---------------------------------------------------------------------------
// Copy
// ---------------------------------------------------------------------------

// uniformAcknowledgement is the single answer to a reset request.
//
// Phrased conditionally ("if an account exists") so it is truthful in both
// cases. A message like "check your inbox" would be a lie half the time, and
// users who mistyped their address would sit waiting for mail that never comes.
func uniformAcknowledgement(locale string) string {
	if locale == "en" {
		return "If an account exists for this address, we've sent a reset link. It expires in 30 minutes."
	}
	return "Si un compte existe pour cette adresse, nous venons d'envoyer un lien de réinitialisation. Il expire dans 30 minutes."
}

func rateLimitMessage(locale string) string {
	if locale == "en" {
		return "Too many reset requests for this address. Try again in an hour."
	}
	return "Trop de demandes de réinitialisation pour cette adresse. Réessayez dans une heure."
}

func invalidTokenMessage(locale string) string {
	if locale == "en" {
		return "This reset link is no longer valid — it may have expired or already been used. Request a new one."
	}
	return "Ce lien de réinitialisation n'est plus valide — il a peut-être expiré ou déjà été utilisé. Demandez-en un nouveau."
}

func resetSuccessMessage(locale string) string {
	if locale == "en" {
		return "Your password has been changed and every active session was signed out."
	}
	return "Votre mot de passe a été modifié et toutes les sessions actives ont été déconnectées."
}

func genericFailure(locale string) string {
	if locale == "en" {
		return "Something went wrong. Please try again shortly."
	}
	return "Une erreur est survenue. Réessayez dans un instant."
}

// resolveLocale picks the response language: explicit body field first, then the
// Accept-Language header, defaulting to French (the product's primary market).
func resolveLocale(c *fiber.Ctx, explicit string) string {
	if explicit == "en" || explicit == "fr" {
		return explicit
	}
	if strings.HasPrefix(strings.ToLower(c.Get("Accept-Language")), "en") {
		return "en"
	}
	return "fr"
}

func (h *PasswordHandler) logAuth(c *fiber.Ctx, action coreauth.AuditAction, success bool, reason *string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.LogFiber(c, nil, nil, action, success, reason)
}

func strPtr(s string) *string { return &s }
