package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// RBACTenantHandler manages tenant RBAC operations
type RBACTenantHandler struct {
	tenantService services.TenantService
	userService   services.UserService
}

// NewRBACTenantHandler creates a new RBAC tenant handler
func NewRBACTenantHandler(
	tenantService services.TenantService,
	userService services.UserService,
) RBACTenantHandler {
	return &RBACTenantHandler{
		tenantService: tenantService,
		userService:   userService,
	}
}

// ListTenantsResponse contains paginated tenant list
type ListTenantsResponse struct {
	Tenants    []domain.Tenant json:"tenants"
	Total      int           json:"total"
	Limit      int             json:"limit"
	Offset     int             json:"offset"
	HasMore    bool            json:"has_more"
	TotalPages int             json:"total_pages"
}

// ListTenants retrieves all tenants for a user
func (h RBACTenantHandler) ListTenants(c fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	limit := 
	offset := 

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed >  {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >=  {
			offset = parsed
		}
	}

	ctx := c.Context()

	// Get all tenants for user
	userTenants, err := h.userService.GetUserTenants(ctx, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve tenants",
		})
	}

	// Extract tenant IDs and fetch full tenant details
	tenants := make([]domain.Tenant, )
	total := int(len(userTenants))

	// Apply pagination
	if int(offset) >= total {
		return c.JSON(ListTenantsResponse{
			Tenants:    tenants,
			Total:      total,
			Limit:      limit,
			Offset:     offset,
			HasMore:    false,
			TotalPages: ,
		})
	}

	endIdx := offset + limit
	if int(endIdx) > total {
		endIdx = int(total)
	}

	for i := offset; i < endIdx; i++ {
		tenant, err := h.tenantService.GetTenant(ctx, userTenants[i].TenantID)
		if err == nil {
			tenants = append(tenants, tenant)
		}
	}

	totalPages := (int(total) + limit - ) / limit

	return c.JSON(ListTenantsResponse{
		Tenants:    tenants,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		HasMore:    int(offset+limit) < total,
		TotalPages: totalPages,
	})
}

// GetTenantResponse contains tenant details with statistics
type GetTenantResponse struct {
	Tenant    domain.Tenant json:"tenant"
	UserCount int         json:"user_count"
	RoleCount int         json:"role_count"
	RiskCount int         json:"risk_count"
	CreatedAt string        json:"created_at"
	UpdatedAt string        json:"updated_at"
}

// GetTenant retrieves a specific tenant
func (h RBACTenantHandler) GetTenant(c fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)

	ctx := c.Context()

	// Verify requesting user is in this tenant
	userID := c.Locals("userID").(uuid.UUID)
	if !h.userService.ValidateUserInTenant(ctx, userID, tenantID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "unauthorized access to tenant",
		})
	}

	// Get tenant
	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "tenant not found",
		})
	}

	// Get statistics
	stats, err := h.tenantService.GetTenantStats(ctx, tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve tenant statistics",
		})
	}

	userCount := int()
	roleCount := int()
	riskCount := int()
	if uc, ok := stats["user_count"].(int); ok {
		userCount = uc
	}
	if rc, ok := stats["role_count"].(int); ok {
		roleCount = rc
	}
	if rk, ok := stats["risk_count"].(int); ok {
		riskCount = rk
	}

	return c.JSON(GetTenantResponse{
		Tenant:    tenant,
		UserCount: userCount,
		RoleCount: roleCount,
		RiskCount: riskCount,
	})
}

// CreateTenantRequest defines request body for creating a tenant
type CreateTenantRequest struct {
	Name     string                 json:"name"
	Slug     string                 json:"slug"
	Metadata map[string]interface{} json:"metadata"
}

// CreateTenant creates a new tenant (admin only)
func (h RBACTenantHandler) CreateTenant(c fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	var req CreateTenantRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "tenant name is required",
		})
	}

	ctx := c.Context()

	// Create tenant
	var metadata []byte

	tenant := &domain.Tenant{
		ID:       uuid.New(),
		Name:     req.Name,
		Slug:     req.Slug,
		OwnerID:  userID,
		IsActive: true,
		Metadata: metadata,
	}

	if err := h.tenantService.CreateTenant(ctx, tenant); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(tenant)
}

