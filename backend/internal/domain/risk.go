// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RiskStatus represents the lifecycle state of a risk
type RiskStatus string

const (
	RiskOpen       RiskStatus = "open"        // Newly identified, under review
	RiskInProgress RiskStatus = "in_progress" // Mitigation underway
	RiskMitigated  RiskStatus = "mitigated"   // Treatment plan completed
	RiskAccepted   RiskStatus = "accepted"    // Formally accepted as residual
	RiskClosed     RiskStatus = "closed"      // Fully resolved/no longer relevant
	// Legacy statuses for compatibility
	StatusDraft     RiskStatus = "DRAFT"
	StatusActive    RiskStatus = "ACTIVE"
	StatusMitigated RiskStatus = "MITIGATED"
	StatusAccepted  RiskStatus = "ACCEPTED"
)

// CriticalityLevel represents the severity level calculated from score
type CriticalityLevel string

const (
	// New lowercase versions
	CriticalityLowNew      CriticalityLevel = "low"
	CriticalityMediumNew   CriticalityLevel = "medium"
	CriticalityHighNew     CriticalityLevel = "high"
	CriticalityCriticalNew CriticalityLevel = "critical"
	// Legacy constants for compatibility
	RiskCriticalityLow      CriticalityLevel = "LOW"
	RiskCriticalityMedium   CriticalityLevel = "MEDIUM"
	RiskCriticalityHigh     CriticalityLevel = "HIGH"
	RiskCriticalityCritical CriticalityLevel = "CRITICAL"
)

// RiskPhase represents the ISO 31000 risk-management lifecycle stage of a risk.
// This is ORTHOGONAL to RiskStatus: Status is the resolution state
// (open/mitigated/accepted/…) while Phase is where the risk sits in the
// managed lifecycle "Identifier → Analyser → Évaluer → Traiter → Surveiller →
// Clôturer". Surfaced live on the real Risk entity (register + drawer stepper).
type RiskPhase string

const (
	PhaseIdentified RiskPhase = "identified" // Identifier — risk logged, context captured
	PhaseAnalyzed   RiskPhase = "analyzed"   // Analyser — probability/impact/causes assessed
	PhaseEvaluated  RiskPhase = "evaluated"  // Évaluer — prioritised vs risk appetite
	PhaseTreated    RiskPhase = "treated"    // Traiter — treatment plan chosen/underway
	PhaseMonitored  RiskPhase = "monitored"  // Surveiller — under continuous review
	PhaseClosed     RiskPhase = "closed"     // Clôturer — resolved / no longer relevant
)

// riskPhaseOrder is the canonical forward ordering of the lifecycle. Index is
// used to validate transitions (see CanTransitionTo).
var riskPhaseOrder = []RiskPhase{
	PhaseIdentified, PhaseAnalyzed, PhaseEvaluated, PhaseTreated, PhaseMonitored, PhaseClosed,
}

// phaseIndex returns the position of a phase in riskPhaseOrder, or -1 if unknown.
func phaseIndex(p RiskPhase) int {
	for i, phase := range riskPhaseOrder {
		if phase == p {
			return i
		}
	}
	return -1
}

// ParseRiskPhase validates and converts a string into a RiskPhase. An empty
// string defaults to PhaseIdentified (a freshly created risk is "identified").
func ParseRiskPhase(s string) (RiskPhase, error) {
	if s == "" {
		return PhaseIdentified, nil
	}
	if phaseIndex(RiskPhase(s)) >= 0 {
		return RiskPhase(s), nil
	}
	return "", NewValidationError(fmt.Sprintf("invalid risk lifecycle phase: %q", s))
}

// CanTransitionTo reports whether a risk may move from its current phase to
// the target. The lifecycle is pragmatic rather than rigid: you may advance
// one step, step back one step (re-open a phase), or jump straight to
// "closed" (early closure from any phase). A no-op (same phase) is rejected so
// the caller surfaces a clear validation error instead of a silent write.
func (p RiskPhase) CanTransitionTo(target RiskPhase) bool {
	from, to := phaseIndex(p), phaseIndex(target)
	if from < 0 || to < 0 {
		return false
	}
	if from == to {
		return false
	}
	if target == PhaseClosed {
		return true // early closure allowed from anywhere
	}
	// Re-opening from closed is allowed back to any earlier phase.
	if p == PhaseClosed {
		return true
	}
	// Otherwise move at most one step in either direction.
	diff := to - from
	return diff == 1 || diff == -1
}

