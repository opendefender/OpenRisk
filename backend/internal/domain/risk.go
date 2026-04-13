package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
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
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`

	// Smart Scoring : 1 (Low) à 5 (Critical)
	Impact      int     `gorm:"default:1;check:impact >= 1 AND impact <= 5" json:"impact"`
	Probability int     `gorm:"default:1;check:probability >= 1 AND probability <= 5" json:"probability"`
	Score       float64 `gorm:"type:numeric(8,2);default:0" json:"score"` // Champ calculé (Impact * Probability * asset factor)

	// Contextualisation & Conformité
	Status RiskStatus     `gorm:"default:'DRAFT';index" json:"status"`
	Tags   pq.StringArray `gorm:"type:text[]" json:"tags"` // Ex: ["CIS", "ISO27001", "GDPR"]
	Owner  string         `json:"owner"`                   // Email ou UserID

	// Intégrations OpenDefender (TheHive, OpenCTI, OpenRMF)
	Source     string `gorm:"default:'MANUAL'" json:"source"` // "MANUAL", "THEHIVE", "OPENRMF"
	ExternalID string `gorm:"index" json:"external_id"`       // ID dans l'outil tiers

	// Multi-tenancy — Claude.md rule #2: filter by tenant_id on EVERY query
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null;default:gen_random_uuid()" json:"organization_id"`
	Level          string    `gorm:"size:20;default:'LOW'" json:"level"` // CRITICAL, HIGH, MEDIUM, LOW

	// Audit
	// Flexible custom fields
	CustomFields datatypes.JSON `gorm:"type:jsonb" json:"custom_fields,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Mitigations []Mitigation `gorm:"foreignKey:RiskID" json:"mitigations,omitempty"`

	Assets []*Asset `gorm:"many2many:risk_assets;" json:"assets,omitempty"`

	// Framework classifications (ISO27001, NIST, CIS, OWASP...)
	Frameworks pq.StringArray `gorm:"type:text[]" json:"frameworks,omitempty"`
}

func (r *Risk) BeforeSave(tx *gorm.DB) (err error) {
	// Basic score calculation only when not already computed by handlers.
	// Handlers may compute a final score using asset criticality and set r.Score.
	if r.Score == 0 {
		r.Score = float64(r.Impact * r.Probability)
	}
	return
}

// AfterSave : Gère la logique après la sauvegarde (enregistrement de l'historique)
// Ce hook est essentiel pour les fonctionnalités de Timeline et de Trends.
func (r *Risk) AfterSave(tx *gorm.DB) (err error) {
	// Always create a history snapshot after save for timeline and trends.
	history := RiskHistory{
		RiskID:      r.ID,
		Score:       r.Score,
		Impact:      r.Impact,
		Probability: r.Probability,
		Status:      r.Status,
		ChangedBy:   r.Owner,
		ChangeType:  "UPDATE",
		CreatedAt:   time.Now(),
	}

	return tx.Create(&history).Error
}
