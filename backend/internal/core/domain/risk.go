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

	Mitigations []Mitigation `gorm:"-" json:"mitigations,omitempty"`

	Assets []*Asset `gorm:"many2many:asset_risks;" json:"assets,omitempty"`
}

func (r *Risk) BeforeSave(tx *gorm.DB) (err error) {
	r.Score = r.Impact * r.Probability
	return
}

// AfterSave : Gère la logique après la sauvegarde (enregistrement de l'historique)
// Ce hook est essentiel pour les fonctionnalités de Timeline et de Trends.
func (r *Risk) AfterSave(tx *gorm.DB) (err error) {
	// 1. Définir le type de changement
	changeType := "UPDATE"
	if tx.Statement.SQL.String() == "" {
		// Une astuce simple pour dev: si SQL est vide, c'est probablement un nouvel enregistrement
		changeType = "CREATE"
	}

	// 2. Créer l'entrée d'historique (snapshot)
	history := RiskHistory{
		RiskID:      r.ID,
		Score:       r.Score,
		Impact:      r.Impact,
		Probability: r.Probability,
		Status:      r.Status,
		ChangedBy:   r.Owner, // Utilise le dernier Owner comme ChangedBy (simplification pour MVP)
		ChangeType:  changeType,
		CreatedAt:   time.Now(),
	}
	
	// 3. Sauvegarder le snapshot dans la table risk_histories
	// Nous utilisons la transaction courante (tx) pour garantir l'atomicité si c'est une transaction.
	return tx.Create(&history).Error
}