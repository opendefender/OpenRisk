package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Resource represents a resource type in the permission system
type Resource string

const (
	ResourceRisks        Resource = "risks"
	ResourceAssets       Resource = "assets"
	ResourceMitigations  Resource = "mitigations"
	ResourceUsers        Resource = "users"
	ResourceAuditLogs    Resource = "audit_logs"
	ResourceSettings     Resource = "settings"
	ResourceMembers      Resource = "members"
	ResourceProfiles     Resource = "profiles"
	ResourceReports      Resource = "reports"
	ResourceIntegrations Resource = "integrations"
	ResourceConnectors   Resource = "connectors"
	ResourceGroups       Resource = "groups"
)

// Action represents an action that can be performed on a resource
type Action string

const (
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
	ActionManage Action = "manage"
	ActionExport Action = "export"
	ActionAssign Action = "assign"
)

// Scope represents the scope of a permission
type Scope string

const (
	ScopeAll      Scope = "all"      // Can access all instances of the resource
	ScopeAssigned Scope = "assigned" // Can only access assigned instances
	ScopeOwn      Scope = "own"      // Can only access own instances
	ScopeNone     Scope = "none"     // No access
)

// Profile represents an IAM role/profile within an organization
type Profile struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID     `gorm:"index" json:"organization_id"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Name           string        `gorm:"not null" json:"name"`
	Description    string        `json:"description,omitempty"`
	IsSystem       bool          `gorm:"default:false" json:"is_system"`  // true = built-in, not deletable
	IsDefault      bool          `gorm:"default:false" json:"is_default"` // auto-assigned to new members
	CreatedByID    uuid.UUID     `gorm:"index" json:"created_by_id"`
	CreatedBy      *User         `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
	CreatedAt      time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time     `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Permissions []ProfilePermission  `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE" json:"permissions,omitempty"`
	Members     []OrganizationMember `gorm:"foreignKey:ProfileID" json:"members,omitempty"`
}

// TableName specifies the table name for Profile
func (Profile) TableName() string {
	return "profiles"
}

// ProfilePermission represents a specific permission granted to a profile
type ProfilePermission struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProfileID  uuid.UUID      `gorm:"index" json:"profile_id"`
	Profile    *Profile       `gorm:"foreignKey:ProfileID" json:"profile,omitempty"`
	Resource   Resource       `gorm:"not null" json:"resource"`
	Action     Action         `gorm:"not null" json:"action"`
	Scope      Scope          `gorm:"not null;default:'none'" json:"scope"`
	Conditions datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"conditions,omitempty"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for ProfilePermission
func (ProfilePermission) TableName() string {
	return "profile_permissions"
}

// GetConditions unmarshals the conditions JSON
func (pp *ProfilePermission) GetConditions() map[string]interface{} {
	var conds map[string]interface{}
	if err := json.Unmarshal(pp.Conditions, &conds); err != nil {
		return map[string]interface{}{}
	}
	return conds
}

// PermissionSet is the resolved in-memory permission map for a request context
type PermissionSet struct {
	IsRoot  bool
	IsAdmin bool
	rules   map[Resource]map[Action]Scope
}

// NewFullPermissionSet creates a permission set with all access (for root/admin)
func NewFullPermissionSet() PermissionSet {
	ps := PermissionSet{IsRoot: true, IsAdmin: true, rules: make(map[Resource]map[Action]Scope)}
	// Grant all permissions
	for _, resource := range []Resource{
		ResourceRisks, ResourceAssets, ResourceMitigations, ResourceUsers,
		ResourceAuditLogs, ResourceSettings, ResourceMembers, ResourceProfiles,
		ResourceReports, ResourceIntegrations, ResourceConnectors, ResourceGroups,
	} {
		ps.rules[resource] = map[Action]Scope{
			ActionRead:   ScopeAll,
			ActionWrite:  ScopeAll,
			ActionDelete: ScopeAll,
			ActionManage: ScopeAll,
			ActionExport: ScopeAll,
			ActionAssign: ScopeAll,
		}
	}
	return ps
}

// NewAdminPermissionSet creates a permission set for admin role
func NewAdminPermissionSet() PermissionSet {
	ps := PermissionSet{IsAdmin: true, rules: make(map[Resource]map[Action]Scope)}
	// Admin can do most things except certain settings
	for _, resource := range []Resource{
		ResourceRisks, ResourceAssets, ResourceMitigations, ResourceUsers,
		ResourceAuditLogs, ResourceMembers, ResourceProfiles, ResourceReports,
		ResourceIntegrations, ResourceConnectors, ResourceGroups,
	} {
		ps.rules[resource] = map[Action]Scope{
			ActionRead:   ScopeAll,
			ActionWrite:  ScopeAll,
			ActionDelete: ScopeAll,
			ActionManage: ScopeAll,
			ActionExport: ScopeAll,
			ActionAssign: ScopeAll,
		}
	}
	// Settings is limited for admins
	ps.rules[ResourceSettings] = map[Action]Scope{
		ActionRead: ScopeAll,
	}
	return ps
}

// NewProfilePermissionSet creates a permission set from profile permissions
func NewProfilePermissionSet(perms []ProfilePermission) PermissionSet {
	ps := PermissionSet{rules: make(map[Resource]map[Action]Scope)}
	for _, perm := range perms {
		if ps.rules[perm.Resource] == nil {
			ps.rules[perm.Resource] = make(map[Action]Scope)
		}
		ps.rules[perm.Resource][perm.Action] = perm.Scope
	}
	return ps
}

// Can checks if the permission set allows an action on a resource
func (ps *PermissionSet) Can(resource Resource, action Action) (allowed bool, scope Scope) {
	if ps.IsRoot {
		return true, ScopeAll
	}
	if ps.IsAdmin {
		return true, ScopeAll
	}

	if actions, ok := ps.rules[resource]; ok {
		if scope, exists := actions[action]; exists {
			return scope != ScopeNone, scope
		}
	}
	return false, ScopeNone
}

// MustScope returns the scope for a permission, panicking if not allowed
func (ps *PermissionSet) MustScope(resource Resource, action Action) Scope {
	_, scope := ps.Can(resource, action)
	return scope
}

// HasResource checks if the permission set has any permissions on a resource
func (ps *PermissionSet) HasResource(resource Resource) bool {
	if ps.IsRoot || ps.IsAdmin {
		return true
	}
	_, ok := ps.rules[resource]
	return ok
}
