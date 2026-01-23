package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// RoleService manages role operations and permissions
type RoleService struct {
	db *gorm.DB
}

// NewRoleService creates a new role service
func NewRoleService(db *gorm.DB) *RoleService {
	return &RoleService{db: db}
}

// CreateRole creates a new role with validation
func (rs *RoleService) CreateRole(ctx context.Context, role *domain.RoleEnhanced) error {
	if role.ID == uuid.Nil {
		role.ID = uuid.New()
	}

	if role.Name == "" {
		return fmt.Errorf("role name is required")
	}

	if !domain.ValidateRoleLevel(role.Level) {
		return fmt.Errorf("invalid role level: %d", role.Level)
	}

	if role.TenantID == uuid.Nil && !role.IsPredefined {
		return fmt.Errorf("tenant_id is required for non-predefined roles")
	}

	return rs.db.WithContext(ctx).Create(role).Error
}

// GetRole retrieves a role by ID
func (rs *RoleService) GetRole(ctx context.Context, roleID uuid.UUID) (*domain.RoleEnhanced, error) {
	var role domain.RoleEnhanced
	err := rs.db.WithContext(ctx).
		Preload("Permissions").
		Where("id = ? AND deleted_at IS NULL", roleID).
		First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("role not found")
		}
		return nil, err
	}

	return &role, nil
}

// GetRoleByName retrieves a role by name within a tenant
func (rs *RoleService) GetRoleByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.RoleEnhanced, error) {
	var role domain.RoleEnhanced
	err := rs.db.WithContext(ctx).
		Preload("Permissions").
		Where("(tenant_id = ? OR is_predefined = true) AND name = ? AND deleted_at IS NULL", tenantID, name).
		First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("role '%s' not found", name)
		}
		return nil, err
	}

	return &role, nil
}

// GetRolesByTenant retrieves all roles for a tenant
func (rs *RoleService) GetRolesByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.RoleEnhanced, error) {
	var roles []domain.RoleEnhanced
	err := rs.db.WithContext(ctx).
		Preload("Permissions").
		Where("(tenant_id = ? OR is_predefined = true) AND is_active = true AND deleted_at IS NULL", tenantID).
		Order("level DESC, name ASC").
		Find(&roles).Error

	return roles, err
}

// UpdateRole updates an existing role
func (rs *RoleService) UpdateRole(ctx context.Context, role *domain.RoleEnhanced) error {
	if role.ID == uuid.Nil {
		return fmt.Errorf("role ID is required")
	}

	// Cannot update predefined roles' hierarchy
	var existing domain.RoleEnhanced
	if err := rs.db.WithContext(ctx).First(&existing, "id = ?", role.ID).Error; err != nil {
		return err
	}

	if existing.IsPredefined && existing.Level != role.Level {
		return fmt.Errorf("cannot change level of predefined role")
	}

	return rs.db.WithContext(ctx).
		Model(role).
		Updates(role).Error
}

// DeleteRole soft-deletes a role
func (rs *RoleService) DeleteRole(ctx context.Context, roleID uuid.UUID) error {
	// Check if role is predefined
	var role domain.RoleEnhanced
	if err := rs.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		return err
	}

	if role.IsPredefined {
		return fmt.Errorf("cannot delete predefined role")
	}

	// Check if any users have this role
	var count int64
	if err := rs.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("role_id = ?", roleID).
		Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("cannot delete role with %d active users", count)
	}

	return rs.db.WithContext(ctx).
		Model(&domain.RoleEnhanced{}).
		Where("id = ?", roleID).
		Update("is_active", false).Error
}

// GetRolePermissions retrieves all permissions for a role
func (rs *RoleService) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.PermissionDB, error) {
	var permissions []domain.PermissionDB
	err := rs.db.WithContext(ctx).
		Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Order("resource, action").
		Find(&permissions).Error

	return permissions, err
}

// AssignPermissionToRole assigns a permission to a role
func (rs *RoleService) AssignPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	// Check if already assigned
	var count int64
	if err := rs.db.WithContext(ctx).
		Model(&domain.RolePermission{}).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil // Already assigned
	}

	rolePermission := &domain.RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}

	return rs.db.WithContext(ctx).Create(rolePermission).Error
}

// RemovePermissionFromRole removes a permission from a role
func (rs *RoleService) RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return rs.db.WithContext(ctx).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Delete(&domain.RolePermission{}).Error
}

