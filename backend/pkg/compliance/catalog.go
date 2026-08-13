// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package compliance holds regulatory control catalogs: static reference data a tenant can
// import into their own ComplianceFramework/ComplianceControl rows (see
// internal/application/compliance/import_catalog.go). Each catalog is self-contained and
// versioned independently — adding a new regulatory framework means adding a new file here
// and registering it in the catalogs map, nothing else in this package changes.
package compliance

// CatalogControl is one control entry in a regulatory catalog, not yet attached to any
// tenant. ImportCatalogUseCase turns these into domain.ComplianceControl rows.
type CatalogControl struct {
	ReferenceCode string // e.g. "A.5.1"
	Name          string
	Description   string
	// SourceReference cites exactly where this control comes from (standard section,
	// circular article, law). Every catalog control must have one — that's the whole point
	// of a catalog versus a tenant creating ad-hoc controls by hand.
	SourceReference string
}

// Catalog is a versioned, citable set of controls for one regulatory framework.
type Catalog struct {
	Key         string // stable identifier, e.g. "iso27001-2022" — used in the API and as a map key
	Name        string // framework display name, e.g. "ISO/IEC 27001"
	Version     string // e.g. "2022"
	Description string
	// Available is false for catalogs that exist as a placeholder (registered so the product
	// can announce them) but have no reviewed control content yet — Controls is empty and
	// ImportCatalogUseCase refuses to import them. Flip to true once a compliance-competent
	// reviewer has verified the content against the actual regulatory text.
	Available bool

	// Withdrawn removes a catalog from the import picker entirely, as opposed to
	// showing it as "coming soon".
	//
	// The difference is a promise. An unavailable catalog says "we will ship
	// this"; a withdrawn one says "we are not offering this, and here is why".
	// A framework we cannot cite article by article must not be importable at
	// all: a shell framework reads as coverage in every dashboard, percentage
	// and report the product produces.
	//
	// Withdrawn catalogs stay registered so their key keeps resolving for tenants
	// who imported them before, and so the work to bring one back has somewhere
	// to land.
	Withdrawn bool
	// WithdrawalReason is shown wherever the withdrawal is surfaced. Required
	// when Withdrawn — "unavailable, no reason given" is not an answer a
	// compliance officer can plan around.
	WithdrawalReason string
	// TrackingTicket points at the work to restore it.
	TrackingTicket string

	Controls []CatalogControl
}

// catalogs is the registry of every known catalog, keyed by Catalog.Key.
var catalogs = map[string]Catalog{}

// register adds a catalog to the registry. Called from each catalog's init().
func register(c Catalog) {
	if _, exists := catalogs[c.Key]; exists {
		panic("compliance: duplicate catalog key " + c.Key)
	}
	catalogs[c.Key] = c
}

// ListWithdrawn returns the catalogs removed from the picker, with their reason.
// Not part of the import surface — it exists so the withdrawal is visible
// somewhere rather than being a silent disappearance.
func ListWithdrawn() []Catalog {
	out := make([]Catalog, 0, 2)
	for _, c := range catalogs {
		if c.Withdrawn {
			out = append(out, c)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Key < out[j-1].Key; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Get returns the catalog for a key, or (Catalog{}, false) if unknown.
//
// Withdrawn catalogs still resolve: a tenant who imported one before it was
// withdrawn keeps a framework whose key has to mean something. Refusing the
// import is ImportCatalogUseCase's job, not this one's.
func Get(key string) (Catalog, bool) {
	c, ok := catalogs[key]
	return c, ok
}

// List returns the catalogs offered for import: available ones and placeholders
// announced as coming, but NOT withdrawn ones. Sorted by Key for a stable API
// response order.
//
// This is what the import picker reads, so withdrawing a catalog here is what
// actually removes it from the product.
func List() []Catalog {
	out := make([]Catalog, 0, len(catalogs))
	for _, c := range catalogs {
		if c.Withdrawn {
			continue
		}
		out = append(out, c)
	}
	// Small, fixed set — insertion sort is plenty and keeps this dependency-free.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Key < out[j-1].Key; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
