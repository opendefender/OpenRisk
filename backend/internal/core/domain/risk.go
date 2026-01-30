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
	ID          uuid.UUID gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	Title       string    gorm:"size:;not null" json:"title"
	Description string    gorm:"type:text" json:"description"

	// Smart Scoring :  (Low) Ã   (Critical)
	Impact      int     gorm:"default:;check:impact >=  AND impact <= " json:"impact"
	Probability int     gorm:"default:;check:probability >=  AND probability <= " json:"probability"
	Score       float gorm:"type:numeric(,);default:" json:"score" // Champ calculÃ (Impact  Probability  asset factor)

	// Contextualisation & ConformitÃ
	Status RiskStatus     gorm:"default:'DRAFT';index" json:"status"
	Tags   pq.StringArray gorm:"type:text[]" json:"tags" // Ex: ["CIS", "ISO", "GDPR"]
	Owner  string         json:"owner"                   // Email ou UserID

	// IntÃgrations OpenDefender (TheHive, OpenCTI, OpenRMF)
	Source     string gorm:"default:'MANUAL'" json:"source" // "MANUAL", "THEHIVE", "OPENRMF"
	ExternalID string gorm:"index" json:"external_id"       // ID dans l'outil tiers

	// Audit
	// Flexible custom fields
	CustomFields datatypes.JSON gorm:"type:jsonb" json:"custom_fields,omitempty"
	CreatedAt    time.Time      json:"created_at"
	UpdatedAt    time.Time      json:"updated_at"
	DeletedAt    gorm.DeletedAt gorm:"index" json:"-"

	Mitigations []Mitigation gorm:"foreignKey:RiskID" json:"mitigations,omitempty"

	Assets []Asset gorm:"manymany:risk_assets;" json:"assets,omitempty"

	// Framework classifications (ISO, NIST, CIS, OWASP...)
	Frameworks pq.StringArray gorm:"type:text[]" json:"frameworks,omitempty"
}

func (r Risk) BeforeSave(tx gorm.DB) (err error) {
	// Basic score calculation only when not already computed by handlers.
	// Handlers may compute a final score using asset criticality and set r.Score.
	if r.Score ==  {
		r.Score = float(r.Impact  r.Probability)
	}
	return
}

// AfterSave : GÃre la logique aprÃs la sauvegarde (enregistrement de l'historique)
// Ce hook est essentiel pour les fonctionnalitÃs de Timeline et de Trends.
func (r Risk) AfterSave(tx gorm.DB) (err error) {
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
