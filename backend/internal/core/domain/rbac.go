package domain

import (
"encoding/json"
"time"

"github.com/google/uuid"
"gorm.io/gorm"
)

// ============================================================================
// RBAC DOMAIN MODELS
// ============================================================================

// RoleLevel represents the hierarchical level of a role
type RoleLevel int

const (
RoleLevelViewer   RoleLevel =  // Read-only access
RoleLevelAnalyst  RoleLevel =  // Can create/update
RoleLevelManager  RoleLevel =  // Can manage resources
RoleLevelAdmin    RoleLevel =  // Full access
)

// Tenant represents a multi-tenant organization
type Tenant struct {
ID        uuid.UUID       gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
Name      string          gorm:"not null;index" json:"name"
Slug      string          gorm:"uniqueIndex;not null" json:"slug"
OwnerID   uuid.UUID       gorm:"type:uuid;not null;index" json:"owner_id"
Status    string          gorm:"default:'active';index" json:"status" // active, suspended, deleted
IsActive  bool            gorm:"default:true;index" json:"is_active"
Metadata  json.RawMessage gorm:"type:jsonb;serializer:json" json:"metadata,omitempty"
CreatedAt time.Time       json:"created_at"
UpdatedAt time.Time       json:"updated_at"
DeletedAt gorm.DeletedAt  gorm:"index" json:"-"
}

// TableName specifies the table name for Tenant
func (Tenant) TableName() string {
return "tenants"
}

// RoleEnhanced extends the existing Role with RBAC support
type RoleEnhanced struct {
ID           uuid.UUID       gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
TenantID     uuid.UUID       gorm:"type:uuid;index" json:"tenant_id"
Name         string          gorm:"not null;index" json:"name"
Description  string          json:"description"
Level        RoleLevel       gorm:"default:;index" json:"level"
IsPredefined bool            gorm:"default:false" json:"is_predefined"
IsActive     bool            gorm:"default:true;index" json:"is_active"
Metadata     json.RawMessage gorm:"type:jsonb;serializer:json" json:"metadata,omitempty"
CreatedAt    time.Time       json:"created_at"
UpdatedAt    time.Time       json:"updated_at"
DeletedAt    gorm.DeletedAt  gorm:"index" json:"-"
}

// TableName specifies the table name for RoleEnhanced
func (RoleEnhanced) TableName() string {
return "roles"
}

// PermissionDB represents a permission in the database
type PermissionDB struct {
ID          uuid.UUID       gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
Resource    string          gorm:"not null;index" json:"resource"
Action      string          gorm:"not null;index" json:"action"
Description string          json:"description"
IsSystem    bool            gorm:"default:true" json:"is_system"
Metadata    json.RawMessage gorm:"type:jsonb;serializer:json" json:"metadata,omitempty"
CreatedAt   time.Time       json:"created_at"
UpdatedAt   time.Time       json:"updated_at"
}

// TableName specifies the table name for PermissionDB
func (PermissionDB) TableName() string {
return "permissions"
}

// RolePermission is the junction table between roles and permissions
type RolePermission struct {
RoleID       uuid.UUID gorm:"type:uuid;primaryKey" json:"role_id"
PermissionID uuid.UUID gorm:"type:uuid;primaryKey" json:"permission_id"
CreatedAt    time.Time json:"created_at"
}

// TableName specifies the table name for RolePermission
func (RolePermission) TableName() string {
return "role_permissions"
}

// UserTenant represents the many-to-many relationship between users and tenants
type UserTenant struct {
UserID    uuid.UUID      gorm:"type:uuid;primaryKey" json:"user_id"
TenantID  uuid.UUID      gorm:"type:uuid;primaryKey;index" json:"tenant_id"
RoleID    uuid.UUID      gorm:"type:uuid;not null;index" json:"role_id"
CreatedAt time.Time      json:"created_at"
UpdatedAt time.Time      json:"updated_at"
}

// TableName specifies the table name for UserTenant
func (UserTenant) TableName() string {
return "user_tenants"
}

// ============================================================================
// PERMISSION CONSTANTS FOR DATABASE SEEDING
// ============================================================================

// Permission resource constants (match PermissionResource in permission.go)
const (
PermissionResourceReport    PermissionResource = "report"
PermissionResourceReports   PermissionResource = "reports"
PermissionResourceAudit     PermissionResource = "audit"
PermissionResourceConnector PermissionResource = "connector"
)

// ============================================================================
// RBAC SERVICE INTERFACES
// ============================================================================

// RoleRepository defines the interface for role data access
type RoleRepository interface {
GetByID(id uuid.UUID) (RoleEnhanced, error)
GetByName(tenantID uuid.UUID, name string) (RoleEnhanced, error)
GetByTenant(tenantID uuid.UUID) ([]RoleEnhanced, error)
Create(role RoleEnhanced) error
Update(role RoleEnhanced) error
Delete(id uuid.UUID) error
}

// PermissionRepository defines the interface for permission data access
type PermissionRepository interface {
GetByID(id uuid.UUID) (PermissionDB, error)
GetByResourceAction(resource, action string) (PermissionDB, error)
GetAll() ([]PermissionDB, error)
Create(permission PermissionDB) error
Update(permission PermissionDB) error
Delete(id uuid.UUID) error
}

// TenantRepository defines the interface for tenant data access
type TenantRepository interface {
GetByID(id uuid.UUID) (Tenant, error)
GetBySlug(slug string) (Tenant, error)
Create(tenant Tenant) error
Update(tenant Tenant) error
Delete(id uuid.UUID) error
ListByOwner(ownerID uuid.UUID) ([]Tenant, error)
}

// ============================================================================
// VALIDATION HELPERS
// ============================================================================

// ValidateRoleLevel checks if the role level is valid
func ValidateRoleLevel(level RoleLevel) bool {
return level == RoleLevelViewer || level == RoleLevelAnalyst || level == RoleLevelManager || level == RoleLevelAdmin
}

// ValidateTenantStatus checks if the tenant status is valid
func ValidateTenantStatus(status string) bool {
return status == "active" || status == "suspended" || status == "deleted"
}

// ============================================================================
// RBAC CONTEXT
// ============================================================================

// RBACContext contains RBAC information for a request
type RBACContext struct {
UserID       uuid.UUID
TenantID     uuid.UUID
RoleLevel    RoleLevel
Permissions  []string // List of "resource:action" strings
IsAdmin      bool
}

// HasPermission checks if the context has a specific permission
func (ctx RBACContext) HasPermission(resource, action string) bool {
if ctx.IsAdmin {
return true
}
for _, perm := range ctx.Permissions {
if perm == resource+":"+action {
return true
}
}
return false
}
