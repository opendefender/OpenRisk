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
	Version     string // e.g. "2022" — the standard's own edition, never our revision
	Description string

	// ContentVersion is OUR revision of this catalogue's content, semver
	// MAJOR.MINOR.PATCH (ADR 0013 D3), distinct from Version (the standard's
	// edition). It is not set in the catalog_*.go files: those may be generated
	// (cmd/catalogsync) and must survive regeneration, so register fills it from
	// contentRevisions (catalog_content.go). TestCatalogContentVersions fails for
	// a catalogue that has none.
	ContentVersion string
	// Changelog says what each content version changed, newest last. It carries
	// an entry for ContentVersion. Same provenance as ContentVersion.
	Changelog []ChangelogEntry
	// Available means importable, nothing more. It is false for catalogs that exist as a
	// placeholder (registered so the product can announce them) but carry no control
	// content yet — Controls is empty and ImportCatalogUseCase refuses to import them.
	// It does NOT say anyone checked the content against the regulatory text: that is
	// Review.Status, and an Available catalog may well be ReviewUnreviewed (D-120).
	Available bool

	// Review records whether a competent reviewer checked this catalog's content against the
	// regulatory text, and who. Every Available catalog must carry a Status; see Review.
	Review Review

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

// ReviewStatus says how far a catalog's content has been checked against the regulatory text.
type ReviewStatus string

const (
	// ReviewUnreviewed: no sign-off is recorded. The catalog is importable but nobody has
	// attested to it. This is the only honest status until a committed review record exists.
	ReviewUnreviewed ReviewStatus = "unreviewed"
	// ReviewStructureReviewed: a reviewer checked reference codes and titles against the
	// source, not the wording of the descriptions.
	ReviewStructureReviewed ReviewStatus = "structure_reviewed"
	// ReviewReviewed: a reviewer checked codes, titles and descriptions against the source.
	ReviewReviewed ReviewStatus = "reviewed"
)

// ReviewSource is one document the reviewer checked the catalog against.
type ReviewSource struct {
	Document string // e.g. "Règlement COBAC R-2016/04"
	URL      string
}

// Review is the sign-off block of a catalog. For any Status other than ReviewUnreviewed,
// Date, Reviewer, at least one Source with a URL, and Gaps are all required (enforced by
// TestCatalogReviewContract). Never fill them without a committed review record: an invented
// reviewer or date is worse than "unreviewed" (D-120).
type Review struct {
	Status ReviewStatus
	// ContentVersion is the content version the reviewer checked (ADR 0013 D1,
	// D-121 item 5). Empty is allowed: no review has been tied to a version yet.
	// When set it is semver.
	ContentVersion string
	Date           string // ISO date, e.g. "2026-10-30"
	Reviewer       string
	Sources        []ReviewSource
	// Gaps lists what the review did not cover or found missing, in writing. An empty Gaps on
	// a signed-off catalog is rejected: "no gaps" must be said, not left blank.
	Gaps []string
}

// catalogs is the registry of every known catalog, keyed by Catalog.Key.
var catalogs = map[string]Catalog{}

// register adds a catalog to the registry. Called from each catalog's init().
func register(c Catalog) {
	if _, exists := catalogs[c.Key]; exists {
		panic("compliance: duplicate catalog key " + c.Key)
	}
	if rev, ok := contentRevisions[c.Key]; ok {
		if c.ContentVersion == "" {
			c.ContentVersion = rev.Version
		}
		if c.Changelog == nil {
			c.Changelog = rev.Changelog
		}
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
