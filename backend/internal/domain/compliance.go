// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ControlStatus represents the implementation state of a compliance control.
type ControlStatus string

const (
	ControlStatusNotImplemented ControlStatus = "not_implemented"
	ControlStatusInProgress     ControlStatus = "in_progress"
	ControlStatusImplemented    ControlStatus = "implemented"
	ControlStatusNotApplicable  ControlStatus = "not_applicable"
)

// ComplianceFramework is a tenant-scoped entity: each tenant owns its own
// frameworks. Examples: ISO 27001, SOC 2, NIST CSF, DORA, COBAC, BCEAO.
//
// Two tenants can each hold their own "ISO 27001 / 2022" instance, and deleting
// one tenant's framework never touches another's. Uniqueness of (name, version)
// is enforced PER TENANT (partial unique index, migration 0030), not globally.
type ComplianceFramework struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Version     string    `gorm:"size:50;not null;default:''" json:"version"`
	Description string    `gorm:"type:text" json:"description"`

	// Relations (loaded via Preload)
	Controls []ComplianceControl `gorm:"foreignKey:FrameworkID" json:"controls,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ComplianceFramework) TableName() string {
	return "compliance_frameworks"
}

// ComplianceControl is a tenant-scoped control linked to a global framework.
type ComplianceControl struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID      uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	FrameworkID   uuid.UUID `gorm:"type:uuid;not null;index" json:"framework_id"`
	ReferenceCode string    `gorm:"size:50;not null;default:''" json:"reference_code"` // e.g. "A.5.1.1"
	Name          string    `gorm:"size:255;not null" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	// SourceReference cites where this control comes from (e.g. "ISO/IEC 27001:2022, Annexe A, A.5.1"
	// or a COBAC/BCEAO circular article) — required for controls imported from a regulatory catalog,
	// optional for ad-hoc controls a tenant creates by hand.
	SourceReference string        `gorm:"size:255;not null;default:''" json:"source_reference"`
	Status          ControlStatus `gorm:"type:varchar(30);not null;default:'not_implemented'" json:"status"`

	// Relations
	Framework ComplianceFramework `gorm:"foreignKey:FrameworkID" json:"framework,omitempty"`
	Evidences []ControlEvidence   `gorm:"foreignKey:ControlID" json:"evidences,omitempty"`

	// EvidenceCount is a computed, non-persisted count of the control's active
	// evidences (gorm:"-": never a column). Populated by the list/get use cases so
	// the UI can show an evidence badge and enforce the "no implemented without a
	// proof" rule client-side without loading every evidence file.
	EvidenceCount int `gorm:"-" json:"evidence_count"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ComplianceControl) TableName() string {
	return "compliance_controls"
}

// ControlEvidence is a tenant-scoped evidence artifact linked to a control.
type ControlEvidence struct {
	ID          uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ControlID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"control_id"`
	Filename    string     `gorm:"size:255;not null;default:''" json:"filename"`
	URL         string     `gorm:"type:text;not null;default:''" json:"url"`
	Description string     `gorm:"type:text" json:"description"`
	UploadedBy  *uuid.UUID `gorm:"type:uuid" json:"uploaded_by"`

	// Relations
	Control ComplianceControl `gorm:"foreignKey:ControlID" json:"control,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ControlEvidence) TableName() string {
	return "control_evidences"
}
