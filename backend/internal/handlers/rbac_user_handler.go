package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/services"
)

// RBACUserHandler manages user RBAC operations
type RBACUserHandler struct {
	userService   *services.UserService
	roleService   *services.RoleService
	tenantService *services.TenantService
}

// NewRBACUserHandler creates a new RBAC user handler
func NewRBACUserHandler(
	userService *services.UserService,
	roleService *services.RoleService,
	tenantService *services.TenantService,
) *RBACUserHandler {
	return &RBACUserHandler{
		userService:   userService,
		roleService:   roleService,
		tenantService: tenantService,
	}
}

// ListUsersRequest defines query parameters for listing users
type ListUsersRequest struct {
	TenantID string `query:"tenant_id"`
	RoleID   string `query:"role_id"`
	Limit    int    `query:"limit"`
	Offset   int    `query:"offset"`
}

// ListUsersResponse contains paginated user list
type ListUsersResponse struct {
	Users      interface{} `json:"users"`
	Total      int64       `json:"total"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	HasMore    bool        `json:"has_more"`
	TotalPages int         `json:"total_pages"`
}

// ListUsers retrieves all users in a tenant
func (h *RBACUserHandler) ListUsers(c *fiber.Ctx) error {
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
	userTenants, total, err := h.userService.GetTenantUsers(ctx, tenantID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list users",
		})
	}

	totalPages := (int(total) + limit - 1) / limit

	return c.JSON(ListUsersResponse{
		Users:      userTenants,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		HasMore:    offset+limit < int(total),
		TotalPages: totalPages,
	})
}

// GetUserRequest is unused but defined for consistency
type GetUserRequest struct {
	UserID string `params:"user_id"`
}

// GetUser retrieves a specific user's tenant information
func (h *RBACUserHandler) GetUser(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)

	userIDStr := c.Params("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid user ID format",
		})
	}

	ctx := c.Context()
	userTenant, err := h.userService.GetUserInTenant(ctx, userID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found in tenant",
		})
	}

	return c.JSON(userTenant)
}

// AddUserToTenantRequest defines request body for adding user to tenant
type AddUserToTenantRequest struct {
	UserID string `json:"user_id"`
	RoleID string `json:"role_id"`
}

// AddUserToTenant adds a user to the current tenant with a role
func (h *RBACUserHandler) AddUserToTenant(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

	var req AddUserToTenantRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid user ID format",
		})
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	ctx := c.Context()

	// Check if requestor has permission to add users (admin or owner)
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can add users to tenant",
		})
	}

	// Add user to tenant
	if err := h.userService.AddUserToTenant(ctx, userID, tenantID, roleID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "user added to tenant successfully",
	})
}

// ChangeUserRoleRequest defines request body for changing user role
type ChangeUserRoleRequest struct {
	RoleID string `json:"role_id"`
}

// ChangeUserRole changes a user's role in the tenant
func (h *RBACUserHandler) ChangeUserRole(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

	userIDStr := c.Params("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid user ID format",
		})
	}

	var req ChangeUserRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role ID format",
		})
	}

	ctx := c.Context()

	// Check if requestor has permission to change roles
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can change user roles",
		})
	}

	// Change user role
	if err := h.userService.ChangeUserRole(ctx, userID, tenantID, roleID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "user role changed successfully",
	})
}

// RemoveUserFromTenantRequest is unused but defined for consistency
type RemoveUserFromTenantRequest struct{}

// RemoveUserFromTenant removes a user from the tenant
func (h *RBACUserHandler) RemoveUserFromTenant(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	requestorID := c.Locals("userID").(uuid.UUID)

	userIDStr := c.Params("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid user ID format",
		})
	}

	ctx := c.Context()

	// Check if requestor has permission to remove users
	requestorLevel, err := h.userService.GetUserLevel(ctx, requestorID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}

	if requestorLevel < 9 { // 9 = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can remove users from tenant",
		})
	}

	// Prevent removing admin user
	userRole, err := h.userService.GetUserRole(ctx, userID, tenantID)
	if err == nil && userRole.Level == 9 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "cannot remove admin users",
		})
	}

	// Remove user from tenant
	if err := h.userService.RemoveUserFromTenant(ctx, userID, tenantID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "user removed from tenant successfully",
	})
}

// GetUserPermissionsResponse contains user's permissions
type GetUserPermissionsResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Role        string    `json:"role"`
	Level       int       `json:"level"`
	Permissions []string  `json:"permissions"`
}

// GetUserPermissions retrieves all permissions for a user in the tenant
func (h *RBACUserHandler) GetUserPermissions(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)

	userIDStr := c.Params("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid user ID format",
		})
	}

	ctx := c.Context()

	// Get user role
	role, err := h.userService.GetUserRole(ctx, userID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found in tenant",
		})
	}

	// Get user permissions
	permissions, err := h.userService.GetUserPermissions(ctx, userID, tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve permissions",
		})
	}

	return c.JSON(GetUserPermissionsResponse{
		UserID:      userID,
		TenantID:    tenantID,
		Role:        role.Name,
		Level:       int(role.Level),
		Permissions: permissions,
	})
}

// GetTenantUsersCountResponse contains user count statistics
type GetTenantUsersCountResponse struct {
	Total      int64 `json:"total"`
	ByAdmins   int64 `json:"by_admins"`
	ByManagers int64 `json:"by_managers"`
	ByAnalysts int64 `json:"by_analysts"`
	ByViewers  int64 `json:"by_viewers"`
}

// GetTenantUserStats retrieves statistics about users in the tenant
func (h *RBACUserHandler) GetTenantUserStats(c *fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)

	ctx := c.Context()

	// Get total user count
	var counts struct {
		Total      int64
		ByAdmins   int64
		ByManagers int64
		ByAnalysts int64
		ByViewers  int64
	}

	// Note: This is a simplified version. A real implementation would query the database
	// For now, we'll just get the total and note that role-based counts would need DB queries

	_, total, err := h.userService.GetTenantUsers(ctx, tenantID, 1, 0)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve user statistics",
		})
	}

	counts.Total = total

	return c.JSON(GetTenantUsersCountResponse{
		Total:      counts.Total,
		ByAdmins:   0, // Would be populated from database query
		ByManagers: 0,
		ByAnalysts: 0,
		ByViewers:  0,
	})
}
