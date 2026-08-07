// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ReportKind is what a job renders. Each kind maps to one generator.
type ReportKind string

const (
	// ReportKindComplianceFramework renders the per-framework compliance PDF.
	ReportKindComplianceFramework ReportKind = "compliance_framework"
)

// Valid reports whether the kind has a generator behind it.
func (k ReportKind) Valid() bool {
	return k == ReportKindComplianceFramework
}

// ReportJobStatus is the job's lifecycle.
type ReportJobStatus string

const (
	ReportJobQueued    ReportJobStatus = "queued"
	ReportJobRunning   ReportJobStatus = "running"
	ReportJobSucceeded ReportJobStatus = "succeeded"
	ReportJobFailed    ReportJobStatus = "failed"
)

// Terminal reports whether the job will not change again.
func (s ReportJobStatus) Terminal() bool {
	return s == ReportJobSucceeded || s == ReportJobFailed
}

// ReportJob is one requested report and the artifact it produced.
//
// Reports are modelled as jobs rather than as a synchronous download because a
// report is a point-in-time snapshot with an address of its own. That address
// (/reports/jobs/:id) is what breaks the Compliance <-> Reports navigation loop:
// asking for a compliance report used to bounce the user to the Reports screen,
// whose Compliance tile bounced them back, with no artifact produced anywhere in
// between. Now the request creates a job and the user lands on that job.
//
// The rendered bytes are stored on the row so a re-download returns the document
// that was actually generated, not a fresh render of a register that has since
// moved on. Reports here are a few hundred KB; if a kind ever outgrows that,
// Artifact should become an object-store key rather than a blob.
type ReportJob struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	Kind   ReportKind      `gorm:"type:varchar(40);not null;index" json:"kind"`
	Status ReportJobStatus `gorm:"type:varchar(20);not null;default:'queued';index" json:"status"`

	// Params carries the kind's inputs (framework_id, locale, ...). Kept opaque
	// so a new report kind needs no migration.
	Params datatypes.JSON `gorm:"type:jsonb" json:"params,omitempty"`

	// Title is a human label resolved at generation time ("ISO/IEC 27001:2022"),
	// so the job list reads meaningfully without re-resolving every framework.
	Title string `json:"title"`

	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	// Artifact is the rendered document. Never serialised — it is served by the
	// download endpoint, not embedded in a JSON status response.
	Artifact  []byte `gorm:"type:bytea" json:"-"`
	SizeBytes int    `json:"size_bytes,omitempty"`

	// Error carries a user-safe failure reason (never a raw internal error).
	Error string `json:"error,omitempty"`

	RequestedBy uuid.UUID  `gorm:"type:uuid;index" json:"requested_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TableName pins the table name.
func (ReportJob) TableName() string { return "report_jobs" }

// AuditEntityType opts ReportJob into the governance audit trail.
func (ReportJob) AuditEntityType() string { return "report_job" }

// ReportJobRepository is the tenant-scoped persistence port for report jobs.
// Every method takes a tenantID and must filter on it (RULE #2).
type ReportJobRepository interface {
	Create(ctx context.Context, job *ReportJob) error
	// GetByID returns ErrNotFound when the job does not exist in this tenant —
	// a job id from another tenant is indistinguishable from a made-up one.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*ReportJob, error)
	List(ctx context.Context, tenantID uuid.UUID, limit int) ([]ReportJob, error)
	Update(ctx context.Context, job *ReportJob) error
}
