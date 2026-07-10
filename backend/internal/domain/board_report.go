// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BoardReportStatus is the human-in-the-loop lifecycle of a board report.
type BoardReportStatus string

const (
	// BoardReportDraft is a freshly generated report: fully editable, not yet
	// endorsed. Generation always produces a draft.
	BoardReportDraft BoardReportStatus = "draft"
	// BoardReportApproved is a report a human has validated. Approved reports are
	// frozen (no further narrative edits) so the diffused version is authoritative.
	BoardReportApproved BoardReportStatus = "approved"
)

// BoardReport is a tenant-scoped, monthly board-of-directors report. It snapshots
// the risk/compliance posture at generation time (so the PDF is reproducible even
// as the underlying data moves) plus an editable, non-technical narrative written
// by an Advisor (Claude when configured, otherwise a deterministic template).
//
// The flow is generate (draft) → edit → approve, all tenant-scoped.
type BoardReport struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	Title            string       `gorm:"size:255;not null;default:''" json:"title"`
	OrganizationName string       `gorm:"size:255;not null;default:''" json:"organization_name"` // snapshot at generation time
	PeriodLabel      string       `gorm:"size:100;not null;default:''" json:"period_label"`      // e.g. "Juillet 2026"
	Locale      string            `gorm:"size:5;not null;default:'fr'" json:"locale"`       // fr | en
	Status      BoardReportStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`

	// --- Posture snapshot (frozen at generation time) ---
	RisksCritical            int     `gorm:"not null;default:0" json:"risks_critical"`
	RisksHigh                int     `gorm:"not null;default:0" json:"risks_high"`
	RisksMedium              int     `gorm:"not null;default:0" json:"risks_medium"`
	RisksLow                 int     `gorm:"not null;default:0" json:"risks_low"`
	RisksTotal               int     `gorm:"not null;default:0" json:"risks_total"`
	FinancialExposureFCFA    int64   `gorm:"not null;default:0" json:"financial_exposure_fcfa"`
	OverallCompliancePercent float64 `gorm:"type:numeric(5,2);not null;default:0" json:"overall_compliance_percent"`
	// FrameworksSnapshot is a JSON array of the per-framework advancement at
	// generation time (name, version, percent, counts) — rendered in the PDF table.
	FrameworksSnapshot datatypes.JSON `gorm:"type:jsonb" json:"frameworks_snapshot"`

	// --- Narrative (editable while draft) ---
	ExecutiveSummary     string         `gorm:"type:text" json:"executive_summary"`
	RiskCommentary       string         `gorm:"type:text" json:"risk_commentary"`
	ComplianceCommentary string         `gorm:"type:text" json:"compliance_commentary"`
	FinancialCommentary  string         `gorm:"type:text" json:"financial_commentary"`
	Recommendations      pq.StringArray `gorm:"type:text[];default:'{}'" json:"recommendations"`

	// GeneratedByModel records provenance: "claude-opus-4-8" when the LLM wrote it,
	// or "template" when the deterministic fallback did.
	GeneratedByModel string `gorm:"size:50;not null;default:''" json:"generated_by_model"`

	// --- Audit ---
	CreatedBy  uuid.UUID  `gorm:"type:uuid;index" json:"created_by"`
	ApprovedBy *uuid.UUID `gorm:"type:uuid" json:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (BoardReport) TableName() string { return "board_reports" }

// BoardReportRepository is the port for board-report persistence. Every method is
// tenant-scoped: a tenant only ever sees or mutates its own reports.
//
// ABSOLUTE RULE: filter by tenant_id in the repository, never in the handler.
type BoardReportRepository interface {
	// Create persists a new (draft) report. report.TenantID MUST be set.
	Create(ctx context.Context, report *BoardReport) error
	// GetByID returns a report scoped to a tenant, or (nil, nil) if not found or
	// owned by another tenant.
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*BoardReport, error)
	// List returns a tenant's reports, most recent first.
	List(ctx context.Context, tenantID uuid.UUID) ([]BoardReport, error)
	// Update saves narrative/status changes. MUST filter by tenant_id AND id.
	Update(ctx context.Context, report *BoardReport) error
	// Delete soft-deletes a report scoped to a tenant.
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
}
