// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/profile"
	"github.com/opendefender/openrisk/internal/domain"
)

// ProfileHandler serves the caller's own profile, preferences and avatar
// (#719). Every route acts on the session's user: the session is the
// authorization, and no path or body field can name another account — except
// GET /users/:id/avatar, which the use case gates by tenant membership.
type ProfileHandler struct {
	svc *profile.Service
}

func NewProfileHandler(svc *profile.Service) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

// GetMe — GET /users/me
func (h *ProfileHandler) GetMe(c *fiber.Ctx) error {
	v, err := h.svc.GetMyProfile(c.UserContext(), tenantID(c), userID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// UpdateMe — PATCH /users/me
func (h *ProfileHandler) UpdateMe(c *fiber.Ctx) error {
	var patch domain.UserProfilePatch
	if err := c.BodyParser(&patch); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	v, err := h.svc.UpdateMyProfile(c.UserContext(), tenantID(c), userID(c), patch)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// UploadMyAvatar — PUT /users/me/avatar (multipart field "file")
func (h *ProfileHandler) UploadMyAvatar(c *fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return writeAppError(c, domain.NewValidationError("avatar: a file field is required"))
	}
	f, err := fh.Open()
	if err != nil {
		return writeAppError(c, domain.NewValidationError("avatar: unreadable upload"))
	}
	defer f.Close()
	v, err := h.svc.UploadMyAvatar(c.UserContext(), tenantID(c), userID(c), f)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// DeleteMyAvatar — DELETE /users/me/avatar
func (h *ProfileHandler) DeleteMyAvatar(c *fiber.Ctx) error {
	v, err := h.svc.DeleteMyAvatar(c.UserContext(), tenantID(c), userID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// GetAvatar — GET /users/:id/avatar
func (h *ProfileHandler) GetAvatar(c *fiber.Ctx) error {
	target, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return writeAppError(c, domain.NewNotFoundError("avatar", c.Params("id")))
	}
	a, err := h.svc.GetAvatar(c.UserContext(), tenantID(c), userID(c), target)
	if err != nil {
		return writeAppError(c, err)
	}
	c.Set(fiber.HeaderContentType, a.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderCacheControl, "private, max-age=300")
	c.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	return c.Send(a.Data)
}
