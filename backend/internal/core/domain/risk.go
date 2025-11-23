package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq" 
	"gorm.io/gorm"
)

type RiskStatus string

const (
	StatusDraft     RiskStatus = "DRAFT"
	StatusActive    RiskStatus = "ACTIVE"
	StatusMitigated RiskStatus = "MITIGATED" 
	StatusAccepted  RiskStatus = "ACCEPTED"
)

type Risk struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title       string         `gorm:"size:255;not null" json:"title"`
	Description string         `gorm:"type:text" json:"description"`

	// Smart Scoring : 1 (Low) à 5 (Critical)
	Impact      int `gorm:"default:1;check:impact >= 1 AND impact <= 5" json:"impact"`
	Probability int `gorm:"default:1;check:probability >= 1 AND probability <= 5" json:"probability"`
	Score       int `json:"score"` // Champ calculé (Impact * Probability)

	// Contextualisation & Conformité
	Status RiskStatus     `gorm:"default:'DRAFT';index" json:"status"`
	Tags   pq.StringArray `gorm:"type:text[]" json:"tags"` // Ex: ["CIS", "ISO27001", "GDPR"]
	Owner  string         `json:"owner"`                   // Email ou UserID

	// Intégrations OpenDefender (TheHive, OpenCTI, OpenRMF)
	Source     string `gorm:"default:'MANUAL'" json:"source"` // "MANUAL", "THEHIVE", "OPENRMF"
	ExternalID string `gorm:"index" json:"external_id"`       // ID dans l'outil tiers

	// Audit
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`


	Mitigations []Mitigation `gorm:"foreignKey:RiskID" json:"mitigations"`
}

func (r *Risk) BeforeSave(tx *gorm.DB) (err error) {
	r.Score = r.Impact * r.Probability
	return
}