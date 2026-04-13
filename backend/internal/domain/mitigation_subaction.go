package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MitigationSubAction struct {
	ID           uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MitigationID uuid.UUID      `gorm:"type:uuid;index" json:"mitigation_id"`
	Title        string         `gorm:"not null" json:"title"`
	Completed    bool           `gorm:"default:false" json:"completed"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MitigationSubAction) TableName() string { return "mitigation_subactions" }
