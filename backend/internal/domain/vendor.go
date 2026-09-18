// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// VENDORS — TPRM v1 (ADR 0004)
// ============================================================================
//
// A vendor is an asset of category vendor (ADR 0001 D5b, ADR 0004 D1). There is
// no vendors table, and this file does not add one: the register reads assets,
// and the vendor→asset links are AssetDependency edges that already exist. What
// this file adds is the vocabulary that decides which of those edges make an
// asset "supplied by" a vendor, and the read shapes of the register and of the
// vendor→asset→risk chain.

// VendorLinkEnd says which end of an AssetDependency edge the vendor occupies
// for a given verb. The stored direction is the dependency vocabulary's own
// ("Source <type> Target"), so it differs from verb to verb.
type VendorLinkEnd string

const (
	// VendorAtTarget: "asset managed_by vendor" — the vendor is the target.
	VendorAtTarget VendorLinkEnd = "target"
	// VendorAtSource: "vendor processes_data_of asset" — the vendor is the source.
	VendorAtSource VendorLinkEnd = "source"
)

// vendorLinkEnds is ADR 0004 D2's table. A verb absent from it does not make an
// edge a vendor link, whatever its endpoints are.
var vendorLinkEnds = map[DependencyType]VendorLinkEnd{
	DepManagedBy:       VendorAtTarget,
	DepHostedBy:        VendorAtTarget,
	DepHostedOn:        VendorAtTarget, // alias of hosted_by
	DepDependsOn:       VendorAtTarget,
	DepProcessesDataOf: VendorAtSource,
	DepStoresDataIn:    VendorAtSource, // alias of processes_data_of
}

// VendorLinkVerbs are the verbs a vendor link may be CREATED with, in the order
// the UI offers them. The two aliases are recognised when reading edges that
// already exist, and are not offered: a new edge uses the canonical verb.
var VendorLinkVerbs = []DependencyType{DepManagedBy, DepHostedBy, DepProcessesDataOf, DepDependsOn}

// VendorLinkEndFor reports where the vendor sits on an edge of type t, and false
// when t is not a vendor-link verb.
func VendorLinkEndFor(t DependencyType) (VendorLinkEnd, bool) {
	end, ok := vendorLinkEnds[t]
	return end, ok
}

// IsCreatableVendorLinkVerb reports whether a new vendor link may use t.
func IsCreatableVendorLinkVerb(t DependencyType) bool {
	for _, v := range VendorLinkVerbs {
		if v == t {
			return true
		}
	}
	return false
}

// VendorLinkedAsset returns the other end of dep when dep is a vendor link of
// vendorID — its verb qualifies AND vendorID sits at that verb's end — and false
// otherwise.
//
// The end matters. "vendor managed_by asset" carries a qualifying verb with the
// vendor at the wrong end: it says the vendor is operated by the asset, which is
// not a supply relation, and it is not read as one.
func VendorLinkedAsset(dep AssetDependency, vendorID uuid.UUID) (uuid.UUID, bool) {
	end, ok := VendorLinkEndFor(dep.Type)
	if !ok {
		return uuid.Nil, false
	}
	switch end {
	case VendorAtTarget:
		if dep.TargetAssetID == vendorID {
			return dep.SourceAssetID, true
		}
	case VendorAtSource:
		if dep.SourceAssetID == vendorID {
			return dep.TargetAssetID, true
		}
	}
	return uuid.Nil, false
}

// AssetAttrString reads a string attribute from an asset's typed attribute bag,
// or "" when it is absent or not a string. A missing attribute is reported as
// absent, never defaulted.
func AssetAttrString(attrs AssetAttributes, key string) string {
	if attrs == nil {
		return ""
	}
	s, _ := attrs[key].(string)
	return s
}

// VendorFilter narrows the vendor register. Empty fields do not filter.
type VendorFilter struct {
	// Search matches, case-insensitively, the asset name or the legal_name
	// attribute.
	Search string
	// ServiceCriticality matches the service_criticality attribute exactly. It
	// is not checked against the shipped enum: a tenant may edit its vendor
	// schema, and a filter must not reject a value the tenant's own schema allows.
	ServiceCriticality string
	Limit              int
	Offset             int
}

// VendorLatestAssessment is the register's view of a vendor's most recent
// questionnaire (ADR 0004 D3). The assessment module (#670) supplies it; until
// that module is wired, the register reports no assessment rather than a
// placeholder.
type VendorLatestAssessment struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
	Score  *float64  `json:"score"`
	Tier   *string   `json:"tier"`
	DueAt  time.Time `json:"due_at"`
}

