package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// RBACRoleHandler manages role RBAC operations
type RBACRoleHandler struct {
	roleService       *services.RoleService
	permissionService *services.PermissionService
	userService       *services.UserService
}

// NewRBACRoleHandler creates a new RBAC role handler
func NewRBACRoleHandler(
	roleService *services.RoleService,
	permissionService *services.PermissionService,
	userService *services.UserService,
) *RBACRoleHandler {
	return &RBACRoleHandler{
		roleService:       roleService,
		permissionService: permissionService,
		userService:       userService,
	}
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
	tenantID := c.Locals("tenantID").(uuid.UUID)

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
	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	ctx := c.Context()

	// Get role
	role, err := h.roleService.GetRole(ctx, roleID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "role not found",
		})
	}

	// Get permissions
	permissions, err := h.roleService.GetRolePermissions(ctx, roleID)
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
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

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
type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

// UpdateRole updates an existing role
func (h *RBACRoleHandler) UpdateRole(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

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
	role, err := h.roleService.GetRole(ctx, roleID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "role not found",
		})
	}

	// Update fields
	role.Name = req.Name
	role.Description = req.Description
	role.IsActive = req.IsActive

	// Update role
	if err := h.roleService.UpdateRole(ctx, role); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(role)
}

// DeleteRole deletes a custom role
func (h *RBACRoleHandler) DeleteRole(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

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
	if err := h.roleService.DeleteRole(ctx, roleID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
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
	roleIDStr := c.Params("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	ctx := c.Context()

	permissions, err := h.roleService.GetRolePermissions(ctx, roleID)
	if err != nil {
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
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

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
	if err := h.roleService.AssignPermissionToRole(ctx, roleID, permissionID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
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
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

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
	if err := h.roleService.RemovePermissionFromRole(ctx, roleID, permissionID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "permission removed successfully",
	})
}
