package domain

import (
	"time"

	"github.com/google/uuid"
)

// InvitationStatus represents the status of an invitation
type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationExpired  InvitationStatus = "expired"
	InvitationRevoked  InvitationStatus = "revoked"
)

// Invitation represents an invitation to join an organization
type Invitation struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Token          uuid.UUID        `gorm:"uniqueIndex" json:"token"`
	OrganizationID uuid.UUID        `gorm:"index" json:"organization_id"`
	Organization   *Organization    `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Email          string           `gorm:"index" json:"email"`
	Role           MemberRole       `gorm:"not null" json:"role"`
	ProfileID      *uuid.UUID       `gorm:"type:uuid" json:"profile_id,omitempty"`
	Profile        *Profile         `gorm:"foreignKey:ProfileID" json:"profile,omitempty"`
	Status         InvitationStatus `gorm:"not null;default:'pending'" json:"status"`
	ExpiresAt      time.Time        `json:"expires_at"`
	InvitedByID    uuid.UUID        `gorm:"index" json:"invited_by_id"`
	InvitedBy      *User            `gorm:"foreignKey:InvitedByID" json:"invited_by,omitempty"`
	CreatedAt      time.Time        `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the table name for Invitation
func (Invitation) TableName() string {
	return "invitations"
}

// IsExpired checks if the invitation has expired
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsUsable checks if the invitation can be accepted
func (i *Invitation) IsUsable() bool {
	return i.Status == InvitationPending && !i.IsExpired()
}