// UpdateTenantRequest defines request body for updating a tenant
type UpdateTenantRequest struct {
	Name     string                 json:"name"
	Slug     string                 json:"slug"
	IsActive bool                   json:"is_active"
	Metadata map[string]interface{} json:"metadata"
}

// UpdateTenant updates an existing tenant
func (h RBACTenantHandler) UpdateTenant(c fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	userID := c.Locals("userID").(uuid.UUID)

	var req UpdateTenantRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx := c.Context()

	// Get tenant to verify ownership
	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "tenant not found",
		})
	}

	// Check if user is owner or admin
	if tenant.OwnerID != userID {
		// Check if user is admin in tenant
		level, err := h.userService.GetUserLevel(ctx, userID, tenantID)
		if err != nil || level <  { //  = Admin
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "only tenant owner or admin can update tenant",
			})
		}
	}

	// Update fields
	if req.Name != "" {
		tenant.Name = req.Name
	}
	if req.Slug != "" {
		tenant.Slug = req.Slug
	}
	tenant.IsActive = req.IsActive

	// Update tenant
	if err := h.tenantService.UpdateTenant(ctx, tenant); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(tenant)
}

// DeleteTenant deletes a tenant (owner only)
func (h RBACTenantHandler) DeleteTenant(c fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	userID := c.Locals("userID").(uuid.UUID)

	ctx := c.Context()

	// Get tenant to verify ownership
	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "tenant not found",
		})
	}

	// Only owner can delete
	if tenant.OwnerID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only tenant owner can delete tenant",
		})
	}

	// Delete tenant
	if err := h.tenantService.DeleteTenant(ctx, tenantID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "tenant deleted successfully",
	})
}

// GetTenantUsersResponse contains tenant user list
type GetTenantUsersResponse struct {
	Users      []domain.UserTenant json:"users"
	Total      int               json:"total"
	Limit      int                 json:"limit"
	Offset     int                 json:"offset"
	HasMore    bool                json:"has_more"
	TotalPages int                 json:"total_pages"
}

// GetTenantUsers retrieves all users in a tenant
func (h RBACTenantHandler) GetTenantUsers(c fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)
	userID := c.Locals("userID").(uuid.UUID)

	limit := 
	offset := 

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed >  {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >=  {
			offset = parsed
		}
	}

	ctx := c.Context()

	// Check if requestor is admin in tenant
	level, err := h.userService.GetUserLevel(ctx, userID, tenantID)
	if err != nil || level <  { //  = Admin
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "only admins can view tenant users",
		})
	}

	// Get users
	users, total, err := h.userService.GetTenantUsers(ctx, tenantID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve tenant users",
		})
	}

	totalPages := (int(total) + limit - ) / limit

	return c.JSON(GetTenantUsersResponse{
		Users:      users,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		HasMore:    int(offset+limit) < total,
		TotalPages: totalPages,
	})
}

// GetTenantStatsResponse contains tenant statistics
type GetTenantStatsResponse struct {
	TenantID        uuid.UUID json:"tenant_id"
	Name            string    json:"name"
	UserCount       int     json:"user_count"
	RoleCount       int     json:"role_count"
	RiskCount       int     json:"risk_count"
	MitigationCount int     json:"mitigation_count"
}

// GetTenantStats retrieves statistics for a tenant
func (h RBACTenantHandler) GetTenantStats(c fiber.Ctx) error {
	tenantID := c.Locals("tenantID").(uuid.UUID)

	ctx := c.Context()

	// Get tenant
	tenant, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "tenant not found",
		})
	}

	// Get statistics
	stats, err := h.tenantService.GetTenantStats(ctx, tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve tenant statistics",
		})
	}

	userCount := int()
	roleCount := int()
	riskCount := int()
	mitigationCount := int()
	if uc, ok := stats["user_count"].(int); ok {
		userCount = uc
	}
	if rc, ok := stats["role_count"].(int); ok {
		roleCount = rc
	}
	if rk, ok := stats["risk_count"].(int); ok {
		riskCount = rk
	}
	if mc, ok := stats["mitigation_count"].(int); ok {
		mitigationCount = mc
	}

	return c.JSON(GetTenantStatsResponse{
		TenantID:        tenantID,
		Name:            tenant.Name,
		UserCount:       userCount,
		RoleCount:       roleCount,
		RiskCount:       riskCount,
		MitigationCount: mitigationCount,
	})
}
