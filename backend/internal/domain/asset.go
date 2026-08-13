// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type AssetCriticality string

const (
	CriticalityLow      AssetCriticality = "LOW"
	CriticalityMedium   AssetCriticality = "MEDIUM"
	CriticalityHigh     AssetCriticality = "HIGH"
	CriticalityCritical AssetCriticality = "CRITICAL"
)

// ScoreFactor maps an AssetCriticality to the numeric multiplier consumed by
// pkg/scoring.Engine (declared range [0.1, 3.0] — see CLAUDE.md's Score Engine
// formula). This is the single source of truth for that mapping: previously
// three different call sites (get_score_breakdown.go, score_service.go,
// score_engine_service.go) each hardcoded their own, inconsistent values.
// Unknown/empty criticality defaults to MEDIUM's factor.
func (c AssetCriticality) ScoreFactor() float64 {
	switch c {
	case CriticalityLow:
		return 0.5
	case CriticalityHigh:
		return 2.5
	case CriticalityCritical:
		return 3.0
	default:
		return 1.5 // MEDIUM and unknown values
	}
}

type Asset struct {
	ID             uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID       uuid.UUID        `gorm:"type:uuid;index" json:"tenant_id"`
	OrganizationID uuid.UUID        `gorm:"index" json:"organization_id"`
	Name           string           `gorm:"not null" json:"name"`
	Type           string           `json:"type"` // Server, Laptop, Database, SaaS
	Criticality    AssetCriticality `gorm:"default:'MEDIUM'" json:"criticality"`
	Owner          string           `json:"owner"`

	// Relation Many-to-Many avec Risk
	Risks []*Risk `gorm:"many2many:risk_assets;" json:"risks,omitempty"`

	Source     string `gorm:"default:'MANUAL'" json:"source"` // MANUAL ou OPENASSET
	ExternalID string `json:"external_id"`

	// CPEs (Common Platform Enumeration) identify the software/hardware running on
	// this asset. Populated by the Infrastructure Scanner on import (and editable
	// manually); CTI matching intersects these against cti_vulnerabilities.affected_cpe
	// to auto-create risks for exposed CVEs.
	CPEs pq.StringArray `gorm:"column:cpes;type:text[]" json:"cpes"`

	// Category selects which attribute schema governs this asset (see
	// AssetTypeSchema). Type stays as the free-text display label — category is
	// the closed vocabulary the form generator and the validator key off.
	// Empty on rows written before typed attributes existed: those assets simply
	// have no typed attributes until someone assigns them a category.
	Category AssetCategory `gorm:"type:varchar(32);index" json:"category"`

	// Attributes holds the typed, schema-validated attribute values. Every write
	// path runs ValidateAttributes against the tenant's schema first, so an
	// out-of-schema key or a malformed value never reaches this column.
	Attributes AssetAttributes `gorm:"type:jsonb" json:"attributes"`

	// --- correlation fingerprints ------------------------------------------
	// Denormalised out of Attributes on every write (see RefreshFingerprints).
	// They exist as their own indexed columns because the vulnerability↔asset
	// correlator matches on them for every ingested finding: querying a JSONB
	// bag per finding would make a 10k-finding import quadratic.
	Hostnames       pq.StringArray `gorm:"column:hostnames;type:text[]" json:"hostnames"`
	IPAddresses     pq.StringArray `gorm:"column:ip_addresses;type:text[]" json:"ip_addresses"`
	CloudResourceID string         `gorm:"size:512;index" json:"cloud_resource_id"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeSave keeps TenantID and OrganizationID in sync — OrganizationID is
// the historical field name (still used by the legacy handler query paths),
// TenantID is the canonical name used by the new repository/use-case layer
// (mirrors domain.Risk's TenantID/OrganizationID alias pattern).
func (a *Asset) BeforeSave(tx *gorm.DB) error {
	if a.TenantID == uuid.Nil && a.OrganizationID != uuid.Nil {
		a.TenantID = a.OrganizationID
	}
	if a.OrganizationID == uuid.Nil && a.TenantID != uuid.Nil {
		a.OrganizationID = a.TenantID
	}
	return nil
}

// RefreshFingerprints recomputes the denormalised correlation columns from the
// asset's typed attributes, given the schema that governs its category.
//
// Called on every asset write. Values already present on the asset (a hostname
// typed into the legacy field, CPEs pushed by the scanner) are PRESERVED and
// merged, never replaced: the scanner learns things about an asset the operator
// never typed, and losing them at the next manual edit is exactly how a
// correlation silently starts failing.
func (a *Asset) RefreshFingerprints(defs []AttributeDef) {
	if a == nil {
		return
	}
	sig := FingerprintSignalsFrom(defs, a.Attributes)

	a.Hostnames = mergeUnique(a.Hostnames, sig.Hostnames)
	a.IPAddresses = mergeUnique(a.IPAddresses, sig.IPs)
	a.CPEs = mergeUnique(a.CPEs, sig.CPEs)
	if len(sig.CloudIDs) > 0 {
		a.CloudResourceID = sig.CloudIDs[0]
	}
}

// mergeUnique appends the values of b that a does not already carry, comparing
// case-insensitively and preserving a's order.
func mergeUnique(a pq.StringArray, b []string) pq.StringArray {
	seen := make(map[string]bool, len(a)+len(b))
	out := make(pq.StringArray, 0, len(a)+len(b))
	for _, v := range a {
		v = strings.TrimSpace(v)
		if v == "" || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	for _, v := range b {
		v = strings.TrimSpace(v)
		if v == "" || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	return out
}

// AssetSnapshot captures an asset's state at a point in time, taken
// immediately before an update or deletion is applied. This is what powers
// the asset inventory's history view (ROADMAP.md M3): "what did this asset
// look like, and when did its criticality change".
//
// Traceability ("qui a modifié quoi, et quand"): the snapshot records the
// prior state (the *quoi*, diffable against the current asset), CreatedAt (the
// *quand*), and ChangedBy — the user who performed the update/delete that
// superseded this state (the *qui*). ChangedBy may be uuid.Nil for rows written
// before this field existed or for non-interactive/system changes.
type AssetSnapshot struct {
	ID          uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID        `gorm:"type:uuid;not null;index" json:"tenant_id"`
	AssetID     uuid.UUID        `gorm:"type:uuid;not null;index" json:"asset_id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Criticality AssetCriticality `json:"criticality"`
	Owner       string           `json:"owner"`
	// Reason describes why the snapshot was taken: "update" or "delete".
	Reason string `gorm:"size:20;not null;default:'update'" json:"reason"`
	// ChangedBy is the ID of the user who caused this snapshot to be taken
	// (i.e. who performed the change). Persisted; nullable for legacy/system rows.
	ChangedBy uuid.UUID `gorm:"type:uuid;index" json:"changed_by"`
	// ChangedByEmail is a computed, denormalized display label resolved from
	// ChangedBy on the read path (ListSnapshots). NOT persisted (gorm:"-") — it
	// is populated best-effort so the history UI can show a human name instead
	// of a raw UUID. Empty when the actor is unknown or cannot be resolved.
	ChangedByEmail string `gorm:"-" json:"changed_by_email,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// TableName overrides the default GORM table name.
func (AssetSnapshot) TableName() string {
	return "asset_snapshots"
}
