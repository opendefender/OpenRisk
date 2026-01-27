package tests

import (
	"context"
	"testing"

	"openrisk/internal/core/domain"
	"openrisk/internal/core/service"

	"github.com/stretchr/testify/assert"
)

// TestRoleServiceCreate tests role creation
func TestRoleServiceCreate(t *testing.T) {
	t.Run("create_valid_role", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Test data
		role := &domain.Role{
			Name:        "Manager",
			Description: "Manager role for teams",
			RoleLevel:   5,
			Permissions: []string{},
		}

		// Execute
		created, err := svc.Create(ctx, role)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, created)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, "Manager", created.Name)
		assert.Equal(t, 5, created.RoleLevel)
	})

	t.Run("create_role_missing_name", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Test data
		role := &domain.Role{
			Name:        "",
			Description: "Invalid role",
			RoleLevel:   5,
		}

		// Execute
		created, err := svc.Create(ctx, role)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Equal(t, "role name is required", err.Error())
	})

	t.Run("create_role_invalid_level", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Test data
		role := &domain.Role{
			Name:        "Invalid",
			Description: "Invalid level",
			RoleLevel:   15, // Out of valid range (0-9)
		}

		// Execute
		created, err := svc.Create(ctx, role)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "role level must be between 0 and 9")
	})
}

// TestRoleServiceRead tests role retrieval
func TestRoleServiceRead(t *testing.T) {
	t.Run("read_existing_role", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create test role
		testRole := &domain.Role{
			ID:          "role-123",
			Name:        "Admin",
			Description: "Administrator",
			RoleLevel:   9,
			Permissions: []string{"*"},
		}
		repo.SetRole(testRole)

		// Execute
		retrieved, err := svc.GetByID(ctx, "role-123")

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "Admin", retrieved.Name)
		assert.Equal(t, 9, retrieved.RoleLevel)
	})

	t.Run("read_nonexistent_role", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Execute
		retrieved, err := svc.GetByID(ctx, "nonexistent")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, retrieved)
	})
}

// TestRoleServiceUpdate tests role updates
func TestRoleServiceUpdate(t *testing.T) {
	t.Run("update_role_fields", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create test role
		testRole := &domain.Role{
			ID:          "role-456",
			Name:        "Viewer",
			Description: "Viewer role",
			RoleLevel:   2,
		}
		repo.SetRole(testRole)

		// Update
		testRole.Description = "Updated viewer role"
		testRole.RoleLevel = 3

		// Execute
		updated, err := svc.Update(ctx, testRole)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, updated)
		assert.Equal(t, "Updated viewer role", updated.Description)
		assert.Equal(t, 3, updated.RoleLevel)
	})

	t.Run("update_invalid_role", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Update non-existent role
		testRole := &domain.Role{
			ID:          "nonexistent",
			Name:        "Test",
			Description: "Test",
			RoleLevel:   5,
		}

		// Execute
		updated, err := svc.Update(ctx, testRole)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, updated)
	})
}

// TestRoleServiceDelete tests role deletion
func TestRoleServiceDelete(t *testing.T) {
	t.Run("delete_existing_role", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create test role
		testRole := &domain.Role{
			ID:        "role-789",
			Name:      "Temporary",
			RoleLevel: 2,
		}
		repo.SetRole(testRole)

		// Execute
		err := svc.Delete(ctx, "role-789")

		// Assert
		assert.NoError(t, err)

		// Verify deletion
		retrieved, _ := svc.GetByID(ctx, "role-789")
		assert.Nil(t, retrieved)
	})

	t.Run("delete_admin_role_fails", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create admin role
		adminRole := &domain.Role{
			ID:        "admin-role",
			Name:      "Administrator",
			RoleLevel: 9,
		}
		repo.SetRole(adminRole)

		// Execute
		err := svc.Delete(ctx, "admin-role")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot delete administrator role")
	})
}

// TestRoleServiceList tests role listing
func TestRoleServiceList(t *testing.T) {
	t.Run("list_all_roles", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create test roles
		roles := []*domain.Role{
			{ID: "role-1", Name: "Admin", RoleLevel: 9},
			{ID: "role-2", Name: "Manager", RoleLevel: 5},
			{ID: "role-3", Name: "Viewer", RoleLevel: 1},
		}

		for _, role := range roles {
			repo.SetRole(role)
		}

		// Execute
		retrieved, err := svc.List(ctx, 0, 10)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 3, len(retrieved))
	})

	t.Run("list_roles_with_pagination", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create multiple roles
		for i := 1; i <= 25; i++ {
			role := &domain.Role{
				ID:        "role-" + string(rune(i)),
				Name:      "Role" + string(rune(i)),
				RoleLevel: i % 10,
			}
			repo.SetRole(role)
		}

		// Execute - get first page
		page1, err := svc.List(ctx, 0, 10)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 10, len(page1))

		// Execute - get second page
		page2, err := svc.List(ctx, 10, 10)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 10, len(page2))
	})
}

