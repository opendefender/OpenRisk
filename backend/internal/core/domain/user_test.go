package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRoleHasPermission(t testing.T) {
	tests := []struct {
		name       string
		role       Role
		permission string
		expected   bool
	}{
		{
			name: "Admin role has all permissions",
			role: &Role{
				Name:        "admin",
				Permissions: []string{PermissionAll},
			},
			permission: PermissionRiskRead,
			expected:   true,
		},
		{
			name: "Analyst role has risk:read permission",
			role: &Role{
				Name:        "analyst",
				Permissions: []string{PermissionRiskRead, PermissionRiskCreate},
			},
			permission: PermissionRiskRead,
			expected:   true,
		},
		{
			name: "Analyst role lacks risk:delete permission",
			role: &Role{
				Name:        "analyst",
				Permissions: []string{PermissionRiskRead, PermissionRiskCreate},
			},
			permission: PermissionRiskDelete,
			expected:   false,
		},
		{
			name: "Wildcard permission risk: matches risk:read",
			role: &Role{
				Name:        "analyst",
				Permissions: []string{PermissionRiskAll},
			},
			permission: PermissionRiskRead,
			expected:   true,
		},
		{
			name: "Wildcard permission risk: matches risk:delete",
			role: &Role{
				Name:        "analyst",
				Permissions: []string{PermissionRiskAll},
			},
			permission: PermissionRiskDelete,
			expected:   true,
		},
		{
			name: "Viewer role has limited permissions",
			role: &Role{
				Name:        "viewer",
				Permissions: []string{PermissionRiskRead},
			},
			permission: PermissionRiskCreate,
			expected:   false,
		},
		{
			name:       "Nil role has no permissions",
			role:       nil,
			permission: PermissionRiskRead,
			expected:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t testing.T) {
			result := RoleHasPermission(tc.role, tc.permission)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUserHasPermission(t testing.T) {
	tests := []struct {
		name       string
		user       User
		permission string
		expected   bool
	}{
		{
			name: "User with admin role has all permissions",
			user: &User{
				ID:     uuid.New(),
				Email:  "admin@example.com",
				RoleID: uuid.New(),
				Role: &Role{
					Name:        "admin",
					Permissions: []string{PermissionAll},
				},
			},
			permission: PermissionRiskDelete,
			expected:   true,
		},
		{
			name: "User with analyst role has specific permissions",
			user: &User{
				ID:     uuid.New(),
				Email:  "analyst@example.com",
				RoleID: uuid.New(),
				Role: &Role{
					Name:        "analyst",
					Permissions: []string{PermissionRiskRead, PermissionRiskCreate},
				},
			},
			permission: PermissionRiskCreate,
			expected:   true,
		},
		{
			name: "User with nil role has no permissions",
			user: &User{
				ID:    uuid.New(),
				Email: "user@example.com",
				Role:  nil,
			},
			permission: PermissionRiskRead,
			expected:   false,
		},
		{
			name:       "Nil user has no permissions",
			user:       nil,
			permission: PermissionRiskRead,
			expected:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t testing.T) {
			result := tc.user.HasPermission(tc.permission)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCanAccessResource(t testing.T) {
	tests := []struct {
		name     string
		user     User
		resource string
		action   string
		expected bool
	}{
		{
			name: "User can access risk:read",
			user: &User{
				ID: uuid.New(),
				Role: &Role{
					Name:        "viewer",
					Permissions: []string{PermissionRiskRead},
				},
			},
			resource: "risk",
			action:   "read",
			expected: true,
		},
		{
			name: "User cannot access risk:delete",
			user: &User{
				ID: uuid.New(),
				Role: &Role{
					Name:        "viewer",
					Permissions: []string{PermissionRiskRead},
				},
			},
			resource: "risk",
			action:   "delete",
			expected: false,
		},
		{
			name: "Admin can access any resource:action",
			user: &User{
				ID: uuid.New(),
				Role: &Role{
					Name:        "admin",
					Permissions: []string{PermissionAll},
				},
			},
			resource: "system",
			action:   "configure",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t testing.T) {
			result := tc.user.CanAccessResource(tc.resource, tc.action)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStandardRoles(t testing.T) {
	// Test admin role
	assert.Equal(t, "admin", AdminRole.Name)
	assert.Contains(t, AdminRole.Permissions, PermissionAll)

	// Test analyst role
	assert.Equal(t, "analyst", AnalystRole.Name)
	assert.Contains(t, AnalystRole.Permissions, PermissionRiskRead)
	assert.Contains(t, AnalystRole.Permissions, PermissionRiskCreate)
	assert.NotContains(t, AnalystRole.Permissions, PermissionRiskDelete)

	// Test viewer role
	assert.Equal(t, "viewer", ViewerRole.Name)
	assert.Contains(t, ViewerRole.Permissions, PermissionRiskRead)
	assert.NotContains(t, ViewerRole.Permissions, PermissionRiskCreate)
}

func BenchmarkRoleHasPermission(b testing.B) {
	role := &Role{
		Name:        "analyst",
		Permissions: []string{PermissionRiskRead, PermissionRiskCreate, PermissionMitigationRead},
	}

	b.ResetTimer()
	for i := ; i < b.N; i++ {
		RoleHasPermission(role, PermissionRiskRead)
	}
}
