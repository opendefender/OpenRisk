// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ReportType is what a report is about. Six, matching the audiences that ask for
// them; each maps to one generator per format.
type ReportType string

const (
	ReportTypeExecutiveSummary      ReportType = "executive_summary"
	ReportTypeComplianceByFramework ReportType = "compliance_framework"
	ReportTypeBoard                 ReportType = "board"
	ReportTypeRiskRegister          ReportType = "risk_register"
	ReportTypeIncident              ReportType = "incident"
	ReportTypeAudit                 ReportType = "audit"
)

// AllReportTypes is the catalogue the configurator reads.
func AllReportTypes() []ReportType {
	return []ReportType{
		ReportTypeExecutiveSummary,
		ReportTypeComplianceByFramework,
		ReportTypeBoard,
		ReportTypeRiskRegister,
		ReportTypeIncident,
		ReportTypeAudit,
	}
}

// Valid reports whether the type is one the product generates.
func (t ReportType) Valid() bool {
	for _, k := range AllReportTypes() {
		if k == t {
			return true
		}
	}
	return false
}

// ReportFormat is the container the document is delivered in.
//
// The choice is not cosmetic: a PDF is what gets signed and filed, a DOCX is
// what a compliance officer edits before sending it on, and an XLSX is what an
// auditor filters and pivots. Offering only PDF forces people to retype.
type ReportFormat string

const (
	ReportFormatPDF  ReportFormat = "pdf"
	ReportFormatDOCX ReportFormat = "docx"
	ReportFormatXLSX ReportFormat = "xlsx"
)

// ParseReportFormat validates a format (empty → pdf).
func ParseReportFormat(s string) (ReportFormat, error) {
	switch ReportFormat(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return ReportFormatPDF, nil
	case ReportFormatPDF:
		return ReportFormatPDF, nil
	case ReportFormatDOCX:
		return ReportFormatDOCX, nil
	case ReportFormatXLSX:
		return ReportFormatXLSX, nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid format: %q (expected pdf, docx or xlsx)", s))
	}
}

// ContentType is the MIME type to serve the format as.
func (f ReportFormat) ContentType() string {
	switch f {
	case ReportFormatDOCX:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ReportFormatXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/pdf"
	}
}

// Extension is the filename suffix.
func (f ReportFormat) Extension() string { return "." + string(f) }

// ReportLocale is the language the DOCUMENT is written in.
//
// Deliberately separate from the interface language. A French-speaking
// compliance officer routinely produces an English report for a foreign
// regulator or a group parent; tying the document's language to the one they
// happen to read the app in forces them to switch the whole product to a
// language they do not want, generate, and switch back.
type ReportLocale string

const (
	ReportLocaleFR ReportLocale = "fr"
	ReportLocaleEN ReportLocale = "en"
)

// ParseReportLocale validates a document language (empty → fr).
func ParseReportLocale(s string) (ReportLocale, error) {
	switch ReportLocale(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return ReportLocaleFR, nil
	case ReportLocaleFR:
		return ReportLocaleFR, nil
	case ReportLocaleEN:
		return ReportLocaleEN, nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid report language: %q (expected fr or en)", s))
	}
}

// ReportRunState is the generation lifecycle — whether the bytes exist yet.
//
// Distinct from ReportLifecycle below, which is about whether the document is
// APPROVED. A report can be generated and still be a draft nobody has signed
// off; conflating the two would make "published" mean "finished rendering".
type ReportRunState string

const (
	ReportRunQueued    ReportRunState = "queued"
	ReportRunRunning   ReportRunState = "running"
	ReportRunSucceeded ReportRunState = "succeeded"
	ReportRunFailed    ReportRunState = "failed"
)

// Terminal reports whether the run will not change again.
func (s ReportRunState) Terminal() bool {
	return s == ReportRunSucceeded || s == ReportRunFailed
}

// ReportLifecycle is the editorial state: has a human accepted this document?
type ReportLifecycle string

const (
	ReportLifecycleDraft     ReportLifecycle = "draft"
	ReportLifecycleInReview  ReportLifecycle = "in_review"
	ReportLifecycleApproved  ReportLifecycle = "approved"
	ReportLifecyclePublished ReportLifecycle = "published"
)