// TestRoleServicePermissions tests permission management
func TestRoleServicePermissions(t *testing.T) {
	t.Run("grant_permission_to_role", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create role
		role := &domain.Role{
			ID:          "role-perm",
			Name:        "Editor",
			RoleLevel:   5,
			Permissions: []string{},
		}
		repo.SetRole(role)

		// Execute
		err := svc.GrantPermission(ctx, "role-perm", "users:write")

		// Assert
		assert.NoError(t, err)
		retrieved, _ := svc.GetByID(ctx, "role-perm")
		assert.Contains(t, retrieved.Permissions, "users:write")
	})

	t.Run("revoke_permission_from_role", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create role with permission
		role := &domain.Role{
			ID:          "role-revoke",
			Name:        "Editor",
			RoleLevel:   5,
			Permissions: []string{"users:read", "users:write"},
		}
		repo.SetRole(role)

		// Execute
		err := svc.RevokePermission(ctx, "role-revoke", "users:write")

		// Assert
		assert.NoError(t, err)
		retrieved, _ := svc.GetByID(ctx, "role-revoke")
		assert.NotContains(t, retrieved.Permissions, "users:write")
		assert.Contains(t, retrieved.Permissions, "users:read")
	})

	t.Run("bulk_grant_permissions", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create role
		role := &domain.Role{
			ID:          "role-bulk",
			Name:        "Manager",
			RoleLevel:   6,
			Permissions: []string{},
		}
		repo.SetRole(role)

		// Execute
		perms := []string{"users:read", "users:write", "roles:read", "roles:write"}
		err := svc.BulkGrantPermissions(ctx, "role-bulk", perms)

		// Assert
		assert.NoError(t, err)
		retrieved, _ := svc.GetByID(ctx, "role-bulk")
		assert.Equal(t, 4, len(retrieved.Permissions))
	})
}

// TestRoleServiceHierarchy tests role hierarchy
func TestRoleServiceHierarchy(t *testing.T) {
	t.Run("verify_role_hierarchy", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)

		// Create roles
		admin := &domain.Role{Name: "Admin", RoleLevel: 9}
		manager := &domain.Role{Name: "Manager", RoleLevel: 5}
		viewer := &domain.Role{Name: "Viewer", RoleLevel: 1}

		// Verify hierarchy
		assert.True(t, svc.IsHigherLevel(admin, manager))
		assert.True(t, svc.IsHigherLevel(manager, viewer))
		assert.False(t, svc.IsHigherLevel(viewer, manager))
	})

	t.Run("prevent_permission_escalation", func(t *testing.T) {
		// Setup
		repo := NewMockRoleRepository()
		svc := service.NewRoleService(repo)
		ctx := context.Background()

		// Create roles
		viewer := &domain.Role{
			ID:        "viewer-role",
			Name:      "Viewer",
			RoleLevel: 1,
		}
		repo.SetRole(viewer)

		// Try to grant admin-only permission
		err := svc.GrantPermission(ctx, "viewer-role", "roles:manage")

		// Should fail or require admin approval
		assert.Error(t, err)
	})
}

// BenchmarkRoleServiceCreate benchmarks role creation
func BenchmarkRoleServiceCreate(b *testing.B) {
	repo := NewMockRoleRepository()
	svc := service.NewRoleService(repo)
	ctx := context.Background()

	role := &domain.Role{
		Name:        "Test",
		Description: "Test role",
		RoleLevel:   5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.Create(ctx, role)
	}
}

// BenchmarkRoleServiceGetByID benchmarks role retrieval
func BenchmarkRoleServiceGetByID(b *testing.B) {
	repo := NewMockRoleRepository()
	svc := service.NewRoleService(repo)
	ctx := context.Background()

	role := &domain.Role{
		ID:        "bench-role",
		Name:      "Bench",
		RoleLevel: 5,
	}
	repo.SetRole(role)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetByID(ctx, "bench-role")
	}
}
