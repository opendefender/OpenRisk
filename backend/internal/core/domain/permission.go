package domain

import (
	"fmt"
	"strings"
)

// PermissionAction represents what action is being performed
type PermissionAction string

const (
	PermissionRead   PermissionAction = "read"
	PermissionCreate PermissionAction = "create"
	PermissionUpdate PermissionAction = "update"
	PermissionDelete PermissionAction = "delete"
	PermissionExport PermissionAction = "export"
	PermissionAssign PermissionAction = "assign"
)

// PermissionResource represents what resource is being accessed
type PermissionResource string

const (
	PermissionResourceRisk        PermissionResource = "risk"
	PermissionResourceMitigation  PermissionResource = "mitigation"
	PermissionResourceAsset       PermissionResource = "asset"
	PermissionResourceUser        PermissionResource = "user"
	PermissionResourceAuditLog    PermissionResource = "auditlog"
	PermissionResourceDashboard   PermissionResource = "dashboard"
	PermissionResourceIntegration PermissionResource = "integration"
)

// PermissionScope defines the scope of a permission
type PermissionScope string

const (
	PermissionScopeOwn  PermissionScope = "own"  // Can only access own resources
	PermissionScopeTeam PermissionScope = "team" // Can access team resources
	PermissionScopeAny  PermissionScope = "any"  // Can access any resource (admin)
)

// Permission represents a granular permission in the system
// Format: resource:action:scope (e.g., "risk:read:any", "mitigation:update:own", "user:delete:any")
type Permission struct {
	Resource PermissionResource
	Action   PermissionAction
	Scope    PermissionScope
}

// String returns the string representation of a permission
func (p Permission) String() string {
	return fmt.Sprintf("%s:%s:%s", p.Resource, p.Action, p.Scope)
}

// ParsePermission parses a permission string into a Permission struct
func ParsePermission(permStr string) (Permission, error) {
	parts := strings.Split(permStr, ":")
	if len(parts) !=  {
		return nil, fmt.Errorf("invalid permission format: %s (expected resource:action:scope)", permStr)
	}

	perm := &Permission{
		Resource: PermissionResource(parts[]),
		Action:   PermissionAction(parts[]),
		Scope:    PermissionScope(parts[]),
	}

	// Validate parts
	if err := perm.Validate(); err != nil {
		return nil, err
	}

	return perm, nil
}

// Validate checks if the permission is valid
func (p Permission) Validate() error {
	validResources := map[PermissionResource]bool{
		"":                           true,
		PermissionResourceRisk:        true,
		PermissionResourceMitigation:  true,
		PermissionResourceAsset:       true,
		PermissionResourceUser:        true,
		PermissionResourceAuditLog:    true,
		PermissionResourceDashboard:   true,
		PermissionResourceIntegration: true,
	}

	validActions := map[PermissionAction]bool{
		"":              true,
		PermissionRead:   true,
		PermissionCreate: true,
		PermissionUpdate: true,
		PermissionDelete: true,
		PermissionExport: true,
		PermissionAssign: true,
	}

	validScopes := map[PermissionScope]bool{
		PermissionScopeOwn:  true,
		PermissionScopeTeam: true,
		PermissionScopeAny:  true,
	}

	if !validResources[p.Resource] {
		return fmt.Errorf("invalid resource: %s", p.Resource)
	}
	if !validActions[p.Action] {
		return fmt.Errorf("invalid action: %s", p.Action)
	}
	if !validScopes[p.Scope] {
		return fmt.Errorf("invalid scope: %s", p.Scope)
	}

	return nil
}