// InitializeDefaultRoles creates the default predefined roles
func (rs *RoleService) InitializeDefaultRoles(ctx context.Context) error {
	predefinedRoles := []struct {
		Name        string
		Level       domain.RoleLevel
		Description string
	}{
		{"Admin", domain.RoleLevelAdmin, "Full system access and administration"},
		{"Manager", domain.RoleLevelManager, "Resource management and team oversight"},
		{"Analyst", domain.RoleLevelAnalyst, "Risk analysis and mitigation creation"},
		{"Viewer", domain.RoleLevelViewer, "Read-only access to dashboards and reports"},
	}

	for _, predefined := range predefinedRoles {
		// Check if already exists
		existing, _ := rs.GetRoleByName(ctx, uuid.Nil, predefined.Name)
		if existing != nil {
			continue
		}

		role := &domain.RoleEnhanced{
			ID:           uuid.New(),
			Name:         predefined.Name,
			Level:        predefined.Level,
			Description:  predefined.Description,
			IsPredefined: true,
			IsActive:     true,
		}

		if err := rs.CreateRole(ctx, role); err != nil {
			return fmt.Errorf("failed to create predefined role %s: %w", predefined.Name, err)
		}

		// Assign permissions to predefined roles
		if err := rs.assignPermissionsToRole(ctx, role.ID, predefined.Level); err != nil {
			return fmt.Errorf("failed to assign permissions to role %s: %w", predefined.Name, err)
		}
	}

	return nil
}

// assignPermissionsToRole assigns appropriate permissions based on role level
func (rs *RoleService) assignPermissionsToRole(ctx context.Context, roleID uuid.UUID, level domain.RoleLevel) error {
	// Get all permissions
	var allPermissions []domain.PermissionDB
	if err := rs.db.WithContext(ctx).Find(&allPermissions).Error; err != nil {
		return err
	}

	// Determine which permissions to assign based on role level
	for _, perm := range allPermissions {
		shouldAssign := false

		switch level {
		case domain.RoleLevelAdmin:
			shouldAssign = true // Admin gets all permissions

		case domain.RoleLevelManager:
			// Manager can: read everything, create/update risks/mitigations, cannot manage users
			if perm.Action == string(domain.PermissionRead) ||
				perm.Action == string(domain.PermissionExport) {
				shouldAssign = true
			}
			if (perm.Resource == "risk" || perm.Resource == "mitigation" || perm.Resource == "asset") &&
				(perm.Action == string(domain.PermissionCreate) || perm.Action == string(domain.PermissionUpdate)) {
				shouldAssign = true
			}

		case domain.RoleLevelAnalyst:
			// Analyst can: read, create/update risks/mitigations, cannot delete
			if perm.Action == string(domain.PermissionRead) ||
				perm.Action == string(domain.PermissionExport) {
				shouldAssign = true
			}
			if (perm.Resource == "risk" || perm.Resource == "mitigation" || perm.Resource == "asset") &&
				(perm.Action == string(domain.PermissionCreate) || perm.Action == string(domain.PermissionUpdate)) {
				shouldAssign = true
			}

		case domain.RoleLevelViewer:
			// Viewer can only read
			if perm.Action == string(domain.PermissionRead) {
				shouldAssign = true
			}
		}

		if shouldAssign {
			if err := rs.AssignPermissionToRole(ctx, roleID, perm.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetRoleHierarchy returns the role with all related data
func (rs *RoleService) GetRoleHierarchy(ctx context.Context, roleID uuid.UUID) (*domain.RoleEnhanced, error) {
	role, err := rs.GetRole(ctx, roleID)
	if err != nil {
		return nil, err
	}

	// Permissions are already preloaded
	return role, nil
}

// ListRoles retrieves all roles for a tenant with pagination
func (rs *RoleService) ListRoles(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]domain.RoleEnhanced, int64, error) {
	var roles []domain.RoleEnhanced
	var total int64

	// Count total
	if err := rs.db.WithContext(ctx).
		Model(&domain.RoleEnhanced{}).
		Where("(tenant_id = ? OR is_predefined = true) AND deleted_at IS NULL", tenantID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	err := rs.db.WithContext(ctx).
		Preload("Permissions").
		Where("(tenant_id = ? OR is_predefined = true) AND deleted_at IS NULL", tenantID).
		Order("level DESC, name ASC").
		Limit(limit).
		Offset(offset).
		Find(&roles).Error

	return roles, total, err
}

// IsUserInRole checks if a user has a specific role
func (rs *RoleService) IsUserInRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) (bool, error) {
	var count int64
	err := rs.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Count(&count).Error

	return count > 0, err
}

// GetRolesByLevel retrieves all roles at or below a specific level
func (rs *RoleService) GetRolesByLevel(ctx context.Context, tenantID uuid.UUID, maxLevel domain.RoleLevel) ([]domain.RoleEnhanced, error) {
	var roles []domain.RoleEnhanced
	err := rs.db.WithContext(ctx).
		Where("(tenant_id = ? OR is_predefined = true) AND level <= ? AND is_active = true AND deleted_at IS NULL", tenantID, maxLevel).
		Order("level DESC, name ASC").
		Find(&roles).Error

	return roles, err
}
