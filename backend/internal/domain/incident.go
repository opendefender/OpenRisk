package domain

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Incident represents a security incident
type Incident struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	TenantID       string         `gorm:"index" json:"tenant_id"`
	Title          string         `gorm:"index" json:"title"`
	Description    string         `gorm:"type:text" json:"description"`
	IncidentType   string         `json:"incident_type"`            // vulnerability, breach, attack, data_loss, etc
	Severity       string         `gorm:"index" json:"severity"`    // critical, high, medium, low
	Status         string         `gorm:"index" json:"status"`      // open, investigating, resolved, closed
	Source         string         `json:"source"`                   // internal, external, third_party
	ExternalID     string         `gorm:"index" json:"external_id"` // external system ref (TheHive, OpenCTI, etc)
	ReportedBy     string         `json:"reported_by"`              // user who reported
	AssignedTo     string         `json:"assigned_to"`              // assigned team member
	RiskID         *uint          `gorm:"index" json:"risk_id"`     // linked risk
	ImpactedAssets datatypes.JSON `gorm:"type:jsonb" json:"impacted_assets"`
	Timeline       datatypes.JSON `gorm:"type:jsonb" json:"timeline"` // array of events
	Resolution     string         `gorm:"type:text" json:"resolution"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// IncidentTimeline represents an event in incident timeline
type IncidentTimeline struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	IncidentID uint           `gorm:"index" json:"incident_id"`
	Incident   Incident       `json:"incident,omitempty"`
	EventType  string         `json:"event_type"` // status_change, assignment, comment, action
	Message    string         `gorm:"type:text" json:"message"`
	Metadata   datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	CreatedBy  string         `json:"created_by"`
	CreatedAt  time.Time      `json:"created_at"`
}

// IncidentAction represents mitigation action for incident
type IncidentAction struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	IncidentID  uint      `gorm:"index" json:"incident_id"`
	Incident    Incident  `json:"incident,omitempty"`
	Title       string    `json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	AssignedTo  string    `json:"assigned_to"`
	DueDate     time.Time `json:"due_date"`
	Status      string    `json:"status"`   // pending, in_progress, completed
	Priority    string    `json:"priority"` // critical, high, medium, low
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IncidentCreateRequest represents incident creation request
type IncidentCreateRequest struct {
	Title          string   `json:"title" binding:"required"`
	Description    string   `json:"description" binding:"required"`
	IncidentType   string   `json:"incident_type" binding:"required"`
	Severity       string   `json:"severity" binding:"required"`
	Source         string   `json:"source" binding:"required"`
	ReportedBy     string   `json:"reported_by" binding:"required"`
	RiskID         *uint    `json:"risk_id"`
	ImpactedAssets []string `json:"impacted_assets"`
}

// IncidentUpdateRequest represents incident update request
type IncidentUpdateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Severity    string `json:"severity"`
	AssignedTo  string `json:"assigned_to"`
	Resolution  string `json:"resolution"`
}

// IncidentQuery filters for incident queries
type IncidentQuery struct {
	Status   string
	Severity string
	RiskID   *uint
	Type     string
	Limit    int
	Offset   int
}
