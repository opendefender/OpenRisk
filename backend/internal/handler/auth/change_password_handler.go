// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// WithChangePassword enables POST /auth/password/change (#720).
func (h *PasswordHandler) WithChangePassword(uc *appauth.ChangePasswordUseCase) *PasswordHandler {
	h.change = uc
	return h
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	Locale          string `json:"locale,omitempty"`
}

// ChangePassword changes the signed-in user's own password.
//
// The user and the session to keep come from the session itself. Neither
// password is ever logged or echoed: audit failures carry a reason code only.
func (h *PasswordHandler) ChangePassword(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil || mwCtx.UserID == uuid.Nil || h.change == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	locale := resolveLocale(c, req.Locale)
	userID := mwCtx.UserID

	out, err := h.change.Execute(c.UserContext(), appauth.ChangePasswordInput{
		UserID:             userID,
		CurrentPassword:    req.CurrentPassword,
		NewPassword:        req.NewPassword,
		CurrentSessionHash: currentSessionHash(c),
		Locale:             locale,
	})

	fail := func(status int, reason, code, message string, extra fiber.Map) error {
		h.logChange(c, userID, false, &reason)
		body := fiber.Map{"error": message, "code": code}
		for k, v := range extra {
			body[k] = v
		}
		return c.Status(status).JSON(body)
	}

	switch {
	case errors.Is(err, appauth.ErrCurrentPasswordIncorrect):
		return fail(fiber.StatusForbidden, "wrong_current_password", "wrong_current_password",
			pick(locale, "Le mot de passe actuel est incorrect.", "The current password is incorrect."), nil)
	case errors.Is(err, appauth.ErrNoLocalPassword):
		return fail(fiber.StatusConflict, "no_local_password", "no_local_password",
			pick(locale,
				"Ce compte se connecte via votre fournisseur d'identité : son mot de passe se change chez lui.",
				"This account signs in through your identity provider: its password is changed there."), nil)
	case errors.Is(err, appauth.ErrSamePassword):
		return fail(fiber.StatusBadRequest, "same_password", "same_password",
			pick(locale, "Le nouveau mot de passe doit être différent de l'actuel.", "The new password must differ from the current one."), nil)
	case err != nil && out != nil && out.Assessment != nil:
		return fail(fiber.StatusBadRequest, "weak_password", "weak_password",
			out.Assessment.Blocking[0].Localised(locale), fiber.Map{"assessment": out.Assessment})
	case err != nil:
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return fail(appErr.Code, "rejected", "rejected", domain.MessageFromError(err), nil)
		}
		return fail(fiber.StatusInternalServerError, "internal", "internal", genericFailure(locale), nil)
	}

	h.logChange(c, userID, true, nil)
	return c.JSON(fiber.Map{
		"message":                pick(locale, "Mot de passe modifié. Vos autres appareils ont été déconnectés.", "Password changed. Your other devices were signed out."),
		"other_sessions_revoked": out.OtherSessionsRevoked,
	})
}

func (h *PasswordHandler) logChange(c *fiber.Ctx, userID uuid.UUID, success bool, reason *string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.LogFiber(c, &userID, nil, coreauth.AuditActionPasswordChange, success, reason)
}

func pick(locale, fr, en string) string {
	if locale == "en" {
		return en
	}
	return fr
}
