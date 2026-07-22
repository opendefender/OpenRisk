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

// MappingRelation qualifies how strongly two controls correspond across
// frameworks — a crosswalk ("cross-mapping") between e.g. ISO 27001 A.5.1 and
// NIST CSF GV.PO.
type MappingRelation string

const (
	MappingRelationEquivalent MappingRelation = "equivalent" // the two controls demand essentially the same thing
	MappingRelationPartial    MappingRelation = "partial"    // overlapping but not fully equivalent
	MappingRelationRelated    MappingRelation = "related"    // thematically related
)

// ParseMappingRelation validates a relation (empty → equivalent).
func ParseMappingRelation(s string) (MappingRelation, error) {
	if s == "" {
		return MappingRelationEquivalent, nil
	}
	switch MappingRelation(s) {
	case MappingRelationEquivalent, MappingRelationPartial, MappingRelationRelated:
		return MappingRelation(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid mapping relation: %q", s))
	}
}

// ControlMapping is a tenant-scoped, undirected crosswalk between two compliance
// controls that (normally) live in different frameworks. It lets the product show
// that satisfying one control corresponds to coverage in another framework.
//
// The pair is stored order-independent-safe: the repository refuses a duplicate
// in either direction (A→B and B→A are the same mapping).
type ControlMapping struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	SourceControlID uuid.UUID `gorm:"type:uuid;not null;index" json:"source_control_id"`
	TargetControlID uuid.UUID `gorm:"type:uuid;not null;index" json:"target_control_id"`

	Relation MappingRelation `gorm:"type:varchar(16);not null;default:'equivalent'" json:"relation"`
	Note     string          `gorm:"type:text" json:"note"`

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

func (ControlMapping) TableName() string { return "control_mappings" }

// ControlMappingRepository is the port for crosswalk persistence. Tenant-scoped
// throughout; a mapping owned by another tenant reads back as not found.
type ControlMappingRepository interface {
	Create(ctx context.Context, m *ControlMapping) error
	// Exists reports whether a mapping between the two controls already exists in
	// EITHER direction for the tenant.
	Exists(ctx context.Context, tenantID, a, b uuid.UUID) (bool, error)
	// List returns every mapping for the tenant. If controlID is non-nil, only
	// mappings touching that control (as source OR target) are returned.
	List(ctx context.Context, tenantID uuid.UUID, controlID *uuid.UUID) ([]ControlMapping, error)
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
}
