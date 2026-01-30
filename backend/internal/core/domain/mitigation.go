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
	ID     uuid.UUID gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	RiskID uuid.UUID gorm:"type:uuid;index" json:"risk_id" // Cl trangre

	Title    string           gorm:"not null" json:"title"
	Assignee string           json:"assignee" // Ex: "john@opendefender.io"
	Status   MitigationStatus gorm:"default:'PLANNED'" json:"status"
	Progress int              gorm:"default:" json:"progress" //  à %

	DueDate   time.Time      json:"due_date"
	CreatedAt time.Time      json:"created_at"
	UpdatedAt time.Time      json:"updated_at"
	DeletedAt gorm.DeletedAt gorm:"index" json:"-"

	// Recommendation Engine
	Cost           int gorm:"default:" json:"cost"            // Catgorie de coût:  (Faible) à  (Élev)
	MitigationTime int gorm:"default:" json:"mitigation_time" // Temps estim en Jours

	// Champ non-persistant pour le calcul du SPP
	WeightedPriority float gorm:"-" json:"weighted_priority"

	// Relation avec le Risque (pour la lecture)
	Risk Risk json:"risk,omitempty" // Preload

	// Checklist / Sub-actions
	SubActions []MitigationSubAction gorm:"foreignKey:MitigationID" json:"sub_actions,omitempty"

	gorm.Model
}