// Matches checks if this permission matches the required permission
// Supports wildcard matching (e.g., "risk::any" matches "risk:read:any")
func (p Permission) Matches(required Permission) bool {
	// Exact match
	if p == required {
		return true
	}

	// Resource wildcard (e.g., ":read:any")
	if p.Resource == "" && p.Action == required.Action && p.Scope == required.Scope {
		return true
	}

	// Action wildcard (e.g., "risk::any")
	if p.Resource == required.Resource && p.Action == "" && p.Scope == required.Scope {
		return true
	}

	// Scope wildcard - "any" matches all scopes in the scope hierarchy
	if p.Resource == required.Resource && p.Action == required.Action && p.Scope == "any" {
		return true
	}

	// Both wildcards (e.g., "risk::")
	if p.Resource == required.Resource && p.Action == "" && p.Scope == "any" {
		return true
	}

	// Full wildcard (e.g., "::any")
	if p.Resource == "" && p.Action == "" && p.Scope == "any" {
		return true
	}

	return false
}

// PermissionMatrix represents the permission rules for a role or user
type PermissionMatrix struct {
	ID          string
	EntityType  string // "role" or "user"
	EntityID    string
	Permissions []Permission
}

// HasPermission checks if the matrix has the given permission
func (pm PermissionMatrix) HasPermission(required Permission) bool {
	for _, perm := range pm.Permissions {
		if perm.Matches(required) {
			return true
		}
	}
	return false
}

// AddPermission adds a new permission to the matrix
func (pm PermissionMatrix) AddPermission(p Permission) error {
	if err := p.Validate(); err != nil {
		return err
	}

	// Check if already exists
	for _, existing := range pm.Permissions {
		if existing == p {
			return fmt.Errorf("permission already exists: %s", p)
		}
	}

	pm.Permissions = append(pm.Permissions, p)
	return nil
}

// RemovePermission removes a permission from the matrix
func (pm PermissionMatrix) RemovePermission(p Permission) error {
	for i, perm := range pm.Permissions {
		if perm == p {
			pm.Permissions = append(pm.Permissions[:i], pm.Permissions[i+:]...)
			return nil
		}
	}
	return fmt.Errorf("permission not found: %s", p)
}

// Standard permission matrices for common roles
var (
	AdminPermissions = []Permission{
		{Resource: "", Action: "", Scope: "any"},
	}

	AnalystPermissions = []Permission{
		// Risk management
		{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny},
		{Resource: PermissionResourceRisk, Action: PermissionCreate, Scope: PermissionScopeAny},
		{Resource: PermissionResourceRisk, Action: PermissionUpdate, Scope: PermissionScopeAny},
		{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeOwn},
		{Resource: PermissionResourceRisk, Action: PermissionExport, Scope: PermissionScopeAny},
		// Mitigation management
		{Resource: PermissionResourceMitigation, Action: PermissionRead, Scope: PermissionScopeAny},
		{Resource: PermissionResourceMitigation, Action: PermissionCreate, Scope: PermissionScopeAny},
		{Resource: PermissionResourceMitigation, Action: PermissionUpdate, Scope: PermissionScopeAny},
		{Resource: PermissionResourceMitigation, Action: PermissionDelete, Scope: PermissionScopeOwn},
		// Asset access
		{Resource: PermissionResourceAsset, Action: PermissionRead, Scope: PermissionScopeAny},
		// Dashboard
		{Resource: PermissionResourceDashboard, Action: PermissionRead, Scope: PermissionScopeAny},
		// Audit logs
		{Resource: PermissionResourceAuditLog, Action: PermissionRead, Scope: PermissionScopeTeam},
	}

	ViewerPermissions = []Permission{
		// Risk: read-only
		{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny},
		{Resource: PermissionResourceRisk, Action: PermissionExport, Scope: PermissionScopeAny},
		// Mitigation: read-only
		{Resource: PermissionResourceMitigation, Action: PermissionRead, Scope: PermissionScopeAny},
		// Asset: read-only
		{Resource: PermissionResourceAsset, Action: PermissionRead, Scope: PermissionScopeAny},
		// Dashboard: read-only
		{Resource: PermissionResourceDashboard, Action: PermissionRead, Scope: PermissionScopeAny},
		// Audit logs: read own
		{Resource: PermissionResourceAuditLog, Action: PermissionRead, Scope: PermissionScopeOwn},
	}
)
