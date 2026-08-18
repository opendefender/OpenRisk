// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/application/orgdeletion"
	"github.com/opendefender/openrisk/internal/infrastructure/orgexport"
)

// OrgDeletionHandler exposes the danger zone: export the org, schedule an erasure
// (export + name confirmation + MFA + 30-day grace), inspect it, and cancel it.
// All routes are admin-guarded at registration.
type OrgDeletionHandler struct {
	svc      *orgdeletion.Service
	exporter *orgexport.Exporter
}

func NewOrgDeletionHandler(svc *orgdeletion.Service, exporter *orgexport.Exporter) *OrgDeletionHandler {
	return &OrgDeletionHandler{svc: svc, exporter: exporter}
}

// GetDeletion handles GET /organization/deletion → the pending request or null.
func (h *OrgDeletionHandler) GetDeletion(c *fiber.Ctx) error {
	req, err := h.svc.GetActive(c.UserContext(), tenantID(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unavailable"})
	}
	if req == nil {
		return c.JSON(fiber.Map{"pending": false})
	}
	return c.JSON(fiber.Map{
		"pending":            true,
		"request":            req,
		"days_remaining":     req.DaysRemaining(time.Now()),
		"scheduled_purge_at": req.ScheduledPurgeAt,
	})
}

type deletionInput struct {
	ConfirmName string `json:"confirm_name"`
	MFACode     string `json:"mfa_code"`
	Reason      string `json:"reason"`
}

// RequestDeletion handles POST /organization/deletion.
func (h *OrgDeletionHandler) RequestDeletion(c *fiber.Ctx) error {
	var in deletionInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	req, err := h.svc.Request(c.UserContext(), tenantID(c), userID(c), in.ConfirmName, in.MFACode, in.Reason)
	if err != nil {
		return deletionError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"request":            req,
		"scheduled_purge_at": req.ScheduledPurgeAt,
		"grace_days":         req.DaysRemaining(time.Now()),
	})
}

// CancelDeletion handles POST /organization/deletion/cancel.
func (h *OrgDeletionHandler) CancelDeletion(c *fiber.Ctx) error {
	if err := h.svc.Cancel(c.UserContext(), tenantID(c), userID(c)); err != nil {
		return deletionError(c, err)
	}
	return c.JSON(fiber.Map{"canceled": true})
}

// ExportOrganization handles GET /organization/export → a downloadable JSON bundle
// of the tenant's data (also the export the deletion flow requires).
func (h *OrgDeletionHandler) ExportOrganization(c *fiber.Ctx) error {
	path, err := h.exporter.Export(c.UserContext(), tenantID(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "export failed"})
	}
	c.Set("Content-Disposition", "attachment; filename=openrisk-export.json")
	return c.Download(path)
}

func deletionError(c *fiber.Ctx, err error) error {
	switch err {
	case orgdeletion.ErrNameMismatch:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "code": "name_mismatch"})
	case orgdeletion.ErrMFARequired:
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error(), "code": "mfa_required"})
	case orgdeletion.ErrAlreadyPending:
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error(), "code": "already_pending"})
	case orgdeletion.ErrNoActiveRequest:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error(), "code": "no_active_request"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
}