// ParseReportLifecycle validates a lifecycle state.
func ParseReportLifecycle(s string) (ReportLifecycle, error) {
	switch ReportLifecycle(strings.TrimSpace(s)) {
	case ReportLifecycleDraft, ReportLifecycleInReview, ReportLifecycleApproved, ReportLifecyclePublished:
		return ReportLifecycle(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid lifecycle state: %q", s))
	}
}

// reportTransitions is the state machine, written once.
//
// Forward only, with two exceptions that reflect how documents actually move:
// a review can send a document back to draft (that is what a review is for), and
// an approved document can be withdrawn to draft if something is found before it
// is published. Once PUBLISHED it is frozen — it has left the building, and
// editing something people already hold is how two versions of "the" report end
// up in circulation. A new version is a new report, which is why Supersedes
// exists.
var reportTransitions = map[ReportLifecycle][]ReportLifecycle{
	ReportLifecycleDraft:     {ReportLifecycleInReview, ReportLifecycleApproved},
	ReportLifecycleInReview:  {ReportLifecycleDraft, ReportLifecycleApproved},
	ReportLifecycleApproved:  {ReportLifecyclePublished, ReportLifecycleDraft},
	ReportLifecyclePublished: {},
}

// CanTransitionTo reports whether the move is allowed, and says why when not.
func (s ReportLifecycle) CanTransitionTo(next ReportLifecycle) error {
	if s == next {
		return NewValidationError(fmt.Sprintf("the report is already %q", s))
	}
	for _, allowed := range reportTransitions[s] {
		if allowed == next {
			return nil
		}
	}
	if s == ReportLifecyclePublished {
		return NewValidationError(
			"a published report cannot be changed — generate a new version instead, so the document people already hold keeps meaning what it meant")
	}
	return NewValidationError(fmt.Sprintf("cannot move a report from %q to %q", s, next))
}

// Editable reports whether the document may still be deleted or regenerated.
func (s ReportLifecycle) Editable() bool { return s != ReportLifecyclePublished }

// Report is one generated document: its request, its bytes, its integrity hash
// and its editorial state.
type Report struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	Type   ReportType   `gorm:"type:varchar(40);not null;index" json:"type"`
	Format ReportFormat `gorm:"type:varchar(8);not null;default:'pdf'" json:"format"`
	Locale ReportLocale `gorm:"type:varchar(8);not null;default:'fr'" json:"locale"`

	// Template identifies the layout AND its version. Stored on the row, not
	// resolved at read time: a report regenerated a year later under a newer
	// template is a different document, and an approved one has to keep saying
	// which layout produced it.
	TemplateKey     string `gorm:"size:64;not null;default:''" json:"template_key"`
	TemplateVersion string `gorm:"size:16;not null;default:''" json:"template_version"`

	// Params carries the configurator's answers (period, scope, recipients…).
	// Opaque so a new report type needs no migration.
	Params datatypes.JSON `gorm:"type:jsonb" json:"params,omitempty"`

	Title string `gorm:"size:255;not null;default:''" json:"title"`

	RunState ReportRunState `gorm:"type:varchar(16);not null;default:'queued';index" json:"run_state"`
	// Progress is 0-100, published as the job advances so the client shows real
	// movement rather than an indeterminate spinner.
	Progress int    `gorm:"not null;default:0" json:"progress"`
	Step     string `gorm:"size:120;not null;default:''" json:"step"`
	Error    string `gorm:"type:text" json:"error,omitempty"`

	Lifecycle ReportLifecycle `gorm:"type:varchar(16);not null;default:'draft';index" json:"lifecycle"`

	Filename    string `gorm:"size:255;not null;default:''" json:"filename,omitempty"`
	ContentType string `gorm:"size:128;not null;default:''" json:"content_type,omitempty"`
	// Artifact is the rendered document. Never serialised — served by the
	// download endpoint, not embedded in a status response.
	Artifact  []byte `gorm:"type:bytea" json:"-"`
	SizeBytes int    `gorm:"not null;default:0" json:"size_bytes,omitempty"`

	// ContentHash is SHA-256 of the exact bytes served. It is what /verify
	// recomputes and what the download sends in X-Content-SHA256, so someone
	// holding the file can prove it is the one the platform produced and that
	// nobody has edited it since.
	ContentHash string `gorm:"size:64;not null;default:'';index" json:"content_hash,omitempty"`

	// ContentFingerprint is SHA-256 of what the report SAYS — the data that went
	// into it — and it is the value printed on the document itself.
	//
	// It has to be a different number from ContentHash, and the reason is
	// arithmetic rather than preference: a file cannot contain the hash of
	// itself, because printing the hash changes the bytes being hashed. An
	// earlier version of this code printed the hash of a first render and stored
	// the hash of a second, so the number on the page silently disagreed with the
	// number the API served — worse than printing nothing, because it looked
	// checkable and was not.
	//
	// So the two answer different questions. The fingerprint says "this is the
	// same content as that other copy" and survives a re-render in another
	// format; the hash says "these exact bytes are untampered". Both are exposed,
	// and /verify reports both.
	ContentFingerprint string `gorm:"size:64;not null;default:''" json:"content_fingerprint,omitempty"`

	// Version counts regenerations of the same report lineage, and Supersedes
	// points at the one this replaces. Together they are the version history, and
	// what makes a diff between two versions possible.
	Version    int        `gorm:"not null;default:1" json:"version"`
	Supersedes *uuid.UUID `gorm:"type:uuid;index" json:"supersedes,omitempty"`

	RequestedBy uuid.UUID  `gorm:"type:uuid;index" json:"requested_by"`
	ApprovedBy  *uuid.UUID `gorm:"type:uuid" json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// ---- Computed, never persisted ----

	// Comments are the review trail, loaded on the detail read.
	Comments []ReportComment `gorm:"-" json:"comments,omitempty"`
	// RequestedByEmail / ApprovedByEmail name the actors for display.
	RequestedByEmail string `gorm:"-" json:"requested_by_email,omitempty"`
	ApprovedByEmail  string `gorm:"-" json:"approved_by_email,omitempty"`
}

func (Report) TableName() string { return "reports" }

// AuditEntityType opts Report into the governance audit trail.
func (Report) AuditEntityType() string { return "report" }

// ComputeContentHash returns the hex SHA-256 of the artifact.
func ComputeContentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ShortFingerprint is the first 16 hex characters of the content fingerprint —
// what gets printed on the document. The full values are served by the API; a
// 64-character string across a page footer is something nobody reads or types.
func (r *Report) ShortFingerprint() string {
	if len(r.ContentFingerprint) < 16 {
		return r.ContentFingerprint
	}
	return r.ContentFingerprint[:16]
}

// VerifyIntegrity recomputes the hash over the stored bytes and reports whether
// it still matches what was recorded at generation time.
func (r *Report) VerifyIntegrity() bool {
	if r.ContentHash == "" || len(r.Artifact) == 0 {
		return false
	}
	return ComputeContentHash(r.Artifact) == r.ContentHash
}

// ReportComment is one remark in the review trail.
//
// Comments live on the report rather than in a generic discussion table because
// they are part of its record: "approved subject to the Q3 figures being
// restated" has to travel with the document it qualifies.
type ReportComment struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ReportID uuid.UUID `gorm:"type:uuid;not null;index" json:"report_id"`

	AuthorID uuid.UUID `gorm:"type:uuid;index" json:"author_id"`
	Body     string    `gorm:"type:text;not null" json:"body"`
	// Transition records the lifecycle move this comment accompanied, empty for a
	// plain remark. It is what turns a comment list into an audit trail: "who
	// approved this, and what did they say when they did".
	Transition ReportLifecycle `gorm:"type:varchar(16);not null;default:''" json:"transition,omitempty"`

	CreatedAt time.Time `json:"created_at"`

	AuthorEmail string `gorm:"-" json:"author_email,omitempty"`
}

func (ReportComment) TableName() string { return "report_comments" }

// ReportFilter narrows a listing. Zero value means "everything".
type ReportFilter struct {
	Type      ReportType
	Lifecycle ReportLifecycle
	Format    ReportFormat
	Limit     int
	Offset    int
	// Sort accepts "-created_at" (default) and "created_at".
	Sort string
}

// ReportRepository is the tenant-scoped persistence port. Every method takes a
// tenantID and must filter on it (RULE #2).
type ReportRepository interface {
	Create(ctx context.Context, r *Report) error
	Update(ctx context.Context, r *Report) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Report, error)
	List(ctx context.Context, tenantID uuid.UUID, f ReportFilter) ([]Report, int64, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error

	// Lineage returns every version of a report, newest first — the version
	// history, and the source of a version comparison.
	Lineage(ctx context.Context, tenantID, id uuid.UUID) ([]Report, error)

	AddComment(ctx context.Context, c *ReportComment) error
	ListComments(ctx context.Context, tenantID, reportID uuid.UUID) ([]ReportComment, error)

	// ClaimQueued atomically moves one queued report to running and returns it,
	// or (nil, nil) when there is nothing to do. Atomic because two workers
	// racing for the same row would render the document twice.
	ClaimQueued(ctx context.Context) (*Report, error)
}
