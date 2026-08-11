// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MitigationStatus string

const (
	MitigationPlanned    MitigationStatus = "PLANNED"
	MitigationInProgress MitigationStatus = "IN_PROGRESS"
	MitigationReview     MitigationStatus = "REVIEW"
	MitigationDone       MitigationStatus = "DONE"
	MitigationCancelled  MitigationStatus = "CANCELLED"
)

type MitigationPriority string

const (
	PriorityLow      MitigationPriority = "low"
	PriorityMedium   MitigationPriority = "medium"
	PriorityHigh     MitigationPriority = "high"
	PriorityCritical MitigationPriority = "critical"
)

// UUIDArray is a PostgreSQL JSONB array of UUIDs
type UUIDArray []uuid.UUID

func (a UUIDArray) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *UUIDArray) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return gorm.ErrInvalidData
	}
	return json.Unmarshal(bytes, &a)
}

// Mitigation represents a mitigation plan for a risk
type Mitigation struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	RiskID   uuid.UUID `gorm:"type:uuid;not null;index" json:"risk_id"`

	// Core fields
	Title       string             `gorm:"size:255;not null" json:"title"`
	Description string             `gorm:"type:text" json:"description"`
	Status      MitigationStatus   `gorm:"type:varchar(20);default:'PLANNED'" json:"status"`
	Priority    MitigationPriority `gorm:"type:varchar(20);default:'medium'" json:"priority"`

	// Ownership — responsable / exécutant / validateur. Same embedded block as
	// Risk, Incident, RemediationPlan and ControlEvidence.
	Ownership `gorm:"embedded"`

	// Deprecated: multi-user assignment (JSONB array). Superseded by
	// Ownership.AssigneeID, which migration 0044 backfills from its first
	// element. Kept so existing readers keep working.
	AssignedTo UUIDArray `gorm:"type:jsonb;default:'[]'::jsonb" json:"assigned_to"`

	// Progress: 0-100, ALWAYS COMPUTED, never accepted from a client.
	// See ComputeMitigationProgress for the rule. It is persisted only so the
	// board can sort and filter on it; every mutation recomputes it server-side.
	Progress int `gorm:"default:0;check:progress >= 0 AND progress <= 100" json:"progress"`

	// Reminder bookkeeping for the D-7 / D-1 due-date nudges. Each is stamped
	// once, so a sweep that runs every hour does not send the same reminder
	// twenty-four times. Cleared when the due date moves.
	ReminderD7SentAt *time.Time `json:"reminder_d7_sent_at,omitempty"`
	ReminderD1SentAt *time.Time `json:"reminder_d1_sent_at,omitempty"`

	// Lifecycle tracking
	CreatedBy  uuid.UUID  `gorm:"type:uuid;not null;index" json:"created_by"`
	ApprovedBy *uuid.UUID `gorm:"type:uuid;index" json:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at"`

	// Source: manual|scanner|cti|ai (using domain.RiskSource shared enum)
	Source         RiskSource `gorm:"type:varchar(20);default:'manual'" json:"source"`
	AutoDetectedAt *time.Time `json:"auto_detected_at"`

	// Link to scanner config if auto-detected
	ScannerConfigID *uuid.UUID `gorm:"type:uuid;index" json:"scanner_config_id"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Legacy fields for backwards compatibility
	OrganizationID   uuid.UUID  `gorm:"index" json:"organization_id"`
	Assignee         string     `json:"assignee"` // Legacy: email or UserID
	Cost             int        `gorm:"default:1" json:"cost"`
	MitigationTime   int        `gorm:"default:1" json:"mitigation_time"`
	DueDate          *time.Time `json:"due_date"`
	WeightedPriority float64    `gorm:"-" json:"weighted_priority"`

	// Relations
	Risk       *Risk                 `json:"risk,omitempty" gorm:"foreignKey:ID;references:RiskID"`
	SubActions []MitigationSubAction `json:"sub_actions,omitempty" gorm:"foreignKey:MitigationID;constraint:OnDelete:CASCADE"`
}

// OwnershipBlock implements OwnedEntity.
func (m *Mitigation) OwnershipBlock() *Ownership { return &m.Ownership }

// ComputeMitigationProgress is the ONE rule for how far along a plan is.
//
// It is a function of the sub-actions, not a number anyone types:
//
//	with sub-actions    → completed / total, as a percentage
//	without sub-actions → the coarse status: planned 0, in progress 50, done 100
//
// The fallback exists because a plan with no checklist still has to report
// something, and "0 %" for a plan somebody marked DONE is the bug being fixed
// ("progression bloquée à 0 %"). A cancelled plan reports 0: it is not
// progress, it is abandonment.
//
// Pure and total: no clock, no database, no error path.
func ComputeMitigationProgress(status MitigationStatus, total, completed int) int {
	if total > 0 {
		if completed < 0 {
			completed = 0
		}
		if completed > total {
			completed = total
		}
		return completed * 100 / total
	}
	switch status {
	case MitigationDone:
		return 100
	case MitigationInProgress, MitigationReview:
		return 50
	default: // PLANNED, CANCELLED, or anything unrecognised
		return 0
	}
}

// IsOverdue reports whether the plan blew its due date without finishing.
// A done or cancelled plan is never late — there is nothing left to be late for.
func (m *Mitigation) IsOverdue(now time.Time) bool {
	if m == nil || m.DueDate == nil {
		return false
	}
	if m.Status == MitigationDone || m.Status == MitigationCancelled {
		return false
	}
	return now.After(*m.DueDate)
}

// DaysUntilDue is the signed day count to the deadline (negative = overdue),
// and false when the plan has no due date. This is what the badge renders.
func (m *Mitigation) DaysUntilDue(now time.Time) (int, bool) {
	if m == nil || m.DueDate == nil {
		return 0, false
	}
	// Compare calendar days, not instants: "due tomorrow" should not flip to
	// "due today" because of the time of day the page was loaded.
	due := time.Date(m.DueDate.Year(), m.DueDate.Month(), m.DueDate.Day(), 0, 0, 0, 0, m.DueDate.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, m.DueDate.Location())
	return int(due.Sub(today).Hours() / 24), true
}

// DueReminderOffsets are the deadlines at which an assignee is nudged, in days
// before the due date. The spec asks for J-7 and J-1.
var DueReminderOffsets = []int{7, 1}

// DueReminderDue reports which reminder (7 or 1) should be sent now, and false
// when none is. A plan that is finished, cancelled, has no due date, or has
// already had that reminder sent is never nudged again.
//
// The rule is "on or past the threshold, and not yet sent", not "exactly on the
// day": a sweep that misses a tick (deploy, outage) must still send the nudge
// rather than skip it silently.
func (m *Mitigation) DueReminderDue(now time.Time) (int, bool) {
	if m == nil || m.DueDate == nil {
		return 0, false
	}
	if m.Status == MitigationDone || m.Status == MitigationCancelled {
		return 0, false
	}
	days, ok := m.DaysUntilDue(now)
	if !ok {
		return 0, false
	}
	// D-1 is checked first: a plan discovered late should get the more urgent
	// nudge, not the one it already blew past.
	if days <= 1 && m.ReminderD1SentAt == nil {
		return 1, true
	}
	if days <= 7 && m.ReminderD7SentAt == nil {
		return 7, true
	}
	return 0, false
}

// MarkReminderSent stamps the reminder so it is not repeated.
func (m *Mitigation) MarkReminderSent(offset int, at time.Time) {
	if m == nil {
		return
	}
	stamp := at
	switch offset {
	case 1:
		m.ReminderD1SentAt = &stamp
		// Reaching D-1 without ever sending D-7 (a plan created inside the
		// window) still closes D-7: sending it afterwards would be noise.
		if m.ReminderD7SentAt == nil {
			m.ReminderD7SentAt = &stamp
		}
	case 7:
		m.ReminderD7SentAt = &stamp
	}
}

// ClearReminders forgets the sent stamps — called when the due date moves, so a
// postponed deadline nudges again on the new schedule.
func (m *Mitigation) ClearReminders() {
	if m == nil {
		return
	}
	m.ReminderD7SentAt = nil
	m.ReminderD1SentAt = nil
}
