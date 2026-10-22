// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// NoticeEmailPolicyHandler serves Settings → Notifications → "E-mails d'avis"
// (#997). Any member may read the window, since it decides how soon they hear
// about changes. Writing is admin and root only, enforced by the route guard.
type NoticeEmailPolicyHandler struct {
	get    *notificationapp.GetNoticeEmailPolicyUseCase
	update *notificationapp.UpdateNoticeEmailPolicyUseCase
}

func NewNoticeEmailPolicyHandler(get *notificationapp.GetNoticeEmailPolicyUseCase, update *notificationapp.UpdateNoticeEmailPolicyUseCase) *NoticeEmailPolicyHandler {
	return &NoticeEmailPolicyHandler{get: get, update: update}
}

func noticePolicyCaller(c *fiber.Ctx) (tenantID, userID uuid.UUID) {
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		return mwCtx.OrganizationID, mwCtx.UserID
	}
	return uuid.Nil, uuid.Nil
}

// Get godoc
// @Summary Read the organization's notice e-mail grouping window
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} notification.NoticeEmailPolicyView
// @Router /notifications/email-policy [get]
func (h *NoticeEmailPolicyHandler) Get(c *fiber.Ctx) error {
	tenantID, _ := noticePolicyCaller(c)
	view, err := h.get.Execute(c.UserContext(), tenantID)
	if err != nil {
		return c.Status(domain.HTTPStatusFromError(err)).JSON(fiber.Map{"error": domain.MessageFromError(err)})
	}
	return c.JSON(view)
}

type updateNoticeEmailPolicyRequest struct {
	WindowMinutes *int `json:"window_minutes"`
}

// Update godoc
// @Summary Set the organization's notice e-mail grouping window
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} notification.NoticeEmailPolicyView
// @Failure 400 {object} fiber.Map
// @Router /notifications/email-policy [put]
func (h *NoticeEmailPolicyHandler) Update(c *fiber.Ctx) error {
	var req updateNoticeEmailPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.WindowMinutes == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "window_minutes is required"})
	}
	tenantID, actorID := noticePolicyCaller(c)
	view, err := h.update.Execute(c.UserContext(), notificationapp.UpdateNoticeEmailPolicyInput{
		TenantID: tenantID, ActorID: actorID, WindowMinutes: *req.WindowMinutes,
	})
	if err != nil {
		return c.Status(domain.HTTPStatusFromError(err)).JSON(fiber.Map{"error": domain.MessageFromError(err)})
	}
	return c.JSON(view)
}
