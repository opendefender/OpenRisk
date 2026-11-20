// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

	// CatalogKey records which regulatory catalog (pkg/compliance) this framework
	// was imported from, empty for one built by hand.
	//
	// It exists so the product can tell what a framework IS, not just what it was
	// named. Curated crosswalks are defined between catalogs, so without this the
	// only way to know that a framework called "SOC 2 (v2)" holds AICPA criteria
	// would be to guess from its title — and inherited coverage would silently
	// stop working the moment somebody renamed one.
	CatalogKey string `gorm:"size:64;not null;default:'';index" json:"catalog_key"`

	// CatalogContentVersion and CatalogCrosswalksVersion are the content version
	// of the pack and of the crosswalks registry this framework was last imported
	// at (ADR 0013 D3). "" means imported before content versions existed: the API
	// reports it as unknown, never as "1.0.0". Written only by the catalogue
	// import (ComplianceRepository.ApplyCatalogImport); read-only in the API.
	CatalogContentVersion    string `gorm:"size:32;not null;default:''" json:"catalog_content_version"`
	CatalogCrosswalksVersion string `gorm:"size:32;not null;default:''" json:"catalog_crosswalks_version"`
	// CatalogPendingKinds is a JSON array, '[]' until the pack upgrade exists
	// (ADR 0013 D4). Never serialised.
	CatalogPendingKinds string `gorm:"type:jsonb;not null;default:'[]'" json:"-"`

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
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	// tenant: struct tag only, no query. The tag adds tenant_id (leading) to the pack unique index.
	TenantID      uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:ux_compliance_controls_pack_item,priority:1,where:pack_key <> ''" json:"tenant_id"`
	FrameworkID   uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:ux_compliance_controls_pack_item,priority:2,where:pack_key <> ''" json:"framework_id"`
	ReferenceCode string    `gorm:"size:50;not null;default:''" json:"reference_code"` // e.g. "A.5.1.1"
	Name          string    `gorm:"size:255;not null" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	// SourceReference cites where this control comes from (e.g. "ISO/IEC 27001:2022, Annexe A, A.5.1"
	// or a COBAC/BCEAO circular article) — required for controls imported from a regulatory catalog,
	// optional for ad-hoc controls a tenant creates by hand.
	SourceReference string        `gorm:"size:255;not null;default:''" json:"source_reference"`
	Status          ControlStatus `gorm:"type:varchar(30);not null;default:'not_implemented'" json:"status"`

	// Pack provenance (ADR 0013 D4). Written only by the catalogue import, absent
	// from every input DTO, never serialised. Empty PackKey: hand-made row.
	// The partial unique index leads on tenant_id and includes soft-deleted rows,
	// which is what makes a deletion a tombstone the import respects.
	PackKey       string  `gorm:"size:64;not null;default:'';uniqueIndex:ux_compliance_controls_pack_item,priority:3,where:pack_key <> ''" json:"-"`
	PackItemKey   string  `gorm:"size:255;not null;default:'';uniqueIndex:ux_compliance_controls_pack_item,priority:4,where:pack_key <> ''" json:"-"`
	PackVersion   string  `gorm:"size:32;not null;default:''" json:"-"`
	PackBase      *string `gorm:"type:jsonb" json:"-"`
	PackBaseHash  *string `gorm:"type:char(64)" json:"-"`
	PackRetiredIn string  `gorm:"size:32;not null;default:''" json:"-"`
	// PackRevertedBy is set only by a pack revert (a later issue); NULL here.
	PackRevertedBy *uuid.UUID `gorm:"type:uuid" json:"-"`

	// Relations
	Framework ComplianceFramework `gorm:"foreignKey:FrameworkID" json:"framework,omitempty"`

	// EvidenceCount is a computed, non-persisted count (gorm:"-": never a column)
	// of the artifacts in the evidence library that CURRENTLY substantiate this
	// control — expired and rejected proof excluded. Populated by the list/get use
	// cases so the UI can show an evidence badge, and read by the "no implemented
	// without proof" rule.
	//
	// There is deliberately no Evidences relation here. Evidence is reusable
	// across controls (see domain.Evidence), so it cannot hang off one control
	// by foreign key; the links live in evidence_control_links.
	EvidenceCount int `gorm:"-" json:"evidence_count"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ComplianceControl) TableName() string {
	return "compliance_controls"
}
