// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
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

// GetRole retrieves a role visible to tenantID: one of the tenant's own roles,
// or a predefined role shared by every tenant. A role owned by another tenant
// answers ErrNotFound, exactly like a role that does not exist, so the caller
// learns nothing about other tenants (GHSA-6m8w-33j5-ff68).
func (rs *RoleService) GetRole(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.RoleEnhanced, error) {
	var role domain.RoleEnhanced
	// No Preload("Permissions"): RoleEnhanced has no such association (see
	// ListRoles), and preloading it errored every call.
	err := rs.db.WithContext(ctx).
		Where("id = ? AND (tenant_id = ? OR is_predefined = true) AND deleted_at IS NULL", roleID, tenantID).
		First(&role).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: role not found", domain.ErrNotFound)
		}
		return nil, err
	}

	return &role, nil
}

// getOwnedRole retrieves a role tenantID may change: a custom role of that
// tenant. Predefined roles are shared by every tenant, so changing one from a
// tenant would change it for all of them; that answers ErrForbidden.
func (rs *RoleService) getOwnedRole(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.RoleEnhanced, error) {
	role, err := rs.GetRole(ctx, tenantID, roleID)
	if err != nil {
		return nil, err
	}
	if role.IsPredefined {
		return nil, fmt.Errorf("%w: predefined roles cannot be changed", domain.ErrForbidden)
	}
	return role, nil
}

// GetRoleByName retrieves a role by name within a tenant
func (rs *RoleService) GetRoleByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.RoleEnhanced, error) {
	var role domain.RoleEnhanced
	// No Preload("Permissions") — RoleEnhanced has no such association; preloading it
	// errored every call, so InitializeDefaultRoles kept re-creating roles and the RBAC
	// seed's adminRole lookup always came back nil (user_tenants never got seeded).
	err := rs.db.WithContext(ctx).
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

// UpdateRole updates the name, description and active flag of one of
// tenantID's custom roles.
func (rs *RoleService) UpdateRole(ctx context.Context, tenantID uuid.UUID, role *domain.RoleEnhanced) error {
	if role.ID == uuid.Nil {
		return fmt.Errorf("%w: role ID is required", domain.ErrValidation)
	}

	existing, err := rs.getOwnedRole(ctx, tenantID, role.ID)
	if err != nil {
		return err
	}

	// Deactivating a role does not take its rights away from the people who
	// hold it, so it is refused while anyone does, as DeleteRole refuses.
	if existing.IsActive && !role.IsActive {
		if err := rs.ensureRoleUnassigned(ctx, role.ID); err != nil {
			return err
		}
	}

	// A map, not the struct: Updates(struct) skips zero values, so is_active
	// could never be set back to false.
	return rs.db.WithContext(ctx).
		Model(&domain.RoleEnhanced{}).
		Where("id = ? AND tenant_id = ?", role.ID, tenantID).
		Updates(map[string]interface{}{
			"name":        role.Name,
			"description": role.Description,
			"is_active":   role.IsActive,
		}).Error
}

// DeleteRole deactivates one of tenantID's custom roles.
func (rs *RoleService) DeleteRole(ctx context.Context, tenantID, roleID uuid.UUID) error {
	if _, err := rs.getOwnedRole(ctx, tenantID, roleID); err != nil {
		return err
	}

	if err := rs.ensureRoleUnassigned(ctx, roleID); err != nil {
		return err
	}

	return rs.db.WithContext(ctx).
		Model(&domain.RoleEnhanced{}).
		Where("id = ? AND tenant_id = ?", roleID, tenantID).
		Update("is_active", false).Error
}

// ensureRoleUnassigned answers ErrConflict while any user holds the role. By
// role_id alone, on purpose: only zero versus non-zero is used, and a stray
// row anywhere must still block.
func (rs *RoleService) ensureRoleUnassigned(ctx context.Context, roleID uuid.UUID) error {
	var count int64
	if err := rs.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("role_id = ?", roleID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: role is held by %d users", domain.ErrConflict, count)
	}
	return nil
}

// GetRolePermissions retrieves all permissions of a role visible to tenantID.
func (rs *RoleService) GetRolePermissions(ctx context.Context, tenantID, roleID uuid.UUID) ([]domain.PermissionDB, error) {
	if _, err := rs.GetRole(ctx, tenantID, roleID); err != nil {
		return nil, err
	}

	var permissions []domain.PermissionDB
	err := rs.db.WithContext(ctx).
		Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Order("resource, action").
		Find(&permissions).Error

	return permissions, err
}

// AssignPermissionToRole assigns a permission to one of tenantID's custom roles.
func (rs *RoleService) AssignPermissionToRole(ctx context.Context, tenantID, roleID, permissionID uuid.UUID) error {
	if _, err := rs.getOwnedRole(ctx, tenantID, roleID); err != nil {
		return err
	}
	return rs.assignPermission(ctx, roleID, permissionID)
}

// assignPermission links a permission to a role with no ownership check. Only
// the predefined-role seed calls it directly.
func (rs *RoleService) assignPermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
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

// RemovePermissionFromRole removes a permission from one of tenantID's custom roles.
func (rs *RoleService) RemovePermissionFromRole(ctx context.Context, tenantID, roleID, permissionID uuid.UUID) error {
	if _, err := rs.getOwnedRole(ctx, tenantID, roleID); err != nil {
		return err
	}
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
			if err := rs.assignPermission(ctx, roleID, perm.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetRoleHierarchy returns a role visible to tenantID
func (rs *RoleService) GetRoleHierarchy(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.RoleEnhanced, error) {
	return rs.GetRole(ctx, tenantID, roleID)
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

	// Fetch paginated results. (No Preload("Permissions"): RoleEnhanced has no such
	// GORM association — role→permission goes through the role_permissions join table,
	// queried separately by GetRolePermissions. Preloading a non-existent relation
	// errored the whole query, which was the 500 on GET /rbac/roles.)
	err := rs.db.WithContext(ctx).
		Where("(tenant_id = ? OR is_predefined = true) AND deleted_at IS NULL", tenantID).
		Order("level DESC, name ASC").
		Limit(limit).
		Offset(offset).
		Find(&roles).Error

	return roles, total, err
}

// IsUserInRole checks if a user holds a specific role in tenantID
func (rs *RoleService) IsUserInRole(ctx context.Context, tenantID, userID, roleID uuid.UUID) (bool, error) {
	var count int64
	err := rs.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("tenant_id = ? AND user_id = ? AND role_id = ?", tenantID, userID, roleID).
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
