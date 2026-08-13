// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import "testing"

// A crosswalk pointing at a control that does not exist is worse than no
// crosswalk: it silently drops out of the materialisation, so the coverage
// number quietly under-reports and nobody can tell why. This test is the reason
// the curated data can be edited with confidence.
func TestCrosswalks_ReferenceRealControls(t *testing.T) {
	for _, e := range AllCrosswalks() {
		src, ok := Get(e.SourceCatalog)
		if !ok {
			t.Errorf("crosswalk references unknown catalog %q", e.SourceCatalog)
			continue
		}
		tgt, ok := Get(e.TargetCatalog)
		if !ok {
			t.Errorf("crosswalk references unknown catalog %q", e.TargetCatalog)
			continue
		}
		if !hasControl(src, e.SourceCode) {
			t.Errorf("%s has no control %q (crosswalk to %s %s)",
				e.SourceCatalog, e.SourceCode, e.TargetCatalog, e.TargetCode)
		}
		if !hasControl(tgt, e.TargetCode) {
			t.Errorf("%s has no control %q (crosswalk from %s %s)",
				e.TargetCatalog, e.TargetCode, e.SourceCatalog, e.SourceCode)
		}
	}
}

func hasControl(c Catalog, code string) bool {
	for _, ctrl := range c.Controls {
		if ctrl.ReferenceCode == code {
			return true
		}
	}
	return false
}

// Every curated link must carry a coverage level and a reason. The reason is
// what a tenant reads before accepting that their existing proof answers a new
// framework's control — an unexplained link is one they cannot defend to an
// auditor, and would be right to distrust.
func TestCrosswalks_CarryCoverageAndRationale(t *testing.T) {
	for _, e := range AllCrosswalks() {
		if !e.Coverage.Valid() {
			t.Errorf("%s %s -> %s %s: invalid coverage %q",
				e.SourceCatalog, e.SourceCode, e.TargetCatalog, e.TargetCode, e.Coverage)
		}
		if len(e.Rationale) < 20 {
			t.Errorf("%s %s -> %s %s: rationale is too thin to be useful (%q)",
				e.SourceCatalog, e.SourceCode, e.TargetCatalog, e.TargetCode, e.Rationale)
		}
	}
}

func TestCrosswalks_NoSelfCatalogLinks(t *testing.T) {
	for _, e := range AllCrosswalks() {
		if e.SourceCatalog == e.TargetCatalog {
			t.Errorf("crosswalk within a single catalog (%s): that is a different, much weaker claim",
				e.SourceCatalog)
		}
	}
}

func TestCrosswalks_NoDuplicatePairs(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range AllCrosswalks() {
		// Both orders, since Between treats the pair as undirected.
		a := e.SourceCatalog + "|" + e.SourceCode + "|" + e.TargetCatalog + "|" + e.TargetCode
		b := e.TargetCatalog + "|" + e.TargetCode + "|" + e.SourceCatalog + "|" + e.SourceCode
		if seen[a] || seen[b] {
			t.Errorf("duplicate crosswalk for %s", a)
		}
		seen[a] = true
	}
}

// Between must answer in the direction asked, whichever way the entry was
// written down — otherwise inherited coverage would only work for whichever
// framework happened to be imported second.
func TestCrosswalksBetween_IsSymmetric(t *testing.T) {
	forward := CrosswalksBetween("iso27001-2022", "soc2-tsc")
	backward := CrosswalksBetween("soc2-tsc", "iso27001-2022")

	if len(forward) == 0 {
		t.Fatal("expected curated ISO <-> SOC 2 content")
	}
	if len(forward) != len(backward) {
		t.Fatalf("both directions must yield the same count: %d vs %d", len(forward), len(backward))
	}
	for _, e := range forward {
		if e.SourceCatalog != "iso27001-2022" || e.TargetCatalog != "soc2-tsc" {
			t.Fatalf("forward direction returned %s -> %s", e.SourceCatalog, e.TargetCatalog)
		}
	}
	for _, e := range backward {
		if e.SourceCatalog != "soc2-tsc" || e.TargetCatalog != "iso27001-2022" {
			t.Fatalf("backward direction returned %s -> %s", e.SourceCatalog, e.TargetCatalog)
		}
	}

	// The flipped entry must keep its reason: it explains the correspondence, not
	// the direction of travel.
	for _, e := range backward {
		if e.Rationale == "" {
			t.Fatal("a flipped crosswalk lost its rationale")
		}
	}
}

func TestCrosswalksBetween_SameCatalogIsEmpty(t *testing.T) {
	if got := CrosswalksBetween("iso27001-2022", "iso27001-2022"); len(got) != 0 {
		t.Fatalf("a catalog does not crosswalk to itself, got %d entries", len(got))
	}
}

func TestCrosswalkCatalogPairs_AreDeduplicated(t *testing.T) {
	pairs := CrosswalkCatalogPairs()
	if len(pairs) == 0 {
		t.Fatal("expected at least one curated pair")
	}
	seen := map[[2]string]bool{}
	for _, p := range pairs {
		if seen[p] {
			t.Fatalf("duplicate pair %v", p)
		}
		seen[p] = true
		if p[0] >= p[1] {
			t.Fatalf("pairs should be normalised and ordered, got %v", p)
		}
	}
}
