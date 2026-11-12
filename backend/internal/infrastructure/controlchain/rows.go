// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package controlchain persists the ADR 0008 control chain (Requirement ->
// Control -> Implementation -> Test -> Result -> Evidence) and reconciles it
// with the legacy compliance tables (#553, D-108).
//
// The row types here are the persistence shape of amendment A10; the canonical
// records live in internal/domain/canonical. Every row carries tenant_id, and
// every constraint migration 0085 declares is also declared as a check: or
// index tag, because a schema built by AutoMigrate alone (a compose boot never
// runs the SQL layer) must refuse the same rows.
package controlchain

import (
	"time"

	"github.com/google/uuid"
)

// Table names.
const (
	TableControls               = "controls"
	TableControlImplementations = "control_implementations"
	TableControlTests           = "control_tests"
	TableControlTestResults     = "control_test_results"
	TableControlGapFindings     = "control_gap_findings"
	TableCanonicalEdges         = "canonical_edges"
)

// Tables lists the six tables of migration 0085.
func Tables() []string {
	return []string{
		TableControls, TableControlImplementations, TableControlTests,
		TableControlTestResults, TableControlGapFindings, TableCanonicalEdges,
	}
}

// Models lists the row types for AutoMigrate.
func Models() []interface{} {
	return []interface{}{
		&ControlRow{}, &ControlImplementationRow{}, &ControlTestRow{},
		&ControlTestResultRow{}, &ControlGapFindingRow{}, &CanonicalEdgeRow{},
	}
}

// ControlRow is canonical.Control, persisted. Ids are assigned in Go (the split
// id is canonical.ControlIDFor), so there is no database default.
type ControlRow struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:ux_controls_identity,priority:1;index:idx_controls_tenant_deleted,priority:1"`
	Source        string     `gorm:"size:64;not null;uniqueIndex:ux_controls_identity,priority:2"`
	SourceID      string     `gorm:"size:255;not null;uniqueIndex:ux_controls_identity,priority:3"`
	SourceVersion *string    `gorm:"size:64"`
	Confidence    float64    `gorm:"type:numeric(3,2);not null;check:chk_controls_confidence,confidence >= 0 AND confidence <= 1"`
	Provenance    string     `gorm:"type:jsonb;not null"`
	ObservedAt    *time.Time `gorm:"type:timestamptz"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time `gorm:"index:idx_controls_tenant_deleted,priority:2"`

	Name                       string     `gorm:"size:255;not null"`
	Description                *string    `gorm:"type:text"`
	Objective                  *string    `gorm:"type:text"`
	OwnerID                    *uuid.UUID `gorm:"type:uuid"`
	AssigneeID                 *uuid.UUID `gorm:"type:uuid"`
	ReviewerID                 *uuid.UUID `gorm:"type:uuid"`
	Applicability              string     `gorm:"size:16;not null;check:chk_controls_applicability,applicability IN ('applicable', 'not_applicable')"`
	ApplicabilityJustification *string    `gorm:"type:text;check:chk_controls_not_applicable,applicability <> 'not_applicable' OR applicability_review_pending OR length(trim(COALESCE(applicability_justification, ''))) > 0"`
	// ApplicabilityReviewPending is A10.1: set only by the D7 split.
	ApplicabilityReviewPending bool   `gorm:"not null;default:false" json:"-"`
	DeclaredStatus             string `gorm:"size:20;not null;check:chk_controls_declared_status,declared_status IN ('not_implemented', 'in_progress', 'implemented')"`
	Tags                       string `gorm:"type:jsonb;not null;default:'[]'"`
}

// TableName implements gorm's Tabler.
func (ControlRow) TableName() string { return TableControls }

// ControlImplementationRow is canonical.ControlImplementation, persisted. The
// implemented_by edge is the control_id column (A10.4).
type ControlImplementationRow struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:ux_control_implementations_identity,priority:1;index:idx_control_implementations_control,priority:1"`
	Source        string     `gorm:"size:64;not null;uniqueIndex:ux_control_implementations_identity,priority:2"`
	SourceID      string     `gorm:"size:255;not null;uniqueIndex:ux_control_implementations_identity,priority:3"`
	SourceVersion *string    `gorm:"size:64"`
	Confidence    float64    `gorm:"type:numeric(3,2);not null;check:chk_control_implementations_confidence,confidence >= 0 AND confidence <= 1"`
	Provenance    string     `gorm:"type:jsonb;not null"`
	ObservedAt    *time.Time `gorm:"type:timestamptz"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time

	ControlID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_control_implementations_control,priority:2"`
	Description  string     `gorm:"type:text;not null"`
	Status       string     `gorm:"size:16;not null;check:chk_control_implementations_status,status IN ('planned', 'in_progress', 'in_place', 'retired')"`
	InPlaceSince *time.Time `gorm:"type:date"`
	OwnerID      *uuid.UUID `gorm:"type:uuid"`
	AssigneeID   *uuid.UUID `gorm:"type:uuid"`
	ReviewerID   *uuid.UUID `gorm:"type:uuid"`
}

