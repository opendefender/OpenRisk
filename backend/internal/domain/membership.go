package domain

import (
	"time"

	"github.com/google/uuid"
)

// MemberRole represents a role within an organization
type MemberRole string

const (
	RoleRoot  MemberRole = "root"
	RoleAdmin MemberRole = "admin"
	RoleUser  MemberRole = "user"
)

// OrganizationMember represents a user's membership in an organization
type OrganizationMember struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID     `gorm:"index" json:"organization_id"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	UserID         uuid.UUID     `gorm:"index" json:"user_id"`
	User           *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role           MemberRole    `gorm:"not null" json:"role"`
	ProfileID      *uuid.UUID    `gorm:"type:uuid;index" json:"profile_id,omitempty"` // For role='user' only
	Profile        *Profile      `gorm:"foreignKey:ProfileID" json:"profile,omitempty"`
	IsActive       bool          `gorm:"default:true;index" json:"is_active"`
	JoinedAt       time.Time     `gorm:"autoCreateTime" json:"joined_at"`
	InvitedByID    *uuid.UUID    `gorm:"type:uuid" json:"invited_by_id,omitempty"`
	InvitedBy      *User         `gorm:"foreignKey:InvitedByID" json:"invited_by,omitempty"`
	CreatedAt      time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for OrganizationMember
func (OrganizationMember) TableName() string {
	return "organization_members"
}

// IsRoot checks if the member has root role
func (m *OrganizationMember) IsRoot() bool {
	return m.Role == RoleRoot
}

// IsAdmin checks if the member has admin role or higher
func (m *OrganizationMember) IsAdmin() bool {
	return m.Role == RoleAdmin || m.Role == RoleRoot
}

// NeedsProfile checks if the member requires a profile to have permissions
func (m *OrganizationMember) NeedsProfile() bool {
	return m.Role == RoleUser
}

// GetPermissionSet returns the PermissionSet for this organization member
func (m *OrganizationMember) GetPermissionSet() PermissionSet {
	switch m.Role {
	case RoleRoot:
		return NewFullPermissionSet()
	case RoleAdmin:
		return NewAdminPermissionSet()
	case RoleUser:
		if m.Profile != nil {
			return NewProfilePermissionSet(m.Profile.Permissions)
		}
		// User without profile has no permissions
		return PermissionSet{rules: make(map[Resource]map[Action]Scope)}
	default:
		return PermissionSet{rules: make(map[Resource]map[Action]Scope)}
	}
}
