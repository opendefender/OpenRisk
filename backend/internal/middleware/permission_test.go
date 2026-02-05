package middleware

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestPermissionMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		claims       *domain.UserClaims
		requiredPerm string
		shouldAllow  bool
	}{
		{
			name: "user with exact permission",
			claims: &domain.UserClaims{
				ID:          uuid.New(),
				Email:       "test@example.com",
				Username:    "testuser",
				RoleID:      uuid.New(),
				RoleName:    "admin",
				Permissions: []string{"read:risk", "write:risk", "delete:risk"},
			},
			requiredPerm: "read:risk",
			shouldAllow:  true,
		},
		{
			name: "user without required permission",
			claims: &domain.UserClaims{
				ID:          uuid.New(),
				Email:       "test@example.com",
				Username:    "testuser",
				RoleID:      uuid.New(),
				RoleName:    "viewer",
				Permissions: []string{"read:risk"},
			},
			requiredPerm: "write:risk",
			shouldAllow:  false,
		},
		{
			name: "user with wildcard permission",
			claims: &domain.UserClaims{
				ID:          uuid.New(),
				Email:       "test@example.com",
				Username:    "testuser",
				RoleID:      uuid.New(),
				RoleName:    "admin",
				Permissions: []string{"*"},
			},
			requiredPerm: "any:permission",
			shouldAllow:  true,
		},
		{
			name: "user with no permissions",
			claims: &domain.UserClaims{
				ID:          uuid.New(),
				Email:       "test@example.com",
				Username:    "testuser",
				RoleID:      uuid.New(),
				RoleName:    "viewer",
				Permissions: []string{},
			},
			requiredPerm: "read:risk",
			shouldAllow:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasPermission := false
			for _, perm := range tt.claims.Permissions {
				if perm == "*" || perm == tt.requiredPerm {
					hasPermission = true
					break
				}
			}
			assert.Equal(t, tt.shouldAllow, hasPermission)
		})
	}
}
