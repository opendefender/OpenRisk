// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
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
	return m.Role == "root"
}

// IsAdmin checks if the member has admin role or higher
func (m *OrganizationMember) IsAdmin() bool {
	return m.Role == "admin" || m.Role == "root"
}

// NeedsProfile checks if the member requires a profile to have permissions
func (m *OrganizationMember) NeedsProfile() bool {
	return m.Role == "user"
}

// OrganizationRole represents a custom IAM role within an organization
type OrganizationRole struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name           string         `gorm:"not null;index" json:"name"` // custom role name
	Description    string         `json:"description"`
	Permissions    datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"permissions"` // list of permissions
	IsActive       bool           `gorm:"default:true;index" json:"is_active"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

// TableName specifies the table name
func (OrganizationRole) TableName() string {
	return "organization_roles"
}

// AuthAuditLog represents an audit log entry for authentication events
type AuthAuditLog struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID            *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	TenantID          *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	Action            string     `gorm:"not null;index" json:"action"` // login | refresh | logout | mfa_setup | switch_org | pat_create ...
	IP                string     `gorm:"type:varchar(45)" json:"ip"`   // IPv4/IPv6
	UserAgent         string     `gorm:"type:text" json:"user_agent"`
	GeoCountry        *string    `gorm:"type:varchar(2)" json:"geo_country,omitempty"`
	Success           bool       `gorm:"default:true;index" json:"success"`
	FailureReason     *string    `gorm:"type:text" json:"failure_reason,omitempty"`
	DeviceFingerprint *string    `gorm:"type:varchar(255)" json:"device_fingerprint,omitempty"`
	CreatedAt         time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName specifies the table name
func (AuthAuditLog) TableName() string {
	return "auth_audit_logs"
}

// PersonalAccessToken represents a PAT for API access
type PersonalAccessToken struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;index;not null" json:"user_id"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description"`
	TokenHash   string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"` // SHA256 hash
	TokenPrefix string         `gorm:"type:varchar(8);index;not null" json:"token_prefix"`
	Scopes      datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"scopes"` // permission scopes
	ExpiresAt   *time.Time     `gorm:"index" json:"expires_at,omitempty"`
	LastUsedAt  *time.Time     `json:"last_used_at,omitempty"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name
func (PersonalAccessToken) TableName() string {
	return "personal_access_tokens"
}

// IsExpired checks if the PAT has expired
func (pat *PersonalAccessToken) IsExpired() bool {
	if pat.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*pat.ExpiresAt)
}

// UpdateLastUsed updates the last used timestamp
func (pat *PersonalAccessToken) UpdateLastUsed() {
	now := time.Now()
	pat.LastUsedAt = &now
	pat.UpdatedAt = now
}

// GetPermissionSet returns the PermissionSet for this organization member
func (m *OrganizationMember) GetPermissionSet() PermissionSet {
	switch m.Role {
	case "root":
		return NewFullPermissionSet()
	case "admin":
		return NewAdminPermissionSet()
	case "user":
		if m.Profile != nil {
			return NewProfilePermissionSet(m.Profile.Permissions)
		}
		// User without profile has no permissions
		return PermissionSet{rules: make(map[Resource]map[Action]Scope)}
	default:
		return PermissionSet{rules: make(map[Resource]map[Action]Scope)}
	}
}