// TableName implements gorm's Tabler.
func (ControlImplementationRow) TableName() string { return TableControlImplementations }

// ControlTestRow is canonical.ControlTest, persisted. The tested_by edge is the
// implementation_id column (A10.4).
type ControlTestRow struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:ux_control_tests_identity,priority:1;index:idx_control_tests_implementation,priority:1"`
	Source        string     `gorm:"size:64;not null;uniqueIndex:ux_control_tests_identity,priority:2"`
	SourceID      string     `gorm:"size:255;not null;uniqueIndex:ux_control_tests_identity,priority:3"`
	SourceVersion *string    `gorm:"size:64"`
	Confidence    float64    `gorm:"type:numeric(3,2);not null;check:chk_control_tests_confidence,confidence >= 0 AND confidence <= 1"`
	Provenance    string     `gorm:"type:jsonb;not null"`
	ObservedAt    *time.Time `gorm:"type:timestamptz"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time

	ImplementationID uuid.UUID `gorm:"type:uuid;not null;index:idx_control_tests_implementation,priority:2"`
	Name             string    `gorm:"size:255;not null"`
	Checks           string    `gorm:"type:text;not null"`
	Method           string    `gorm:"size:16;not null;check:chk_control_tests_method,method IN ('manual', 'automated')"`
	Procedure        *string   `gorm:"type:text;check:chk_control_tests_declares_itself,(method <> 'manual' OR length(trim(COALESCE(procedure, ''))) > 0) AND (method <> 'automated' OR length(trim(COALESCE(collector, ''))) > 0)"`
	Collector        *string   `gorm:"size:255"`
	Cadence          *string   `gorm:"size:32"`
	PassCriteria     string    `gorm:"type:text;not null"`
}

// TableName implements gorm's Tabler.
func (ControlTestRow) TableName() string { return TableControlTests }

// ControlTestResultRow is canonical.ControlTestResult, persisted. It is
// append-only, so it has no updated_at and no deleted_at, and it carries the
// test's tenant_id (A10.5).
type ControlTestResultRow struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID `gorm:"type:uuid;not null;index:idx_control_test_results_test,priority:1;uniqueIndex:ux_control_test_results_identity,priority:1"`
	TestID         uuid.UUID `gorm:"type:uuid;not null;index:idx_control_test_results_test,priority:2;uniqueIndex:ux_control_test_results_identity,priority:2"`
	Outcome        string    `gorm:"size:16;not null;check:chk_control_test_results_outcome,outcome IN ('pass', 'fail', 'inconclusive')"`
	ObservedAt     time.Time `gorm:"type:timestamptz;not null"`
	RecordedAt     time.Time `gorm:"type:timestamptz;not null;index:idx_control_test_results_test,priority:3"`
	Source         string    `gorm:"size:64;not null;uniqueIndex:ux_control_test_results_identity,priority:3"`
	SourceID       string    `gorm:"size:255;not null;uniqueIndex:ux_control_test_results_identity,priority:4"`
	Confidence     float64   `gorm:"type:numeric(3,2);not null;check:chk_control_test_results_confidence,confidence >= 0 AND confidence <= 1"`
	ExecutedByKind string    `gorm:"size:16;not null;check:chk_control_test_results_executed_by,executed_by_kind IN ('user', 'system', 'vendor_contact')"`
	ExecutedByID   string    `gorm:"size:255;not null"`
	Note           *string   `gorm:"type:text"`
}

// TableName implements gorm's Tabler.
func (ControlTestResultRow) TableName() string { return TableControlTestResults }

// ControlGapFindingRow is canonical.Finding of kind control_gap, persisted in
// its own table (A10.6).
type ControlGapFindingRow struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_control_gap_findings_identity,priority:1;index:idx_control_gap_findings_test,priority:1;index:idx_control_gap_findings_status,priority:1"`
	Source        string    `gorm:"size:64;not null;uniqueIndex:ux_control_gap_findings_identity,priority:2"`
	SourceID      string    `gorm:"size:255;not null;uniqueIndex:ux_control_gap_findings_identity,priority:3"`
	SourceVersion *string   `gorm:"size:64"`
	Confidence    float64   `gorm:"type:numeric(3,2);not null;check:chk_control_gap_findings_confidence,confidence >= 0 AND confidence <= 1"`
	Provenance    string    `gorm:"type:jsonb;not null"`
	ObservedAt    time.Time `gorm:"type:timestamptz;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Kind            string     `gorm:"size:16;not null;default:'control_gap';check:chk_control_gap_findings_kind,kind = 'control_gap'"`
	TestID          uuid.UUID  `gorm:"type:uuid;not null;index:idx_control_gap_findings_test,priority:2"`
	Title           string     `gorm:"size:512;not null"`
	Description     *string    `gorm:"type:text"`
	Severity        string     `gorm:"size:16;not null;check:chk_control_gap_findings_severity,severity IN ('critical', 'high', 'medium', 'low', 'info')"`
	Status          string     `gorm:"size:16;not null;index:idx_control_gap_findings_status,priority:2;check:chk_control_gap_findings_status,status IN ('open', 'triaged', 'in_remediation', 'remediated', 'accepted', 'false_positive')"`
	FirstObservedAt time.Time  `gorm:"type:timestamptz;not null"`
	SLADueAt        *time.Time `gorm:"column:sla_due_at;type:timestamptz"`
	RemediatedAt    *time.Time `gorm:"type:timestamptz"`
	RemediationHint *string    `gorm:"type:text"`
}

// TableName implements gorm's Tabler.
func (ControlGapFindingRow) TableName() string { return TableControlGapFindings }

// CanonicalEdgeRow is one stored relationship (A10.3). Qualifier is the nil
// uuid when the edge has none, so the unique key is total. An edge has no
// deleted_at: it is live when both its ends are, and it is hard-deleted.
type CanonicalEdgeRow struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_canonical_edges_key,priority:1;index:idx_canonical_edges_to,priority:1"`
	FromType   string    `gorm:"size:32;not null;uniqueIndex:ux_canonical_edges_key,priority:2"`
	FromID     uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_canonical_edges_key,priority:3"`
	Verb       string    `gorm:"size:24;not null;uniqueIndex:ux_canonical_edges_key,priority:4;index:idx_canonical_edges_to,priority:4;check:chk_canonical_edges_verb,verb IN ('addressed_by', 'applies_to', 'evidenced_by', 'mitigated_by', 'raised_by', 'indicates')"`
	ToType     string    `gorm:"size:32;not null;uniqueIndex:ux_canonical_edges_key,priority:5;index:idx_canonical_edges_to,priority:2"`
	ToID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_canonical_edges_key,priority:6;index:idx_canonical_edges_to,priority:3"`
	Qualifier  uuid.UUID `gorm:"type:uuid;not null;default:'00000000-0000-0000-0000-000000000000';uniqueIndex:ux_canonical_edges_key,priority:7"`
	Attributes string    `gorm:"type:jsonb;not null;default:'{}'"`
	Source     string    `gorm:"size:64;not null"`
	SourceID   string    `gorm:"size:255;not null"`
	Confidence float64   `gorm:"type:numeric(3,2);not null;check:chk_canonical_edges_confidence,confidence >= 0 AND confidence <= 1"`
	Provenance string    `gorm:"type:jsonb;not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName implements gorm's Tabler.
func (CanonicalEdgeRow) TableName() string { return TableCanonicalEdges }
