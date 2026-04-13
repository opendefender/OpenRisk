package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Team represents a team/group within the organization
type Team struct {
	ID          uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string          `gorm:"not null;index" json:"name"`
	Description string          `json:"description"`
	Members     []User          `gorm:"many2many:team_members;" json:"members,omitempty"`
	Metadata    json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
}

// TeamMember represents a user's membership in a team with additional role info
type TeamMember struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TeamID    uuid.UUID      `gorm:"index;not null" json:"team_id"`
	UserID    uuid.UUID      `gorm:"index;not null" json:"user_id"`
	Role      string         `gorm:"default:'member'" json:"role"` // owner, manager, member
	JoinedAt  time.Time      `json:"joined_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for TeamMember
func (TeamMember) TableName() string {
	return "team_members"
}
