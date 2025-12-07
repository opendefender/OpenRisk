package services

import (
	"testing"

	"github.com/opendefender/openrisk/internal/core/domain"
)

func TestPermissionServiceSetRolePermissions(t *testing.T) {
	ps := NewPermissionService()

	permissions := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
		{Resource: domain.PermissionResourceMitigation, Action: domain.PermissionCreate, Scope: domain.PermissionScopeAny},
	}

	if err := ps.SetRolePermissions("analyst", permissions); err != nil {
		t.Errorf("SetRolePermissions() error = %v", err)
	}

	retrieved := ps.GetRolePermissions("analyst")
	if len(retrieved) != len(permissions) {
		t.Errorf("expected %d permissions, got %d", len(permissions), len(retrieved))
	}
}

func TestPermissionServiceCheckPermission(t *testing.T) {
	ps := NewPermissionService()

	permissions := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
		{Resource: domain.PermissionResourceMitigation, Action: domain.PermissionCreate, Scope: domain.PermissionScopeOwn},
	}

	ps.SetRolePermissions("analyst", permissions)

	tests := []struct {
		name     string
		userID   string
		roleID   string
		required domain.Permission
		want     bool
	}{
		{
			name:     "user has permission",
			userID:   "user1",
			roleID:   "analyst",
			required: domain.Permission{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
			want:     true,
		},
		{
			name:     "user does not have permission",
			userID:   "user1",
			roleID:   "analyst",
			required: domain.Permission{Resource: domain.PermissionResourceRisk, Action: domain.PermissionDelete, Scope: domain.PermissionScopeAny},
			want:     false,
		},
		{
			name:     "user has mitigation create permission",
			userID:   "user1",
			roleID:   "analyst",
			required: domain.Permission{Resource: domain.PermissionResourceMitigation, Action: domain.PermissionCreate, Scope: domain.PermissionScopeOwn},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ps.CheckPermission(tt.userID, tt.roleID, tt.required); got != tt.want {
				t.Errorf("CheckPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPermissionServiceUserPermissionsOverrideRole(t *testing.T) {
	ps := NewPermissionService()

	rolePermissions := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
	}

	userPermissions := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionDelete, Scope: domain.PermissionScopeAny},
	}

	ps.SetRolePermissions("analyst", rolePermissions)
	ps.SetUserPermissions("user1", userPermissions)

	// User should have both role and custom permissions
	allPerms := ps.GetUserPermissions("user1", "analyst")
	if len(allPerms) < 2 {
		t.Errorf("expected at least 2 permissions, got %d", len(allPerms))
	}

	// Check that user has custom permission
	if !ps.CheckPermission("user1", "analyst", userPermissions[0]) {
		t.Errorf("user should have custom permission")
	}
}

func TestPermissionServiceAddPermissionToRole(t *testing.T) {
	ps := NewPermissionService()

	perm := domain.Permission{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny}

	if err := ps.AddPermissionToRole("analyst", perm); err != nil {
		t.Errorf("AddPermissionToRole() error = %v", err)
	}

	if !ps.CheckPermission("user1", "analyst", perm) {
		t.Errorf("permission should be added to role")
	}
}

func TestPermissionServiceRemovePermissionFromRole(t *testing.T) {
	ps := NewPermissionService()

	perm := domain.Permission{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny}

	ps.AddPermissionToRole("analyst", perm)

	if err := ps.RemovePermissionFromRole("analyst", perm); err != nil {
		t.Errorf("RemovePermissionFromRole() error = %v", err)
	}

	if ps.CheckPermission("user1", "analyst", perm) {
		t.Errorf("permission should be removed from role")
	}
}

func TestPermissionServiceCheckPermissionMultiple(t *testing.T) {
	ps := NewPermissionService()

	permissions := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
		{Resource: domain.PermissionResourceMitigation, Action: domain.PermissionCreate, Scope: domain.PermissionScopeAny},
	}

	ps.SetRolePermissions("analyst", permissions)

	// Check multiple - should have at least one
	required := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionDelete, Scope: domain.PermissionScopeAny},
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
	}

	if !ps.CheckPermissionMultiple("user1", "analyst", required) {
		t.Errorf("should have at least one permission")
	}
}

func TestPermissionServiceCheckPermissionAll(t *testing.T) {
	ps := NewPermissionService()

	permissions := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
		{Resource: domain.PermissionResourceMitigation, Action: domain.PermissionCreate, Scope: domain.PermissionScopeAny},
	}

	ps.SetRolePermissions("analyst", permissions)

	// Check all - should have all
	required := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
		{Resource: domain.PermissionResourceMitigation, Action: domain.PermissionCreate, Scope: domain.PermissionScopeAny},
	}

	if !ps.CheckPermissionAll("user1", "analyst", required) {
		t.Errorf("should have all permissions")
	}

	// Add impossible permission
	required = append(required, domain.Permission{Resource: domain.PermissionResourceUser, Action: domain.PermissionDelete, Scope: domain.PermissionScopeAny})

	if ps.CheckPermissionAll("user1", "analyst", required) {
		t.Errorf("should not have all permissions")
	}
}

func TestPermissionServiceInitializeDefaultRoles(t *testing.T) {
	ps := NewPermissionService()

	if err := ps.InitializeDefaultRoles(); err != nil {
		t.Errorf("InitializeDefaultRoles() error = %v", err)
	}

	// Check that admin role exists and can do anything
	if !ps.CheckPermission("admin_user", "admin", domain.Permission{Resource: "*", Action: "*", Scope: "any"}) {
		t.Errorf("admin should have full permissions")
	}

	// Check that analyst has limited permissions
	if !ps.CheckPermission("analyst_user", "analyst", domain.Permission{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny}) {
		t.Errorf("analyst should be able to read risks")
	}

	// Check that analyst cannot delete users
	if ps.CheckPermission("analyst_user", "analyst", domain.Permission{Resource: domain.PermissionResourceUser, Action: domain.PermissionDelete, Scope: domain.PermissionScopeAny}) {
		t.Errorf("analyst should not be able to delete users")
	}

	// Check that viewer has read-only
	if ps.CheckPermission("viewer_user", "viewer", domain.Permission{Resource: domain.PermissionResourceRisk, Action: domain.PermissionCreate, Scope: domain.PermissionScopeAny}) {
		t.Errorf("viewer should not be able to create risks")
	}

	if !ps.CheckPermission("viewer_user", "viewer", domain.Permission{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny}) {
		t.Errorf("viewer should be able to read risks")
	}
}

func TestPermissionServiceGetUserPermissions(t *testing.T) {
	ps := NewPermissionService()

	rolePerms := []domain.Permission{
		{Resource: domain.PermissionResourceRisk, Action: domain.PermissionRead, Scope: domain.PermissionScopeAny},
	}

	userPerms := []domain.Permission{
		{Resource: domain.PermissionResourceMitigation, Action: domain.PermissionCreate, Scope: domain.PermissionScopeAny},
	}

	ps.SetRolePermissions("analyst", rolePerms)
	ps.SetUserPermissions("user1", userPerms)

	allPerms := ps.GetUserPermissions("user1", "analyst")

	// Should have 2 total: 1 from role + 1 custom
	if len(allPerms) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(allPerms))
	}
}

func TestPermissionServiceNonexistentRole(t *testing.T) {
	ps := NewPermissionService()

	retrieved := ps.GetRolePermissions("nonexistent")
	if len(retrieved) != 0 {
		t.Errorf("expected 0 permissions for nonexistent role, got %d", len(retrieved))
	}
}