// VendorRegisterEntry is one row of the vendor register. Every field comes from
// the vendor asset or from a tenant-scoped count; nothing is computed here.
type VendorRegisterEntry struct {
	ID                 uuid.UUID               `json:"id"`
	Name               string                  `json:"name"`
	LegalName          string                  `json:"legal_name"`
	Country            string                  `json:"country"`
	ServiceProvided    string                  `json:"service_provided"`
	ServiceCriticality string                  `json:"service_criticality"`
	ContractEnd        string                  `json:"contract_end"`
	Criticality        AssetCriticality        `json:"criticality"`
	Owner              string                  `json:"owner"`
	LinkedAssets       int                     `json:"linked_assets"`
	LatestAssessment   *VendorLatestAssessment `json:"latest_assessment"`
}

// NewVendorRegisterEntry reads a vendor asset into a register row. The linked
// asset count and the latest assessment are filled by the caller.
func NewVendorRegisterEntry(a Asset) VendorRegisterEntry {
	return VendorRegisterEntry{
		ID:                 a.ID,
		Name:               a.Name,
		LegalName:          AssetAttrString(a.Attributes, "legal_name"),
		Country:            AssetAttrString(a.Attributes, "country"),
		ServiceProvided:    AssetAttrString(a.Attributes, "service_provided"),
		ServiceCriticality: AssetAttrString(a.Attributes, "service_criticality"),
		ContractEnd:        AssetAttrString(a.Attributes, "contract_end"),
		Criticality:        a.Criticality,
		Owner:              a.Owner,
	}
}

// VendorPage is one page of the register. Items is always an array, never null.
type VendorPage struct {
	Items  []VendorRegisterEntry `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

// VendorChainRisk is a risk reached through a vendor-linked asset.
type VendorChainRisk struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Score       float64   `json:"score"`
	Criticality string    `json:"criticality"`
	Status      string    `json:"status"`
}

// VendorChainAsset is the asset end of a vendor link.
type VendorChainAsset struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Category    AssetCategory    `json:"category"`
	Criticality AssetCriticality `json:"criticality"`
}

// VendorChainLink is one hop of the chain: a vendor link, the asset it reaches,
// and that asset's risks. Risks is always an array, never null.
type VendorChainLink struct {
	LinkID uuid.UUID         `json:"link_id"`
	Verb   DependencyType    `json:"verb"`
	Asset  VendorChainAsset  `json:"asset"`
	Risks  []VendorChainRisk `json:"risks"`
}

// VendorChain is the vendor→asset→risk chain, one hop deep (ADR 0004 D2).
// Multi-hop chains are out of scope (#376).
type VendorChain struct {
	VendorID   uuid.UUID         `json:"vendor_id"`
	VendorName string            `json:"vendor_name"`
	Links      []VendorChainLink `json:"links"`
}

// VendorRepository is the read port of the vendor register. Every method puts
// the tenant in its query; a row of another tenant is absent, never an error.
type VendorRepository interface {
	// ListVendors returns every asset of category vendor in the tenant, by name.
	ListVendors(ctx context.Context, tenantID uuid.UUID) ([]Asset, error)

	// GetVendor returns the asset only when it is in the tenant AND of category
	// vendor, and (nil, nil) otherwise — so a server's id and a foreign id are
	// the same not-found (ADR 0001 D5b).
	GetVendor(ctx context.Context, id, tenantID uuid.UUID) (*Asset, error)

	// AssetsByIDs returns the tenant's assets among ids. Foreign and absent ids
	// are simply not returned.
	AssetsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]Asset, error)

	// RisksByAssetIDs returns, per asset id, the tenant's risks linked to it.
	// risk_assets has no tenant_id, so the gate is the PARENT RISK's tenant_id
	// (ADR 0004 D2), never the join row.
	RisksByAssetIDs(ctx context.Context, tenantID uuid.UUID, assetIDs []uuid.UUID) (map[uuid.UUID][]VendorChainRisk, error)
}

// VendorLatestAssessmentReader supplies the latest assessment per vendor. The
// assessment module (#670) implements it.
type VendorLatestAssessmentReader interface {
	LatestByVendor(ctx context.Context, tenantID uuid.UUID, vendorIDs []uuid.UUID) (map[uuid.UUID]VendorLatestAssessment, error)
}
