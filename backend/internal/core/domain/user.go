package domain

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin   Role = "ADMIN"   // Tout pouvoir + Config
	RoleAnalyst Role = "ANALYST" // CRUD Risques & Mitigations
	RoleViewer  Role = "VIEWER"  // Read-only Dashboard
)

type User struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	Password  string         `json:"-"` 
	FullName  string         `json:"full_name"`
	Role      Role           `gorm:"default:'VIEWER'" json:"role"`
	AvatarURL string         `json:"avatar_url"`
	
	LastLogin *time.Time     `json:"last_login"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}