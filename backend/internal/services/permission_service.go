package services

import (
	"fmt"
	"sync"

	"github.com/opendefender/openrisk/internal/core/domain"
)

// PermissionService handles permission management and checks
type PermissionService struct {
	matrices map[string]domain.PermissionMatrix
	mu       sync.RWMutex
}

// NewPermissionService creates a new permission service
func NewPermissionService() PermissionService {
	return &PermissionService{
		matrices: make(map[string]domain.PermissionMatrix),
	}
}

// SetRolePermissions sets the permissions for a role
func (ps PermissionService) SetRolePermissions(roleID string, permissions []domain.Permission) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	matrix := &domain.PermissionMatrix{
		ID:          fmt.Sprintf("role_%s", roleID),
		EntityType:  "role",
		EntityID:    roleID,
		Permissions: make([]domain.Permission, , len(permissions)),
	}

	for _, p := range permissions {
		if err := matrix.AddPermission(p); err != nil {
			return fmt.Errorf("failed to add permission: %w", err)
		}
	}

	ps.matrices[matrix.ID] = matrix
	return nil
}

// SetUserPermissions sets custom permissions for a user (overrides role permissions)
func (ps PermissionService) SetUserPermissions(userID string, permissions []domain.Permission) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	matrix := &domain.PermissionMatrix{
		ID:          fmt.Sprintf("user_%s", userID),
		EntityType:  "user",
		EntityID:    userID,
		Permissions: make([]domain.Permission, , len(permissions)),
	}

	for _, p := range permissions {
		if err := matrix.AddPermission(p); err != nil {
			return fmt.Errorf("failed to add permission: %w", err)
		}
	}

	ps.matrices[matrix.ID] = matrix
	return nil
}

// GetUserPermissions gets all permissions for a user (role + custom)
func (ps PermissionService) GetUserPermissions(userID string, roleID string) []domain.Permission {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	permissions := make(map[string]domain.Permission)

	// First, add role permissions
	roleMatrix, exists := ps.matrices[fmt.Sprintf("role_%s", roleID)]
	if exists {
		for _, p := range roleMatrix.Permissions {
			permissions[p.String()] = p
		}
	}

	// Then, add/override with user-specific permissions
	userMatrix, exists := ps.matrices[fmt.Sprintf("user_%s", userID)]
	if exists {
		for _, p := range userMatrix.Permissions {
			permissions[p.String()] = p
		}
	}

	// Convert map to slice
	result := make([]domain.Permission, , len(permissions))
	for _, p := range permissions {
		result = append(result, p)
	}

	return result
}

// CheckPermission checks if a user has a specific permission
func (ps PermissionService) CheckPermission(userID string, roleID string, required domain.Permission) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Check user-specific permissions first
	userMatrixID := fmt.Sprintf("user_%s", userID)
	if userMatrix, exists := ps.matrices[userMatrixID]; exists {
		if userMatrix.HasPermission(required) {
			return true
		}
	}

	// Check role permissions
	roleMatrixID := fmt.Sprintf("role_%s", roleID)
	if roleMatrix, exists := ps.matrices[roleMatrixID]; exists {
		if roleMatrix.HasPermission(required) {
			return true
		}
	}

	return false
}

// CheckPermissionMultiple checks if a user has any of multiple permissions
func (ps PermissionService) CheckPermissionMultiple(userID string, roleID string, required []domain.Permission) bool {
	for _, perm := range required {
		if ps.CheckPermission(userID, roleID, perm) {
			return true
		}
	}
	return false
}

// CheckPermissionAll checks if a user has all of multiple permissions
func (ps PermissionService) CheckPermissionAll(userID string, roleID string, required []domain.Permission) bool {
	for _, perm := range required {
		if !ps.CheckPermission(userID, roleID, perm) {
			return false
		}
	}
	return true
}

// AddPermissionToRole adds a permission to a role
func (ps PermissionService) AddPermissionToRole(roleID string, permission domain.Permission) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	matrixID := fmt.Sprintf("role_%s", roleID)
	matrix, exists := ps.matrices[matrixID]
	if !exists {
		matrix = &domain.PermissionMatrix{
			ID:          matrixID,
			EntityType:  "role",
			EntityID:    roleID,
			Permissions: []domain.Permission{},
		}
		ps.matrices[matrixID] = matrix
	}

	return matrix.AddPermission(permission)
}

// RemovePermissionFromRole removes a permission from a role
func (ps PermissionService) RemovePermissionFromRole(roleID string, permission domain.Permission) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	matrixID := fmt.Sprintf("role_%s", roleID)
	matrix, exists := ps.matrices[matrixID]
	if !exists {
		return fmt.Errorf("role not found: %s", roleID)
	}

	return matrix.RemovePermission(permission)
}

// GetRolePermissions gets all permissions for a role
func (ps PermissionService) GetRolePermissions(roleID string) []domain.Permission {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	matrixID := fmt.Sprintf("role_%s", roleID)
	if matrix, exists := ps.matrices[matrixID]; exists {
		return matrix.Permissions
	}

	return []domain.Permission{}
}

// InitializeDefaultRoles sets up the default role permissions
func (ps PermissionService) InitializeDefaultRoles() error {
	if err := ps.SetRolePermissions("admin", domain.AdminPermissions); err != nil {
		return err
	}

	if err := ps.SetRolePermissions("analyst", domain.AnalystPermissions); err != nil {
		return err
	}

	if err := ps.SetRolePermissions("viewer", domain.ViewerPermissions); err != nil {
		return err
	}

	return nil
}
