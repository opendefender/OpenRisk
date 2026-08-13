// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EvidenceType is the nature of the artifact, not the file's MIME type.
//
// An auditor asks "what kind of proof is this?", not "is it a PDF?" — a policy
// document, a screen capture, a configuration export, a signed attestation and a
// log extract carry very different evidential weight, and the same PDF container
// can hold any of them.
type EvidenceType string

const (
	EvidenceTypeDocument      EvidenceType = "document"
	EvidenceTypeCapture       EvidenceType = "capture"
	EvidenceTypeConfiguration EvidenceType = "configuration"
	EvidenceTypeAttestation   EvidenceType = "attestation"
	EvidenceTypeLog           EvidenceType = "log"
)

// ParseEvidenceType validates a type (empty → document).
func ParseEvidenceType(s string) (EvidenceType, error) {
	if s == "" {
		return EvidenceTypeDocument, nil
	}
	switch EvidenceType(s) {
	case EvidenceTypeDocument, EvidenceTypeCapture, EvidenceTypeConfiguration,
		EvidenceTypeAttestation, EvidenceTypeLog:
		return EvidenceType(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid evidence type: %q", s))
	}
}

// EvidenceSource records where the artifact came from — a human upload, a
// connected system, or an automated collector. It is provenance, and it is what
// lets an auditor weigh a screenshot someone pasted against a configuration
// pulled straight from the cloud account.
type EvidenceSource string

const (
	EvidenceSourceManual      EvidenceSource = "manual"
	EvidenceSourceIntegration EvidenceSource = "integration"
	EvidenceSourceScanner     EvidenceSource = "scanner"
	EvidenceSourceAutomation  EvidenceSource = "automation"
)

// ParseEvidenceSource validates a source (empty → manual).
func ParseEvidenceSource(s string) (EvidenceSource, error) {
	if s == "" {
		return EvidenceSourceManual, nil
	}
	switch EvidenceSource(s) {
	case EvidenceSourceManual, EvidenceSourceIntegration, EvidenceSourceScanner, EvidenceSourceAutomation:
		return EvidenceSource(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid evidence source: %q", s))
	}
}

// EvidenceReview is the human verdict on an artifact: has someone competent
// looked at it and accepted it as proof?
//
// This is deliberately NOT the same field as the freshness state below. A
// rejected artifact is rejected whether or not it has expired, and an accepted
// artifact still goes stale on its own schedule. Collapsing the two into one
// column is how evidence registers end up claiming a control is covered by a
// screenshot an auditor threw out eighteen months ago.
type EvidenceReview string

const (
	EvidenceReviewPending  EvidenceReview = "pending"
	EvidenceReviewAccepted EvidenceReview = "accepted"
	EvidenceReviewRejected EvidenceReview = "rejected"
)

// ParseEvidenceReview validates a review verdict (empty → accepted).
//
// Accepted is the default because the person uploading a document into their own
// compliance register is, in the common case, asserting it. Making them then
// approve their own upload would be ceremony, not control; a tenant that wants
// four-eyes on evidence has the governance approval engine for that.
func ParseEvidenceReview(s string) (EvidenceReview, error) {
	if s == "" {
		return EvidenceReviewAccepted, nil
	}
	switch EvidenceReview(s) {
	case EvidenceReviewPending, EvidenceReviewAccepted, EvidenceReviewRejected:
		return EvidenceReview(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid evidence review status: %q", s))
	}
}

// EvidenceStatus is the effective state an auditor reads off the register. It is
// DERIVED, never stored: see Evidence.EffectiveStatus.
type EvidenceStatus string

const (
	EvidenceStatusValid    EvidenceStatus = "valid"
	EvidenceStatusExpiring EvidenceStatus = "expiring_soon"
	EvidenceStatusExpired  EvidenceStatus = "expired"
	EvidenceStatusRejected EvidenceStatus = "rejected"
	EvidenceStatusPending  EvidenceStatus = "pending"
)

// EvidenceExpiryWindow is how far ahead an artifact is called "expiring soon".
//
// Thirty days is the shortest window in which a real organisation can get a
// penetration test rebooked, a policy re-approved by a committee that meets
// monthly, or an attestation re-signed by a supplier. A shorter warning is a
// warning nobody can act on.
const EvidenceExpiryWindow = 30 * 24 * time.Hour

// Evidence is a reusable proof artifact in a tenant's evidence library.
//
// The defining property is that it is NOT owned by one control. The same SOC 2
// bridge letter, ISO certificate or hardening baseline export answers a dozen
// controls across several frameworks, and re-uploading it per control is both
// how registers rot (twelve copies, one of them refreshed) and the single
// biggest source of busywork in a compliance programme. One artifact, N links.
type Evidence struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	Title       string       `gorm:"size:255;not null;default:''" json:"title"`
	Type        EvidenceType `gorm:"type:varchar(20);not null;default:'document';index" json:"type"`
	Description string       `gorm:"type:text" json:"description"`

	// FileRef is the storage key of the artifact's bytes, served only through the
	// download endpoint. Empty for evidence that is a statement rather than a file
	// (an attestation recorded inline, a link to a system of record).
	FileRef  string `gorm:"type:text;not null;default:''" json:"file_ref"`
	Filename string `gorm:"size:255;not null;default:''" json:"filename"`
	// ExternalURL points at evidence that lives in another system of record. Kept
	// distinct from FileRef so the UI never offers a download for something the
	// product does not hold the bytes of.
	ExternalURL string `gorm:"type:text;not null;default:''" json:"external_url"`

	// CollectedAt is when the proof was *taken*, which is not when the row was
	// created: a January access review uploaded in March is January evidence, and
	// dating it March would misstate the control's coverage.
	CollectedAt time.Time  `gorm:"not null;index" json:"collected_at"`
	ValidUntil  *time.Time `gorm:"index" json:"valid_until"`
	CollectedBy *uuid.UUID `gorm:"type:uuid;index" json:"collected_by"`

	Review       EvidenceReview `gorm:"type:varchar(16);not null;default:'accepted';index" json:"review"`
	ReviewNote   string         `gorm:"type:text" json:"review_note"`
	ReviewedBy   *uuid.UUID     `gorm:"type:uuid" json:"reviewed_by"`
	ReviewedAt   *time.Time     `json:"reviewed_at"`
	Source       EvidenceSource `gorm:"type:varchar(20);not null;default:'manual';index" json:"source"`
	SourceDetail string         `gorm:"size:255;not null;default:''" json:"source_detail"`

	// Ownership — who answers for this proof, who must refresh it, who validates
	// it. Same embedded block as Risk/Mitigation/Incident/RemediationPlan.
	Ownership `gorm:"embedded"`

	// ReminderSentAt stamps the last expiry reminder so a worker that runs on a
	// cadence cannot spam the owner every tick. Stamped BEFORE the notification
	// goes out, for the same reason the mitigation due worker does it: a duplicate
	// reminder is noise, a reminder sent twice because the send succeeded and the
	// stamp failed is a bug that trains people to ignore the channel.
	ReminderSentAt *time.Time `json:"reminder_sent_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// ---- Computed, never persisted (gorm:"-") ----

	// Status is the effective state; filled by the use cases on read so the API
	// and the UI never re-derive it (and never disagree about it).
	Status EvidenceStatus `gorm:"-" json:"status"`
	// DaysUntilExpiry is negative once expired, nil when the evidence never expires.
	DaysUntilExpiry *int `gorm:"-" json:"days_until_expiry,omitempty"`
	// ControlIDs is the set of controls this artifact currently answers — the
	// `control_id[]` of the spec, materialised from the link table on read.
	ControlIDs []uuid.UUID `gorm:"-" json:"control_ids"`
	// Controls carries a light projection (code, name, framework) for display.
	Controls []EvidenceControlRef `gorm:"-" json:"controls,omitempty"`
	// CollectedByEmail resolves the actor for display; empty when unresolvable.
	CollectedByEmail string `gorm:"-" json:"collected_by_email,omitempty"`
}

func (Evidence) TableName() string { return "evidences" }

// OwnershipBlock implements OwnedEntity.
func (e *Evidence) OwnershipBlock() *Ownership { return &e.Ownership }

// AuditEntityType opts Evidence into the automatic audit trail.
func (Evidence) AuditEntityType() string { return "evidence" }

// EvidenceControlRef is a display projection of one linked control.
type EvidenceControlRef struct {
	ControlID     uuid.UUID `json:"control_id"`
	ReferenceCode string    `json:"reference_code"`
	Name          string    `json:"name"`
	FrameworkID   uuid.UUID `json:"framework_id"`
	FrameworkName string    `json:"framework_name"`
}

// EffectiveStatus derives the state an auditor should read, at time now.
//
// Derived rather than stored on purpose. A stored status is a claim that was
// true when someone last wrote it; evidence expiry is a function of the clock,
// and a register whose "valid" column only updates when a worker happens to run
// is a register that tells auditors a control is covered by expired proof.
func (e *Evidence) EffectiveStatus(now time.Time) EvidenceStatus {
	// A human verdict outranks the calendar in both directions: rejected proof is
	// not proof however fresh, and unreviewed proof is not yet proof either.
	switch e.Review {
	case EvidenceReviewRejected:
		return EvidenceStatusRejected
	case EvidenceReviewPending:
		return EvidenceStatusPending
	}
	if e.ValidUntil == nil {
		return EvidenceStatusValid
	}
	if !e.ValidUntil.After(now) {
		return EvidenceStatusExpired
	}
	if e.ValidUntil.Before(now.Add(EvidenceExpiryWindow)) {
		return EvidenceStatusExpiring
	}
	return EvidenceStatusValid
}

// Covers reports whether this artifact currently substantiates a control — the
// single predicate the whole module answers to.
//
// Expired and rejected evidence does NOT cover. This is the rule that makes the
// difference between a document store and a GRC tool: a control whose only proof
// went stale must fall back out of "implemented", not sit there claiming coverage
// on the strength of a file that was uploaded once.
func (e *Evidence) Covers(now time.Time) bool {
	s := e.EffectiveStatus(now)
	return s == EvidenceStatusValid || s == EvidenceStatusExpiring
}

// DaysUntil returns whole calendar days until expiry (negative once past), or nil
// when the evidence carries no expiry.
func (e *Evidence) DaysUntil(now time.Time) *int {
	if e.ValidUntil == nil {
		return nil
	}
	d := int(e.ValidUntil.Sub(now).Hours() / 24)
	return &d
}

// Decorate fills the computed fields. Called by every read path so the API shape
// is identical no matter which use case produced the row.
func (e *Evidence) Decorate(now time.Time) {
	e.Status = e.EffectiveStatus(now)
	e.DaysUntilExpiry = e.DaysUntil(now)
	if e.ControlIDs == nil {
		e.ControlIDs = []uuid.UUID{}
	}
}

// EvidenceControlLink attaches one artifact to one control. The join table is
// what makes evidence reusable; it is tenant-stamped as well as keyed so a
// mis-scoped query can never bridge two tenants' registers.
type EvidenceControlLink struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	// The pair is unique on the MODEL, not only in the migration: schemas here are
	// built by AutoMigrate as often as by golang-migrate, and a constraint that
	// exists in only one of them is a constraint that silently is not there.
	EvidenceID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_evidence_control_pair" json:"evidence_id"`
	ControlID  uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_evidence_control_pair" json:"control_id"`
	// Note records why this artifact answers this particular control — the same
	// certificate justifies A.5.1 and CC6.1 for different reasons, and an auditor
	// asks about the reason, not the file.
	Note      string     `gorm:"type:text" json:"note"`
	LinkedBy  *uuid.UUID `gorm:"type:uuid" json:"linked_by"`
	CreatedAt time.Time  `json:"created_at"`
}

func (EvidenceControlLink) TableName() string { return "evidence_control_links" }

// EvidenceFilter narrows a library listing. Zero value means "everything".
type EvidenceFilter struct {
	Type      EvidenceType
	Review    EvidenceReview
	ControlID *uuid.UUID
	// FrameworkID narrows to evidence linked to any control of that framework.
	FrameworkID *uuid.UUID
	Search      string
	// ExpiringBefore selects artifacts whose ValidUntil falls before this instant
	// (the reminder worker's query, and the "expiring soon" tab).
	ExpiringBefore *time.Time
	Limit          int
	Offset         int
}

// EvidenceRepository is the port for the evidence library. Tenant-scoped
// throughout: an artifact owned by another tenant reads back as not found.
type EvidenceRepository interface {
	Create(ctx context.Context, e *Evidence) error
	Update(ctx context.Context, e *Evidence) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Evidence, error)
	List(ctx context.Context, tenantID uuid.UUID, f EvidenceFilter) ([]Evidence, int64, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error

	// Link attaches an artifact to a control. Idempotent: re-linking the same
	// pair is a no-op rather than a duplicate row.
	Link(ctx context.Context, link *EvidenceControlLink) error
	Unlink(ctx context.Context, tenantID, evidenceID, controlID uuid.UUID) error
	// ListLinks returns every link for a set of evidences (used to materialise
	// ControlIDs in one query rather than one per row).
	ListLinks(ctx context.Context, tenantID uuid.UUID, evidenceIDs []uuid.UUID) ([]EvidenceControlLink, error)
	// ListByControl returns the artifacts linked to one control.
	ListByControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]Evidence, error)

	// CountCoveringByFramework returns, per control of a framework, how many
	// artifacts CURRENTLY cover it (expired and rejected excluded — that is the
	// whole point). One grouped query, no N+1.
	CountCoveringByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID, now time.Time) (map[uuid.UUID]int, error)
	// ControlsWithCoverage returns the set of controls in a framework that have at
	// least one covering artifact. Powers the "missing evidence" view and the
	// crosswalk coverage estimate.
	ControlsWithCoverage(ctx context.Context, tenantID, frameworkID uuid.UUID, now time.Time) (map[uuid.UUID]bool, error)

	// ListExpiring returns artifacts whose expiry falls within the window and that
	// have not been reminded since their last change — the worker's query.
	ListExpiring(ctx context.Context, now time.Time, window time.Duration, limit int) ([]Evidence, error)
	// MarkReminded stamps the reminder time.
	MarkReminded(ctx context.Context, id uuid.UUID, at time.Time) error
}
