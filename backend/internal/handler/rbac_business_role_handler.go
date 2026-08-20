// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/domain"
)

// BusinessRoleHandler exposes the RBAC catalog: the permission vocabulary and
// the business-role presets, so the UI can render a self-describing permission
// matrix. Open to any authenticated member — it is reference material about the
// product, not data about anybody's organization.
//
// Listing members, inviting them and assigning their roles used to live here
// too. They now live under /organization/members (W0-04), which is one system
// rather than two: this handler's invite created a user immediately and handed
// the administrator a temporary password to relay, with no token, no expiry, no
// revocation and no resend.
type BusinessRoleHandler struct{}

// NewBusinessRoleHandler builds the handler.
func NewBusinessRoleHandler() *BusinessRoleHandler { return &BusinessRoleHandler{} }

// RBACCatalogResponse is the self-describing permission matrix payload.
type RBACCatalogResponse struct {
	Permissions   []domain.PermissionDef `json:"permissions"`
	BusinessRoles []domain.BusinessRole  `json:"business_roles"`
}

// GetCatalog returns the full permission catalog and the business-role presets.
// GET /rbac/business-roles
func (h *BusinessRoleHandler) GetCatalog(c *fiber.Ctx) error {
	return c.JSON(RBACCatalogResponse{
		Permissions:   domain.PermissionCatalog,
		BusinessRoles: domain.ListBusinessRoles(),
	})
}