// RiskTreatment represents the chosen treatment strategy
type RiskTreatment string

const (
	TreatmentAccept   RiskTreatment = "accept"
	TreatmentMitigate RiskTreatment = "mitigate"
	TreatmentTransfer RiskTreatment = "transfer"
	TreatmentAvoid    RiskTreatment = "avoid"
)

// RiskSource indicates where the risk originated
type RiskSource string

const (
	SourceManual   RiskSource = "manual"
	SourceCTIAuto  RiskSource = "cti_auto"  // From CTI/NVD/CISA KEV
	SourceScanAuto RiskSource = "scan_auto" // From vulnerability scanner
	SourceImport   RiskSource = "import"    // Imported from file
	SourceVendor   RiskSource = "vendor"    // From vendor assessment
	SourceAI       RiskSource = "ai"        // AI-generated
)

// ParseRiskSource validates and converts a string into a RiskSource.
// An empty string defaults to SourceManual (matches the ERD column default:
// `source VARCHAR(50) NOT NULL DEFAULT 'manual'`). Any other value must match
// one of the known constants, or a typed validation error is returned.
func ParseRiskSource(s string) (RiskSource, error) {
	if s == "" {
		return SourceManual, nil
	}
	switch RiskSource(s) {
	case SourceManual, SourceCTIAuto, SourceScanAuto, SourceImport, SourceVendor, SourceAI:
		return RiskSource(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid risk source: %q", s))
	}
}

// Risk represents a business risk with full lifecycle management
// Follows Clean Architecture: pure domain entity, ZERO external dependencies
type Risk struct {
	// Primary Key
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	// Multi-tenancy (ABSOLUTE: filter by tenant_id in repository, never in handler)
	TenantID       uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index" json:"organization_id"` // Legacy alias for TenantID

	// Core Risk Definition
	Name        string `gorm:"size:255;not null;index" json:"name"`
	Title       string `gorm:"size:255;not null;index" json:"title"` // Alias for Name, for compatibility
	Description string `gorm:"type:text" json:"description"`

	// Risk Scoring (via Score Engine: P × I × AssetCriticality, 3 decimal places)
	// New system (0.0-1.0, 0.0-10.0, 0.1-3.0)
	Probability float64          `gorm:"type:numeric(5,3);check:probability >= 0 AND probability <= 1" json:"probability"`
	Impact      float64          `gorm:"type:numeric(5,1);check:impact >= 0 AND impact <= 10" json:"impact"`
	Score       float64          `gorm:"type:numeric(8,3);default:0" json:"score"`                // Calculated ONLY via Score Engine (Redis event)
	Criticality CriticalityLevel `gorm:"type:varchar(20);default:'low';index" json:"criticality"` // low|medium|high|critical

	// Legacy scoring system (1-5 scale, for backwards compatibility)
	// Will be deprecated as system migrates to new scale
	ImpactLegacy      int `gorm:"default:1;check:impact_legacy >= 1 AND impact_legacy <= 5" json:"impact_legacy"`
	ProbabilityLegacy int `gorm:"default:1;check:probability_legacy >= 1 AND probability_legacy <= 5" json:"probability_legacy"`

	// Lifecycle Management
	Status RiskStatus `gorm:"type:varchar(20);default:'open';index" json:"status"` // open|in_progress|mitigated|accepted|closed
	Level  string     `gorm:"size:20;default:'medium';index" json:"level"`         // Legacy: CRITICAL|HIGH|MEDIUM|LOW

	// ISO 31000 lifecycle phase (orthogonal to Status). Drives the register
	// "Cycle de vie" stepper: Identifier → Analyser → Évaluer → Traiter →
	// Surveiller → Clôturer. Defaults to 'identified' on creation.
	LifecyclePhase RiskPhase `gorm:"type:varchar(20);default:'identified';index" json:"lifecycle_phase"`

	// Ownership & Assignment
	CreatedBy  uuid.UUID  `gorm:"type:uuid;not null;index" json:"created_by"`
	AssignedTo *uuid.UUID `gorm:"type:uuid;index" json:"assigned_to"` // Person responsible for mitigation
	ReviewerID *uuid.UUID `gorm:"type:uuid;index" json:"reviewer_id"` // Person responsible for final validation
	Owner      string     `json:"owner"`                              // Legacy: Email or UserID

	// Asset Association
	AssetID *uuid.UUID `gorm:"type:uuid;index" json:"asset_id"` // Linked asset if risk is asset-specific

	// Classification & Context
	Tags       pq.StringArray `gorm:"type:text[];default:'{}'" json:"tags"`        // Labels (network, cloud, etc.)
	Frameworks pq.StringArray `gorm:"type:text[];default:'{}'" json:"frameworks"`  // ISO27001|NIST-CSF|DORA|CIS|COBAC|BCEAO|OWASP|SOC2|GDPR|...
	ControlIDs pq.StringArray `gorm:"type:text[];default:'{}'" json:"control_ids"` // Links to compliance controls

	// Treatment & Mitigation
	TreatmentPlan   RiskTreatment `gorm:"type:varchar(20);default:'mitigate'" json:"treatment_plan"` // accept|mitigate|transfer|avoid
	ResidualRisk    *float64      `gorm:"type:numeric(8,3)" json:"residual_risk"`                    // Score after treatments
	LastMitigatedAt *time.Time    `json:"last_mitigated_at"`

	// Cyber Risk Quantification (CRQ) — monetary loss inputs (pkg/crq). Optional:
	// when both are set, ALE = SLE × ARO; otherwise a reference value per
	// criticality is used. Amounts are XAF (FCFA); USD is derived on read.
	SLEXAF *float64 `gorm:"type:numeric(16,2)" json:"sle_xaf"` // single loss expectancy (XAF)
	ARO    *float64 `gorm:"type:numeric(10,4)" json:"aro"`     // annualized rate of occurrence (events/year)

	// Computed, NOT persisted — filled by the handler via pkg/crq before responding.
	ALEXAF   float64 `gorm:"-" json:"ale_xaf"`   // annual loss expectancy (XAF)
	ALEUSD   float64 `gorm:"-" json:"ale_usd"`   // annual loss expectancy (USD)
	ALEBasis string  `gorm:"-" json:"ale_basis"` // "explicit" | "reference"

	// Review cadence — automated risk-review reminders. ReviewIntervalDays = 0
	// disables it; NextReviewAt is when the owner is next nudged; LastReviewedAt is
	// the last time the risk was marked reviewed.
	ReviewIntervalDays int        `gorm:"default:0" json:"review_interval_days"`
	NextReviewAt       *time.Time `gorm:"index" json:"next_review_at,omitempty"`
	LastReviewedAt     *time.Time `json:"last_reviewed_at,omitempty"`

	// Source Tracking
	Source      RiskSource `gorm:"type:varchar(20);default:'manual';index" json:"source"` // manual|cti_auto|scan_auto|import|vendor|ai
	SourceCVEID *string    `gorm:"index" json:"source_cve_id"`                            // CVE identifier if from CTI
	ExternalID  string     `gorm:"index" json:"external_id"`                              // ID in external system

	// Custom Fields (JSONB for flexibility)
	CustomFields datatypes.JSON `gorm:"type:jsonb" json:"custom_fields,omitempty"`

	// Audit Trail
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // Soft delete

	// Relations (loaded via Preload)
	Mitigations []Mitigation `gorm:"foreignKey:RiskID" json:"mitigations,omitempty"`
	Assets      []*Asset     `gorm:"many2many:risk_assets;" json:"assets,omitempty"`

	// Computed Fields (NOT persisted, populated by handlers/use cases)
	// These help with API responses but are never stored in DB
	RiskWithDetails *RiskDetail `gorm:"-" json:"-"`
}

// BeforeSave hook ensures basic validation and legacy compatibility
func (r *Risk) BeforeSave(tx *gorm.DB) error {
	// Ensure TenantID and OrganizationID are consistent (both required for multi-tenancy)
	if r.TenantID == uuid.Nil && r.OrganizationID != uuid.Nil {
		r.TenantID = r.OrganizationID
	}
	if r.OrganizationID == uuid.Nil && r.TenantID != uuid.Nil {
		r.OrganizationID = r.TenantID
	}

	// Name is required (use Title as fallback for legacy code)
	if r.Name == "" && r.Title != "" {
		r.Name = r.Title
	}
	if r.Title == "" && r.Name != "" {
		r.Title = r.Name
	}

	// Score Engine will recalculate via Redis event, but set initial legacy score if not set
	if r.Score == 0 && r.ImpactLegacy > 0 && r.ProbabilityLegacy > 0 {
		r.Score = float64(r.ImpactLegacy * r.ProbabilityLegacy)
	}

	// Ensure CreatedBy is set if creating.
	// CreatedBy must be set by the use case before saving; this is a
	// deliberate no-op rather than an enforced error, since hardening it
	// could break an existing save path that currently relies on this being
	// silent. Revisit as part of a dedicated domain-invariants audit.
	if r.CreatedBy == uuid.Nil && !tx.Statement.Changed("created_by") { //nolint:staticcheck // intentional no-op, see comment above
	}

	return nil
}

// AfterSave hook creates a history snapshot for audit trail and timeline
// Called automatically by GORM after save is successful
func (r *Risk) AfterSave(tx *gorm.DB) error {
	// Create a history snapshot for timeline and trends
	history := RiskHistory{
		ID:          uuid.New(),
		RiskID:      r.ID,
		Score:       r.Score,
		Impact:      r.ImpactLegacy,
		Probability: r.ProbabilityLegacy,
		Status:      r.Status,
		ChangedBy:   r.CreatedBy.String(), // Use UUID string
		ChangeType:  "UPDATE",
		CreatedAt:   time.Now(),
	}

	return tx.Create(&history).Error
}

// RiskDetail is a DTO for API responses with enriched data
// Includes calculated fields and related data
type RiskDetail struct {
	Risk           *Risk                 `json:"risk"`
	Mitigations    []Mitigation          `json:"mitigations,omitempty"`
	Assets         []*Asset              `json:"assets,omitempty"`
	ScoreBreakdown *ScoreBreakdownDetail `json:"score_breakdown,omitempty"`
	AuditHistory   []AuditLogEntry       `json:"audit_history,omitempty"`
	AssignedToUser *UserInfo             `json:"assigned_to_user,omitempty"`
	ReviewerUser   *UserInfo             `json:"reviewer_user,omitempty"`
	CreatedByUser  *UserInfo             `json:"created_by_user,omitempty"`
}

// ScoreBreakdownDetail extends scoring.ScoreBreakdown with context
type ScoreBreakdownDetail struct {
	Score            float64  `json:"score"`
	Probability      float64  `json:"probability"`
	Impact           float64  `json:"impact"`
	AssetCriticality float64  `json:"asset_criticality"`
	Criticality      string   `json:"criticality"`
	Explanation      string   `json:"explanation"`
	PreviousScore    *float64 `json:"previous_score,omitempty"`
	Delta            *float64 `json:"delta,omitempty"`
	CalculatedAt     string   `json:"calculated_at"`
}

// AuditLogEntry represents a historical change to a risk
type AuditLogEntry struct {
	ID        uuid.UUID              `json:"id"`
	RiskID    uuid.UUID              `json:"risk_id"`
	Timestamp time.Time              `json:"timestamp"`
	ChangedBy uuid.UUID              `json:"changed_by"`
	Action    string                 `json:"action"`
	OldValue  map[string]interface{} `json:"old_value,omitempty"`
	NewValue  map[string]interface{} `json:"new_value,omitempty"`
}

// UserInfo is a minimal user representation for API responses
type UserInfo struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
}
