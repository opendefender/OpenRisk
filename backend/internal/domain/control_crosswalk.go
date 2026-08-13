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

// CrosswalkCoverage says how much of the target control the source control
// answers — the question the whole feature turns on.
//
// Two values, not the previous three (equivalent / partial / related). "Related"
// was unusable: it fed no decision, because thematic relatedness does not tell
// anyone whether their existing proof can be reused. What a compliance officer
// actually asks is binary — can I reuse this as it stands, or is there something
// left to demonstrate — and that is full versus partial.
type CrosswalkCoverage string

const (
	// CoverageFull: the same evidence satisfies both controls as written.
	CoverageFull CrosswalkCoverage = "full"
	// CoveragePartial: it contributes, but leaves something to demonstrate.
	CoveragePartial CrosswalkCoverage = "partial"
)

// ParseCrosswalkCoverage validates a coverage level (empty → full).
func ParseCrosswalkCoverage(s string) (CrosswalkCoverage, error) {
	switch CrosswalkCoverage(s) {
	case "":
		return CoverageFull, nil
	case CoverageFull, CoveragePartial:
		return CrosswalkCoverage(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid coverage: %q (expected full or partial)", s))
	}
}

// CrosswalkOrigin records who asserted the link.
//
// It matters because the two carry different authority. A curated link is the
// product's editorial judgement, shipped with the catalogs; a manual one is the
// tenant's own. Showing which is which is what lets someone review what the
// product claimed on their behalf instead of inheriting it silently.
type CrosswalkOrigin string

const (
	CrosswalkOriginCurated CrosswalkOrigin = "curated"
	CrosswalkOriginManual  CrosswalkOrigin = "manual"
)

// ControlCrosswalk is a tenant-scoped, undirected correspondence between two
// compliance controls that (normally) live in different frameworks.
//
// This is what lets the product answer the question that makes a second
// framework bearable: "how much of this do I already have?" A tenant importing
// SOC 2 with a mature ISO programme should be told, before they do anything,
// that a large fraction of it is already answered by proof they hold — and be
// able to see exactly which links produced that number, and why.
//
// The pair is stored order-independent-safe: the repository refuses a duplicate
// in either direction (A→B and B→A are the same crosswalk).
type ControlCrosswalk struct {
	// No gen_random_uuid() default: the id is assigned in Go, and a Postgres-only
	// default means the model cannot migrate under sqlite — which forces every
	// test to hand-write its schema, which is how three of them drifted from the
	// struct they were mirroring. Same fix as domain.AuditEvent.
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	SourceControlID uuid.UUID `gorm:"type:uuid;not null;index" json:"source_control_id"`
	TargetControlID uuid.UUID `gorm:"type:uuid;not null;index" json:"target_control_id"`

	Coverage CrosswalkCoverage `gorm:"type:varchar(16);not null;default:'full'" json:"coverage"`
	// Rationale says WHY the two correspond. Required, and not decoration: the
	// number it feeds is one an auditor will push back on, so the tenant has to be
	// able to read the reasoning, accept it, argue with it, or delete the link.
	Rationale string          `gorm:"type:text" json:"rationale"`
	Origin    CrosswalkOrigin `gorm:"type:varchar(16);not null;default:'manual';index" json:"origin"`

	CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Computed, NOT persisted — filled by the list use case so the UI can render
	// each side without extra round-trips.
	SourceCode          string `gorm:"-" json:"source_code,omitempty"`
	SourceName          string `gorm:"-" json:"source_name,omitempty"`
	SourceFrameworkID   string `gorm:"-" json:"source_framework_id,omitempty"`
	SourceFrameworkName string `gorm:"-" json:"source_framework_name,omitempty"`
	TargetCode          string `gorm:"-" json:"target_code,omitempty"`
	TargetName          string `gorm:"-" json:"target_name,omitempty"`
	TargetFrameworkID   string `gorm:"-" json:"target_framework_id,omitempty"`
	TargetFrameworkName string `gorm:"-" json:"target_framework_name,omitempty"`
}

func (ControlCrosswalk) TableName() string { return "control_crosswalks" }

// ControlCrosswalkRepository is the port for crosswalk persistence.
// Tenant-scoped throughout; a crosswalk owned by another tenant reads as absent.
type ControlCrosswalkRepository interface {
	Create(ctx context.Context, m *ControlCrosswalk) error
	// Exists reports whether a crosswalk between the two controls already exists
	// in EITHER direction for the tenant.
	Exists(ctx context.Context, tenantID, a, b uuid.UUID) (bool, error)
	// List returns every crosswalk for the tenant. If controlID is non-nil, only
	// crosswalks touching that control (as source OR target) are returned.
	List(ctx context.Context, tenantID uuid.UUID, controlID *uuid.UUID) ([]ControlCrosswalk, error)
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
	// ListByFramework returns every crosswalk with one foot in the framework —
	// the query behind inherited coverage.
	ListByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID) ([]ControlCrosswalk, error)
}
