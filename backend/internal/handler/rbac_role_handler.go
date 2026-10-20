// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/service"
)

// RBACRoleHandler manages role RBAC operations
type RBACRoleHandler struct {
	roleService       *service.RoleService
	permissionService *service.PermissionService
	userService       *service.UserService
}

// NewRBACRoleHandler creates a new RBAC role handler
func NewRBACRoleHandler(
	roleService *service.RoleService,
	permissionService *service.PermissionService,
	userService *service.UserService,
) *RBACRoleHandler {
	return &RBACRoleHandler{
		roleService:       roleService,
		permissionService: permissionService,
		userService:       userService,
	}
}

// roleErrorStatus maps a role service error to its HTTP status. A role of
// another tenant is ErrNotFound, so it answers 404 like a missing role.
func roleErrorStatus(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return fiber.StatusNotFound
	case errors.Is(err, domain.ErrForbidden):
		return fiber.StatusForbidden
	case errors.Is(err, domain.ErrConflict):
		return fiber.StatusConflict
	default:
		return fiber.StatusBadRequest
	}
}

// roleErrorMessage returns the text of a typed role error, and a generic
// message for anything else, so database errors never reach the client.
func roleErrorMessage(err error) string {
	for _, typed := range []error{domain.ErrNotFound, domain.ErrForbidden, domain.ErrConflict, domain.ErrValidation} {
		if errors.Is(err, typed) {
			return err.Error()
		}
	}
	return "request failed"
}

// ListRolesResponse contains paginated role list
type ListRolesResponse struct {
	Roles      []domain.RoleEnhanced `json:"roles"`
	Total      int64                 `json:"total"`
	Limit      int                   `json:"limit"`
	Offset     int                   `json:"offset"`
	HasMore    bool                  `json:"has_more"`
	TotalPages int                   `json:"total_pages"`
}

// ListRoles retrieves all roles in a tenant
func (h *RBACRoleHandler) ListRoles(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")

	limit := 20
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	ctx := c.Context()
	roles, total, err := h.roleService.ListRoles(ctx, tenantID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list roles",
		})
	}

	totalPages := (int(total) + limit - 1) / limit

	return c.JSON(ListRolesResponse{
		Roles:      roles,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		HasMore:    offset+limit < int(total),
		TotalPages: totalPages,
	})
}

// GetRoleResponse contains role details with permissions
type GetRoleResponse struct {
	Role        domain.RoleEnhanced   `json:"role"`
	Permissions []domain.PermissionDB `json:"permissions"`
	UserCount   int64                 `json:"user_count"`
}

// GetRole retrieves a specific role with its permissions
func (h *RBACRoleHandler) GetRole(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no tenant in session",
		})
	}

	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	ctx := c.Context()

	// Get role
	role, err := h.roleService.GetRole(ctx, tenantID, roleID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "role not found",
		})
	}

	// Get permissions
	permissions, err := h.roleService.GetRolePermissions(ctx, tenantID, roleID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve permissions",
		})
	}

	// Note: User count would require a database query
	// For now, returning 0 as placeholder

	return c.JSON(GetRoleResponse{
		Role:        *role,
		Permissions: permissions,
		UserCount:   0,
	})
}

// CreateRoleRequest defines request body for creating a role
type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Level       int    `json:"level"`
}

// CreateRole creates a new custom role in the tenant
func (h *RBACRoleHandler) CreateRole(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")
	requestorID := safeGetUUID(c, "userID")

	var req CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx := c.Context()

	// Check if requestor is admin
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can create roles",
		})
	}

	// Validate role level
	if !domain.ValidateRoleLevel(domain.RoleLevel(req.Level)) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role level",
		})
	}

	// Create role
	role := &domain.RoleEnhanced{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Name:         req.Name,
		Description:  req.Description,
		Level:        domain.RoleLevel(req.Level),
		IsPredefined: false,
		IsActive:     true,
	}

	if err := h.roleService.CreateRole(ctx, role); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(role)
}

