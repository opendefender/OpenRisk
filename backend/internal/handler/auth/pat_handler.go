// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	coreauth "github.com/opendefender/openrisk/internal/auth"
)

// PATHandler exposes DB-backed Personal Access Token management (L5): create, list,
// revoke. The raw token value is shown exactly once, at creation.
type PATHandler struct {
	svc   *coreauth.PersonalAccessTokenService
	audit *coreauth.AuditService
}

// NewPATHandler wires the PAT service.
func NewPATHandler(svc *coreauth.PersonalAccessTokenService, audit *coreauth.AuditService) *PATHandler {
	return &PATHandler{svc: svc, audit: audit}
}

type createPATRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays *int     `json:"expires_in_days,omitempty"`
}

// Create mints a new PAT for the authenticated user. Requires a full JWT session
// (a PAT cannot mint other PATs by convention).
func (h *PATHandler) Create(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}
	if c.Locals("is_pat") == true {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "personal access tokens cannot create other tokens"})
	}

	var req createPATRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	if len(req.Scopes) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "at least one scope is required"})
	}

	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}

	pat, raw, err := h.svc.CreateToken(c.UserContext(), userID, req.Name, req.Description, req.Scopes, expiresAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create token"})
	}

	if h.audit != nil {
		uid, tid := userID, tenantID
		_ = h.audit.LogFiber(c, &uid, &tid, coreauth.AuditActionPatCreate, true, nil)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":           pat.ID,
		"name":         pat.Name,
		"token_prefix": pat.TokenPrefix,
		"scopes":       req.Scopes,
		"expires_at":   pat.ExpiresAt,
		"created_at":   pat.CreatedAt,
		"token":        raw, // shown ONCE
	})
}

// List returns the caller's tokens (metadata only, never the secret).
func (h *PATHandler) List(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}
	tokens, err := h.svc.ListUserTokens(c.UserContext(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
	}
	return c.JSON(fiber.Map{"tokens": tokens})
}

// Revoke deletes one of the caller's tokens.
func (h *PATHandler) Revoke(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}
	tokenID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid token id"})
	}
	if err := h.svc.RevokeToken(c.UserContext(), tokenID, userID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}
	if h.audit != nil {
		uid, tid := userID, tenantID
		_ = h.audit.LogFiber(c, &uid, &tid, coreauth.AuditActionPatRevoke, true, nil)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
