// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	apprbac "github.com/opendefender/openrisk/internal/application/rbac"
	"github.com/opendefender/openrisk/internal/domain"
)

// BusinessRoleHandler exposes the RBAC catalog (permissions + business-role
// presets), the tenant's members with their resolved access, and the
// assign-a-business-role action. Reads are open to any authenticated member (so
// the UI can render matrices); the assign action is admin-gated at the route.
type BusinessRoleHandler struct {
	listMembers *apprbac.ListMembersUseCase
	assignRole  *apprbac.AssignBusinessRoleUseCase
}

// NewBusinessRoleHandler builds the handler.
func NewBusinessRoleHandler(listMembers *apprbac.ListMembersUseCase, assignRole *apprbac.AssignBusinessRoleUseCase) *BusinessRoleHandler {
	return &BusinessRoleHandler{listMembers: listMembers, assignRole: assignRole}
}

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

// ListMembers returns the tenant's members with their org role, business role
// and resolved permission set.
// GET /rbac/members
func (h *BusinessRoleHandler) ListMembers(c *fiber.Ctx) error {
	views, err := h.listMembers.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"members": views})
}

// AssignBusinessRoleRequest is the assign-role body.
type AssignBusinessRoleRequest struct {
	BusinessRole string `json:"business_role"` // preset key, or "" to clear
	MemberRole   string `json:"member_role"`   // optional: "admin" | "user"
}

// AssignBusinessRole assigns (or clears) a member's business role, optionally
// changing their org role in the same call.
// PUT /rbac/members/:userId/business-role
func (h *BusinessRoleHandler) AssignBusinessRole(c *fiber.Ctx) error {
	targetID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	var req AssignBusinessRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	view, err := h.assignRole.Execute(c.UserContext(), tenantID(c), apprbac.AssignBusinessRoleInput{
		TargetUserID: targetID,
		BusinessRole: domain.BusinessRoleKey(req.BusinessRole),
		MemberRole:   domain.MemberRole(req.MemberRole),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}