// UpdateRoleRequest defines request body for updating a role
// Pointers, so a PATCH changes only the fields it sends.
type UpdateRoleRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

// UpdateRole updates an existing role
func (h *RBACRoleHandler) UpdateRole(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no tenant in session",
		})
	}
	requestorID := safeGetUUID(c, "userID")

	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	var req UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx := c.Context()

	// Check if requestor is admin
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can update roles",
		})
	}

	// Get existing role
	role, err := h.roleService.GetRole(ctx, tenantID, roleID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "role not found",
		})
	}

	// Update fields
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "role name is required",
			})
		}
		role.Name = *req.Name
	}
	if req.Description != nil {
		role.Description = *req.Description
	}
	if req.IsActive != nil {
		role.IsActive = *req.IsActive
	}

	// Update role
	if err := h.roleService.UpdateRole(ctx, tenantID, role); err != nil {
		return c.Status(roleErrorStatus(err)).JSON(fiber.Map{
			"error": roleErrorMessage(err),
		})
	}

	return c.JSON(role)
}

// DeleteRole deletes a custom role
func (h *RBACRoleHandler) DeleteRole(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no tenant in session",
		})
	}
	requestorID := safeGetUUID(c, "userID")

	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	ctx := c.Context()

	// Check if requestor is admin
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can delete roles",
		})
	}

	// Delete role
	if err := h.roleService.DeleteRole(ctx, tenantID, roleID); err != nil {
		return c.Status(roleErrorStatus(err)).JSON(fiber.Map{
			"error": roleErrorMessage(err),
		})
	}

	return c.JSON(fiber.Map{
		"message": "role deleted successfully",
	})
}

// GetRolePermissionsResponse contains role permissions
type GetRolePermissionsResponse struct {
	RoleID      uuid.UUID             `json:"role_id"`
	Permissions []domain.PermissionDB `json:"permissions"`
}

// GetRolePermissions retrieves all permissions for a role
func (h *RBACRoleHandler) GetRolePermissions(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no tenant in session",
		})
	}

	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	ctx := c.Context()

	permissions, err := h.roleService.GetRolePermissions(ctx, tenantID, roleID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "role not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve permissions",
		})
	}

	return c.JSON(GetRolePermissionsResponse{
		RoleID:      roleID,
		Permissions: permissions,
	})
}

// AssignPermissionRequest defines request body for assigning permission
type AssignPermissionRequest struct {
	PermissionID string `json:"permission_id"`
}

// AssignPermissionToRole assigns a permission to a role
func (h *RBACRoleHandler) AssignPermissionToRole(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no tenant in session",
		})
	}
	requestorID := safeGetUUID(c, "userID")

	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	var req AssignPermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	permissionID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid permission ID format",
		})
	}

	ctx := c.Context()

	// Check if requestor is admin
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can assign permissions",
		})
	}

	// Assign permission
	if err := h.roleService.AssignPermissionToRole(ctx, tenantID, roleID, permissionID); err != nil {
		return c.Status(roleErrorStatus(err)).JSON(fiber.Map{
			"error": roleErrorMessage(err),
		})
	}

	return c.JSON(fiber.Map{
		"message": "permission assigned successfully",
	})
}

// RemovePermissionRequest defines request body for removing permission
type RemovePermissionRequest struct {
	PermissionID string `json:"permission_id"`
}

// RemovePermissionFromRole removes a permission from a role
func (h *RBACRoleHandler) RemovePermissionFromRole(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenantID")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no tenant in session",
		})
	}
	requestorID := safeGetUUID(c, "userID")

	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	var req RemovePermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	permissionID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid permission ID format",
		})
	}

	ctx := c.Context()

	// Check if requestor is admin
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can remove permissions",
		})
	}

	// Remove permission
	if err := h.roleService.RemovePermissionFromRole(ctx, tenantID, roleID, permissionID); err != nil {
		return c.Status(roleErrorStatus(err)).JSON(fiber.Map{
			"error": roleErrorMessage(err),
		})
	}

	return c.JSON(fiber.Map{
		"message": "permission removed successfully",
	})
}
