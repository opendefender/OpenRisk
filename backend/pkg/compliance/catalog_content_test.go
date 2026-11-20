// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"regexp"
	"testing"
)

// Criterion 4 of #1483: every registered catalogue carries a valid content
// version, a changelog entry for it, and a Review.ContentVersion that is empty or
// valid. Withdrawn and unavailable catalogues are included: List() hides
// withdrawn ones, so this walks the registry itself.
func TestCatalogContentVersions(t *testing.T) {
	if len(catalogs) == 0 {
		t.Fatal("registry is empty")
	}
	for key, cat := range catalogs {
		t.Run(key, func(t *testing.T) {
			if !ValidSemver(cat.ContentVersion) {
				t.Fatalf("ContentVersion %q is not MAJOR.MINOR.PATCH", cat.ContentVersion)
			}
			found := false
			for _, e := range cat.Changelog {
				if !ValidSemver(e.Version) {
					t.Errorf("changelog version %q is not semver", e.Version)
				}
				if e.Summary == "" {
					t.Errorf("changelog %s has no summary", e.Version)
				}
				if e.Version == cat.ContentVersion {
					found = true
				}
			}
			if !found {
				t.Errorf("no changelog entry for ContentVersion %s", cat.ContentVersion)
			}
			if cat.Review.ContentVersion != "" && !ValidSemver(cat.Review.ContentVersion) {
				t.Errorf("Review.ContentVersion %q is not semver", cat.Review.ContentVersion)
			}
		})
	}
	// A table entry for a key that is not registered is a typo that would never apply.
	for key := range contentRevisions {
		if _, ok := catalogs[key]; !ok {
			t.Errorf("contentRevisions has %q, which is not a registered catalogue", key)
		}
	}
}

func TestCrosswalksVersion_IsSemver(t *testing.T) {
	if !ValidSemver(CrosswalksVersion) {
		t.Fatalf("CrosswalksVersion %q is not semver", CrosswalksVersion)
	}
}

func TestValidSemver(t *testing.T) {
	for _, v := range []string{"1.0.0", "0.12.3", "10.20.30"} {
		if !ValidSemver(v) {
			t.Errorf("%q should be valid", v)
		}
	}
	for _, v := range []string{"", "1.0", "v1.0.0", "1.0.0-rc1", "01.0.0", "1.0.0\n", "1.0.0.0"} {
		if ValidSemver(v) {
			t.Errorf("%q should be invalid", v)
		}
	}
}

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestPackBase_RequirementHashIsStable64Hex(t *testing.T) {
	pb := RequirementPackBase(CatalogControl{ReferenceCode: "A.1", Name: "N <x>", Description: "D & é", SourceReference: "S"})
	if !hex64.MatchString(pb.Hash) {
		t.Fatalf("hash %q is not 64 lowercase hex", pb.Hash)
	}
	want := `{"description":"D & é","name":"N <x>","source_reference":"S"}`
	if pb.JSON != want {
		t.Fatalf("canonical json = %s, want %s (sorted keys, no HTML escaping)", pb.JSON, want)
	}
	// Recomputed from a re-ordered, re-spaced document, as jsonb returns it.
	again, err := HashPackBase([]byte(`{"name": "N <x>", "source_reference": "S", "description": "D & é"}`))
	if err != nil || again != pb.Hash {
		t.Fatalf("recomputed %q (%v), want %q", again, err, pb.Hash)
	}
}

func TestHashPackBase_ChangesWithContentAndRejectsNonObject(t *testing.T) {
	a, _ := HashPackBase([]byte(`{"name":"a"}`))
	b, _ := HashPackBase([]byte(`{"name":"b"}`))
	if a == b {
		t.Fatal("different content must hash differently")
	}
	for _, bad := range []string{``, `[]`, `"x"`, `{"a":1} {"b":2}`} {
		if _, err := HashPackBase([]byte(bad)); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestCrosswalkItemKey_OrderedByCatalogKey(t *testing.T) {
	e := CrosswalkEntry{SourceCatalog: "soc2-tsc", SourceCode: "CC6.1", TargetCatalog: "iso27001-2022", TargetCode: "A.5.15"}
	want := "iso27001-2022:A.5.15|soc2-tsc:CC6.1"
	if got := CrosswalkItemKey(e); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	e.SourceCatalog, e.SourceCode, e.TargetCatalog, e.TargetCode = e.TargetCatalog, e.TargetCode, e.SourceCatalog, e.SourceCode
	if got := CrosswalkItemKey(e); got != want {
		t.Fatalf("reversed entry gives %q, want %q", got, want)
	}
}

func TestCrosswalkPackBase_HoldsCoverageAndRationale(t *testing.T) {
	pb := CrosswalkPackBase(CrosswalkEntry{Coverage: CoveragePartial, Rationale: "r"})
	if pb.JSON != `{"coverage":"partial","rationale":"r"}` || !hex64.MatchString(pb.Hash) {
		t.Fatalf("unexpected %+v", pb)
	}
}
