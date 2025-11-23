package domain

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MitigationStatus string

const (
	MitigationPlanned    MitigationStatus = "PLANNED"
	MitigationInProgress MitigationStatus = "IN_PROGRESS"
	MitigationDone       MitigationStatus = "DONE"
)

type Mitigation struct {
	ID        uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RiskID    uuid.UUID        `gorm:"type:uuid;index" json:"risk_id"` // Clé étrangère
	Risk      Risk             `json:"-"` // Relation pour GORM (ne pas serializer pour éviter boucle)
	
	Title     string           `gorm:"not null" json:"title"`
	Assignee  string           `json:"assignee"` // Ex: "john@opendefender.io"
	Status    MitigationStatus `gorm:"default:'PLANNED'" json:"status"`
	Progress  int              `gorm:"default:0" json:"progress"` // 0 à 100%
	
	DueDate   time.Time      `json:"due_date"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}