// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
	Controls  []CatalogControl
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

// Get returns the catalog for a key, or (Catalog{}, false) if unknown.
func Get(key string) (Catalog, bool) {
	c, ok := catalogs[key]
	return c, ok
}

// List returns every registered catalog (available and placeholder alike), sorted by Key for
// a stable API response order.
func List() []Catalog {
	out := make([]Catalog, 0, len(catalogs))
	for _, c := range catalogs {
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
