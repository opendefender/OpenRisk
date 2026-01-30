package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AssetCriticality string

const (
	CriticalityLow      AssetCriticality = "LOW"
	CriticalityMedium   AssetCriticality = "MEDIUM"
	CriticalityHigh     AssetCriticality = "HIGH"
	CriticalityCritical AssetCriticality = "CRITICAL"
)

type Asset struct {
	ID          uuid.UUID        gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	Name        string           gorm:"not null" json:"name"
	Type        string           json:"type" // Server, Laptop, Database, SaaS
	Criticality AssetCriticality gorm:"default:'MEDIUM'" json:"criticality"
	Owner       string           json:"owner"

	// Relation Many-to-Many avec Risk
	Risks []Risk gorm:"manymany:risk_assets;" json:"risks,omitempty"

	Source     string gorm:"default:'MANUAL'" json:"source" // MANUAL ou OPENASSET
	ExternalID string json:"external_id"

	CreatedAt time.Time      json:"created_at"
	UpdatedAt time.Time      json:"updated_at"
	DeletedAt gorm.DeletedAt gorm:"index" json:"-"
}
