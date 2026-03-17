package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserSession represents an active user session scoped to an organization
type UserSession struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID     `gorm:"index" json:"user_id"`
	User           *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	OrganizationID uuid.UUID     `gorm:"index" json:"organization_id"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	TokenHash      string        `gorm:"index" json:"token_hash"`
	IPAddress      string        `json:"ip_address,omitempty"`
	UserAgent      string        `json:"user_agent,omitempty"`
	ExpiresAt      time.Time     `json:"expires_at"`
	CreatedAt      time.Time     `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the table name for UserSession
func (UserSession) TableName() string {
	return "user_sessions"
}

// IsExpired checks if the session has expired
func (s *UserSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
