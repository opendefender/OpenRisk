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
	"github.com/opendefender/openrisk/internal/middleware"
)

// SessionHandler serves Settings → Sessions: which devices are signed in, and
// signing them out.
type SessionHandler struct {
	list        *appauth.ListSessionsUseCase
	revoke      *appauth.RevokeSessionUseCase
	revokeOther *appauth.RevokeOtherSessionsUseCase
	audit       *coreauth.AuditService
}

// NewSessionHandler builds the handler.
func NewSessionHandler(
	list *appauth.ListSessionsUseCase,
	revoke *appauth.RevokeSessionUseCase,
	revokeOther *appauth.RevokeOtherSessionsUseCase,
	audit *coreauth.AuditService,
) *SessionHandler {
	return &SessionHandler{list: list, revoke: revoke, revokeOther: revokeOther, audit: audit}
}

// currentSessionHash identifies the caller's own session from its refresh token.
//
// The token arrives as an HttpOnly cookie (see the cookie-session work) or, for
// API clients, in the body of a refresh. Hashing it here means the handler can
// mark "this device" without the raw token ever being compared or logged.
func currentSessionHash(c *fiber.Ctx) string {
	raw := c.Cookies(middleware.RefreshTokenCookie)
	if raw == "" {
		raw = c.Get("X-Refresh-Token")
	}
	if raw == "" {
		return ""
	}
	return coreauth.HashToken(raw)
}

// ListSessions returns every device signed in to the caller's account.
func (h *SessionHandler) ListSessions(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil || mwCtx.UserID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	records, err := h.list.Execute(c.UserContext(), mwCtx.UserID, currentSessionHash(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list sessions"})
	}
	return c.JSON(fiber.Map{"sessions": records, "total": len(records)})
}

// RevokeSession signs one device out.
//
// Scoped to the caller's own account by the use case; a session ID belonging to
// somebody else reads as "not found" rather than "forbidden", so the endpoint
// cannot be used to probe which session IDs exist.
func (h *SessionHandler) RevokeSession(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil || mwCtx.UserID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	sessionID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session id"})
	}

	err = h.revoke.Execute(c.UserContext(), mwCtx.UserID, sessionID)
	switch {
	case errors.Is(err, appauth.ErrSessionNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
	case err != nil:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke session"})
	}

	h.logAudit(c, &mwCtx.UserID, coreauth.AuditActionSessionRevoke, true)
	return c.SendStatus(fiber.StatusNoContent)
}

// RevokeOtherSessions signs out every device except the caller's own.
//
// This is the "someone else is in my account" control. It keeps the current
// session on purpose: a button that also logs you out is one people hesitate to
// press, and hesitation is the opposite of what this moment needs.
func (h *SessionHandler) RevokeOtherSessions(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil || mwCtx.UserID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	revoked, err := h.revokeOther.Execute(c.UserContext(), mwCtx.UserID, currentSessionHash(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke sessions"})
	}

	h.logAudit(c, &mwCtx.UserID, coreauth.AuditActionSessionRevokeAll, true)
	return c.JSON(fiber.Map{"revoked": revoked})
}

func (h *SessionHandler) logAudit(c *fiber.Ctx, userID *uuid.UUID, action coreauth.AuditAction, success bool) {
	if h.audit == nil {
		return
	}
	_ = h.audit.LogFiber(c, userID, nil, action, success, nil)
}
