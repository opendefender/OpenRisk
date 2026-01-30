package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Role represents a set of permissions for RBAC
type Role struct {
	ID          uuid.UUID      gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	Name        string         gorm:"uniqueIndex;not null" json:"name" // admin, analyst, viewer
	Description string         json:"description"
	Permissions pq.StringArray gorm:"type:text[]" json:"permissions" // e.g., ["risk:read", "risk:create"]
	CreatedAt   time.Time      json:"created_at"
	UpdatedAt   time.Time      json:"updated_at"
}

// User represents an authenticated system user with a role
type User struct {
	ID         uuid.UUID  gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	Email      string     gorm:"uniqueIndex;not null" json:"email"
	Username   string     gorm:"uniqueIndex;not null" json:"username"
	Password   string     json:"-" // Never return in JSON
	FullName   string     json:"full_name"
	Bio        string     json:"bio"
	Phone      string     json:"phone"
	Department string     json:"department"
	Timezone   string     gorm:"default:'UTC'" json:"timezone"
	RoleID     uuid.UUID  gorm:"index" json:"role_id"
	Role       Role      json:"role,omitempty"
	IsActive   bool       gorm:"default:true;index" json:"is_active"
	AvatarURL  string     json:"avatar_url"
	LastLogin  time.Time json:"last_login,omitempty"

	// RBAC Extensions (Phase  Priority )
	TenantID    uuid.UUID gorm:"type:uuid;index" json:"tenant_id,omitempty" // NULL for system-wide users
	CreatedByID uuid.UUID gorm:"type:uuid;index" json:"created_by_id,omitempty"

	CreatedAt time.Time      json:"created_at"
	UpdatedAt time.Time      json:"updated_at"
	DeletedAt gorm.DeletedAt gorm:"index" json:"-"
}

// UserClaims represents JWT claims with user and role information
type UserClaims struct {
	ID          uuid.UUID json:"id"
	Email       string    json:"email"
	Username    string    json:"username"
	RoleID      uuid.UUID json:"role_id"
	RoleName    string    json:"role_name"
	Permissions []string  json:"permissions"
	ExpiresAt   int     json:"exp"
	IssuedAt    int     json:"iat"
}

// Implement jwt.Claims interface
func (c UserClaims) GetExpirationTime() (jwt.NumericDate, error) {
	if c.ExpiresAt ==  {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, )), nil
}

func (c UserClaims) GetIssuedAt() (jwt.NumericDate, error) {
	if c.IssuedAt ==  {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, )), nil
}

func (c UserClaims) GetNotBefore() (jwt.NumericDate, error) {
	return nil, nil
}

func (c UserClaims) GetIssuer() (string, error) {
	return "openrisk", nil
}

func (c UserClaims) GetSubject() (string, error) {
	return c.ID.String(), nil
}

func (c UserClaims) GetAudience() (jwt.ClaimStrings, error) {
	return jwt.ClaimStrings{"openrisk-api"}, nil
}

// HasPermission checks if user claims has a specific permission
func (c UserClaims) HasPermission(permission string) bool {
	if c == nil || len(c.Permissions) ==  {
		return false
	}

	for _, perm := range c.Permissions {
		// Exact match or admin wildcard
		if perm == permission || perm == PermissionAll {
			return true
		}
		// Resource-level wildcard (e.g., "risk:" matches "risk:read")
		if len(perm) >  && perm[len(perm)-:] == ":" {
			resourceWildcard := perm[:len(perm)-]
			if len(permission) > len(resourceWildcard) && permission[:len(resourceWildcard)] == resourceWildcard {
				return true
			}
		}
	}
	return false
}

const (
	// Risk permissions
	PermissionRiskRead   = "risk:read"
	PermissionRiskCreate = "risk:create"
	PermissionRiskUpdate = "risk:update"
	PermissionRiskDelete = "risk:delete"
	PermissionRiskAll    = "risk:"

	// Mitigation permissions
	PermissionMitigationRead   = "mitigation:read"
	PermissionMitigationCreate = "mitigation:create"
	PermissionMitigationUpdate = "mitigation:update"
	PermissionMitigationDelete = "mitigation:delete"
	PermissionMitigationAll    = "mitigation:"

	// Asset permissions
	PermissionAssetRead = "asset:read"
	PermissionAssetAll  = "asset:"

	// User/Admin permissions
	PermissionUserManage = "user:manage"
	PermissionAll        = ""
)

// HasPermission checks if user has a specific permission
func (u User) HasPermission(permission string) bool {
	if u == nil || u.Role == nil {
		return false
	}
	return RoleHasPermission(u.Role, permission)
}

// RoleHasPermission checks if role has a specific permission
func RoleHasPermission(role Role, permission string) bool {
	if role == nil {
		return false
	}

	for _, perm := range role.Permissions {
		// Exact match or admin wildcard
		if perm == permission || perm == PermissionAll {
			return true
		}
		// Resource-level wildcard (e.g., "risk:" matches "risk:read")
		if len(perm) >  && perm[len(perm)-:] == ":" {
			resourceWildcard := perm[:len(perm)-]
			if len(permission) > len(resourceWildcard) && permission[:len(resourceWildcard)] == resourceWildcard {
				return true
			}
		}
	}
	return false
}

// CanAccessResource checks if user can access a specific resource with action
func (u User) CanAccessResource(resource string, action string) bool {
	return u.HasPermission(resource + ":" + action)
}

// Standard roles for RBAC
var (
	// AdminRole has all permissions
	AdminRole = &Role{
		Name:        "admin",
		Permissions: []string{PermissionAll},
	}

	// AnalystRole can create and manage risks/mitigations
	AnalystRole = &Role{
		Name: "analyst",
		Permissions: []string{
			PermissionRiskRead, PermissionRiskCreate, PermissionRiskUpdate,
			PermissionMitigationRead, PermissionMitigationCreate, PermissionMitigationUpdate,
			PermissionAssetRead,
		},
	}

	// ViewerRole has read-only access
	ViewerRole = &Role{
		Name: "viewer",
		Permissions: []string{
			PermissionRiskRead,
			PermissionMitigationRead,
			PermissionAssetRead,
		},
	}
)
