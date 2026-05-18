package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CompletionSource string

const (
	CompletionManual  CompletionSource = "manual"
	CompletionScanner CompletionSource = "scanner"
	CompletionAI      CompletionSource = "ai"
)

// MitigationSubAction represents a sub-task/checklist item within a mitigation plan
type MitigationSubAction struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MitigationID uuid.UUID `gorm:"type:uuid;not null;index" json:"mitigation_id"`
	
	// Core fields
	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`
	
	// Completion tracking
	Completed   bool       `gorm:"default:false" json:"completed"`
	CompletedAt *time.Time `json:"completed_at"`
	CompletedBy *uuid.UUID `gorm:"type:uuid;index" json:"completed_by"` // nil if auto-detected
	
	// Source of completion: manual|scanner|ai
	CompletedSource *CompletionSource `gorm:"type:varchar(20)" json:"completed_source"`
	
	// Auto-detection tracking (scanner ran and detected fix)
	AutoDetectedAt *time.Time `json:"auto_detected_at"`
	
	// Dependency management: this action depends on DependsOn
	DependsOn *uuid.UUID `gorm:"type:uuid;index" json:"depends_on"`
	
	// Ordering for UI display
	Order int `gorm:"default:0" json:"order"`
	
	// Due date
	DueDate *time.Time `json:"due_date"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MitigationSubAction) TableName() string { return "mitigation_subactions" }

