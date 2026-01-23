package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// UserService manages user operations and multi-tenant relationships
type UserService struct {
	db *gorm.DB
}

// NewUserService creates a new user service
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// GetUserTenants retrieves all tenants for a user
func (us *UserService) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.UserTenant, error) {
	var userTenants []domain.UserTenant
	err := us.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Role").
		Order("created_at DESC").
		Find(&userTenants).Error

	return userTenants, err
}

// GetUserTenantsByRole retrieves user-tenant relationships filtered by role level
func (us *UserService) GetUserTenantsByRole(ctx context.Context, userID uuid.UUID, minLevel domain.RoleLevel) ([]domain.UserTenant, error) {
	var userTenants []domain.UserTenant
	err := us.db.WithContext(ctx).
		Joins("JOIN roles ON user_tenants.role_id = roles.id").
		Where("user_tenants.user_id = ? AND roles.level >= ?", userID, minLevel).
		Preload("Role").
		Order("created_at DESC").
		Find(&userTenants).Error

	return userTenants, err
}

// GetUserInTenant retrieves a specific user-tenant relationship
func (us *UserService) GetUserInTenant(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) (*domain.UserTenant, error) {
	var userTenant domain.UserTenant
	err := us.db.WithContext(ctx).
		Preload("Role").
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&userTenant).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not in tenant")
		}
		return nil, err
	}

	return &userTenant, nil
}

// GetUserRole retrieves the user's role within a specific tenant
func (us *UserService) GetUserRole(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) (*domain.RoleEnhanced, error) {
	var role domain.RoleEnhanced
	err := us.db.WithContext(ctx).
		Joins("JOIN user_tenants ON roles.id = user_tenants.role_id").
		Where("user_tenants.user_id = ? AND user_tenants.tenant_id = ?", userID, tenantID).
		First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user role not found in tenant")
		}
		return nil, err
	}

	return &role, nil
}

// GetUserLevel retrieves the user's role level within a specific tenant
func (us *UserService) GetUserLevel(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) (domain.RoleLevel, error) {
	role, err := us.GetUserRole(ctx, userID, tenantID)
	if err != nil {
		return 0, err
	}
	return role.Level, nil
}

// ValidateUserInTenant checks if a user belongs to a specific tenant
func (us *UserService) ValidateUserInTenant(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) bool {
	var count int64
	us.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Count(&count)

	return count > 0
}

// ValidateUserPermission checks if a user has a specific permission in a tenant
func (us *UserService) ValidateUserPermission(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, resource string, action string) (bool, error) {
	// Get user's role
	role, err := us.GetUserRole(ctx, userID, tenantID)
	if err != nil {
		return false, err
	}

	// Admins have all permissions
	if role.Level == domain.RoleLevelAdmin {
		return true, nil
	}

	// Check if user has the permission
	var count int64
	err = us.db.WithContext(ctx).
		Model(&domain.PermissionDB{}).
		Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ? AND permissions.resource = ? AND permissions.action = ?", role.ID, resource, action).
		Count(&count).Error

	return count > 0, err
}

// GetTenantUsers retrieves all users in a tenant with pagination
func (us *UserService) GetTenantUsers(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]domain.UserTenant, int64, error) {
	var userTenants []domain.UserTenant
	var total int64

	// Count total
	if err := us.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("tenant_id = ?", tenantID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	err := us.db.WithContext(ctx).
		Preload("Role").
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&userTenants).Error

	return userTenants, total, err
}

// ChangeUserRole changes a user's role in a tenant
func (us *UserService) ChangeUserRole(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, newRoleID uuid.UUID) error {
	// Validate new role exists
	var role domain.RoleEnhanced
	if err := us.db.WithContext(ctx).
		Where("id = ? AND (tenant_id = ? OR is_predefined = true)", newRoleID, tenantID).
		First(&role).Error; err != nil {
		return fmt.Errorf("invalid role for tenant")
	}

	// Update user-tenant relationship
	return us.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Update("role_id", newRoleID).Error
}

// AddUserToTenant adds a user to a tenant with a specific role
func (us *UserService) AddUserToTenant(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, roleID uuid.UUID) error {
	// Check if user is already in tenant
	existing, _ := us.GetUserInTenant(ctx, userID, tenantID)
	if existing != nil {
		return fmt.Errorf("user already in tenant")
	}

	// Validate role exists in tenant
	var role domain.RoleEnhanced
	if err := us.db.WithContext(ctx).
		Where("id = ? AND (tenant_id = ? OR is_predefined = true)", roleID, tenantID).
		First(&role).Error; err != nil {
		return fmt.Errorf("invalid role for tenant")
	}

	userTenant := &domain.UserTenant{
		UserID:   userID,
		TenantID: tenantID,
		RoleID:   roleID,
	}

	return us.db.WithContext(ctx).Create(userTenant).Error
}

// RemoveUserFromTenant removes a user from a tenant
func (us *UserService) RemoveUserFromTenant(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) error {
	return us.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Delete(&domain.UserTenant{}).Error
}

// GetUserPermissions retrieves all permissions for a user in a tenant
func (us *UserService) GetUserPermissions(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) ([]string, error) {
	// Get user's role
	role, err := us.GetUserRole(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}

	// Admin has all permissions
	if role.Level == domain.RoleLevelAdmin {
		return []string{"*:*"}, nil
	}

	// Get role permissions
	var permissions []struct {
		Resource string
		Action   string
	}

	err = us.db.WithContext(ctx).
		Model(&domain.PermissionDB{}).
		Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", role.ID).
		Select("permissions.resource, permissions.action").
		Scan(&permissions).Error

	if err != nil {
		return nil, err
	}

	// Build permission strings
	permissionStrings := make([]string, len(permissions))
	for i, perm := range permissions {
		permissionStrings[i] = perm.Resource + ":" + perm.Action
	}

	return permissionStrings, nil
}

// CheckUserAccess checks if user can access a specific resource
func (us *UserService) CheckUserAccess(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, resource string, action string) (bool, error) {
	// First validate user is in tenant
	if !us.ValidateUserInTenant(ctx, userID, tenantID) {
		return false, nil
	}

	// Check permission
	return us.ValidateUserPermission(ctx, userID, tenantID, resource, action)
}

// GetUserTenantCount retrieves the number of tenants a user belongs to
func (us *UserService) GetUserTenantCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := us.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("user_id = ?", userID).
		Count(&count).Error

	return count, err
}

// GetHighestUserRole retrieves the highest-level role a user has across all tenants
func (us *UserService) GetHighestUserRole(ctx context.Context, userID uuid.UUID) (domain.RoleLevel, error) {
	var level int

	err := us.db.WithContext(ctx).
		Model(&domain.RoleEnhanced{}).
		Joins("JOIN user_tenants ON roles.id = user_tenants.role_id").
		Where("user_tenants.user_id = ?", userID).
		Order("roles.level DESC").
		Limit(1).
		Pluck("roles.level", &level).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, fmt.Errorf("user has no roles")
		}
		return 0, err
	}

	return domain.RoleLevel(level), nil
}

// ListUsersByRole retrieves all users with a specific role in a tenant
func (us *UserService) ListUsersByRole(ctx context.Context, tenantID uuid.UUID, roleID uuid.UUID, limit int, offset int) ([]domain.UserTenant, int64, error) {
	var userTenants []domain.UserTenant
	var total int64

	// Count total
	if err := us.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("tenant_id = ? AND role_id = ?", tenantID, roleID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	err := us.db.WithContext(ctx).
		Preload("Role").
		Where("tenant_id = ? AND role_id = ?", tenantID, roleID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&userTenants).Error

	return userTenants, total, err
}
