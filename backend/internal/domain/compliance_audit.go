// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// =============================================================================
// Compliance audits ("Audits" — planification, exécution, historique)
// =============================================================================

// AuditType is the nature of a compliance audit.
type AuditType string

const (
	AuditTypeInternal      AuditType = "internal"      // self-assessment
	AuditTypeExternal      AuditType = "external"      // third-party audit
	AuditTypeCertification AuditType = "certification" // certification body audit (e.g. ISO 27001 stage 2)
	AuditTypeSurveillance  AuditType = "surveillance"  // periodic surveillance audit
)

// ParseAuditType validates an audit type (empty → internal).
func ParseAuditType(s string) (AuditType, error) {
	if s == "" {
		return AuditTypeInternal, nil
	}
	switch AuditType(s) {
	case AuditTypeInternal, AuditTypeExternal, AuditTypeCertification, AuditTypeSurveillance:
		return AuditType(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid audit type: %q", s))
	}
}

// AuditStatus is the lifecycle state of a compliance audit.
type AuditStatus string

const (
	AuditStatusPlanned    AuditStatus = "planned"
	AuditStatusInProgress AuditStatus = "in_progress"
	AuditStatusCompleted  AuditStatus = "completed"
	AuditStatusCancelled  AuditStatus = "cancelled"
)

// ParseAuditStatus validates an audit status (empty → planned).
func ParseAuditStatus(s string) (AuditStatus, error) {
	if s == "" {
		return AuditStatusPlanned, nil
	}
	switch AuditStatus(s) {
	case AuditStatusPlanned, AuditStatusInProgress, AuditStatusCompleted, AuditStatusCancelled:
		return AuditStatus(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid audit status: %q", s))
	}
}

// ComplianceAudit is a tenant-scoped audit: planned, executed, then archived as
// history. It may target a single framework (FrameworkID set) or the whole
// compliance program (FrameworkID nil).
type ComplianceAudit struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	Title       string     `gorm:"size:255;not null" json:"title"`
	FrameworkID *uuid.UUID `gorm:"type:uuid;index" json:"framework_id"` // nil = program-wide
	Type        AuditType  `gorm:"type:varchar(24);not null;default:'internal'" json:"type"`
	Status      AuditStatus `gorm:"type:varchar(24);not null;default:'planned';index" json:"status"`

	Auditor string `gorm:"size:255" json:"auditor"` // auditor name or firm
	Scope   string `gorm:"type:text" json:"scope"`
	Summary string `gorm:"type:text" json:"summary"` // conclusions / findings summary

	// ComplianceScore is an optional posture snapshot (0–100) recorded when the
	// audit is completed.
	ComplianceScore float64 `gorm:"type:numeric(5,2)" json:"compliance_score"`

	ScheduledStart *time.Time `json:"scheduled_start"`
	ScheduledEnd   *time.Time `json:"scheduled_end"`
	CompletedAt    *time.Time `json:"completed_at"`

	CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ComplianceAudit) TableName() string { return "compliance_audits" }

// ComplianceAuditRepository is the port for audit persistence. Every method is
// tenant-scoped: a resource belonging to another tenant is returned as not found.
type ComplianceAuditRepository interface {
	CreateAudit(ctx context.Context, a *ComplianceAudit) error
	GetAuditByID(ctx context.Context, id, tenantID uuid.UUID) (*ComplianceAudit, error)
	ListAudits(ctx context.Context, tenantID uuid.UUID) ([]ComplianceAudit, error)
	UpdateAudit(ctx context.Context, a *ComplianceAudit) error
	DeleteAudit(ctx context.Context, id, tenantID uuid.UUID) error
}

// =============================================================================
// Remediation plans ("Plans de remédiation")
// =============================================================================

// RemediationPriority ranks the urgency of a remediation action.
type RemediationPriority string

const (
	RemediationPriorityLow      RemediationPriority = "low"
	RemediationPriorityMedium   RemediationPriority = "medium"
	RemediationPriorityHigh     RemediationPriority = "high"
	RemediationPriorityCritical RemediationPriority = "critical"
)

// ParseRemediationPriority validates a priority (empty → medium).
func ParseRemediationPriority(s string) (RemediationPriority, error) {
	if s == "" {
		return RemediationPriorityMedium, nil
	}
	switch RemediationPriority(s) {
	case RemediationPriorityLow, RemediationPriorityMedium, RemediationPriorityHigh, RemediationPriorityCritical:
		return RemediationPriority(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid remediation priority: %q", s))
	}
}

// RemediationStatus is the lifecycle state of a remediation plan.
type RemediationStatus string

const (
	RemediationStatusOpen       RemediationStatus = "open"
	RemediationStatusInProgress RemediationStatus = "in_progress"
	RemediationStatusCompleted  RemediationStatus = "completed"
	RemediationStatusCancelled  RemediationStatus = "cancelled"
)

// ParseRemediationStatus validates a status (empty → open).
func ParseRemediationStatus(s string) (RemediationStatus, error) {
	if s == "" {
		return RemediationStatusOpen, nil
	}
	switch RemediationStatus(s) {
	case RemediationStatusOpen, RemediationStatusInProgress, RemediationStatusCompleted, RemediationStatusCancelled:
		return RemediationStatus(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid remediation status: %q", s))
	}
}

// RemediationPlan is a tenant-scoped action to close a compliance gap. It is
// linked to the control it remediates (ControlID) and, optionally, to the audit
// that surfaced the gap (AuditID). This is compliance-specific remediation,
// distinct from risk Mitigations (which hang off a Risk).
type RemediationPlan struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`

	ControlID   *uuid.UUID `gorm:"type:uuid;index" json:"control_id"`   // the gap being remediated
	FrameworkID *uuid.UUID `gorm:"type:uuid;index" json:"framework_id"` // denormalised for filtering
	AuditID     *uuid.UUID `gorm:"type:uuid;index" json:"audit_id"`     // origin audit, if any

	Priority RemediationPriority `gorm:"type:varchar(16);not null;default:'medium'" json:"priority"`
	Status   RemediationStatus   `gorm:"type:varchar(24);not null;default:'open';index" json:"status"`

	AssignedTo  *uuid.UUID `gorm:"type:uuid;index" json:"assigned_to"`
	DueDate     *time.Time `json:"due_date"`
	CompletedAt *time.Time `json:"completed_at"`

	CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Computed, NOT persisted — the linked control's reference code/name, filled
	// by the list use case for a readable UI without an extra round-trip.
	ControlCode string `gorm:"-" json:"control_code,omitempty"`
	ControlName string `gorm:"-" json:"control_name,omitempty"`
}

func (RemediationPlan) TableName() string { return "remediation_plans" }

// RemediationFilter narrows a remediation list query. Zero-value fields are
// ignored (no filter on that dimension).
type RemediationFilter struct {
	ControlID   *uuid.UUID
	FrameworkID *uuid.UUID
	AuditID     *uuid.UUID
	Status      RemediationStatus
}

// RemediationPlanRepository is the port for remediation-plan persistence.
// Tenant-scoped throughout.
type RemediationPlanRepository interface {
	CreateRemediation(ctx context.Context, r *RemediationPlan) error
	GetRemediationByID(ctx context.Context, id, tenantID uuid.UUID) (*RemediationPlan, error)
	ListRemediations(ctx context.Context, tenantID uuid.UUID, filter RemediationFilter) ([]RemediationPlan, error)
	UpdateRemediation(ctx context.Context, r *RemediationPlan) error
	DeleteRemediation(ctx context.Context, id, tenantID uuid.UUID) error
}
