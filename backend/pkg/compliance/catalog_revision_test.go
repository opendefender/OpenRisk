// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"regexp"
	"testing"
)

func revCatalog() Catalog {
	return Catalog{Controls: []CatalogControl{
		{ReferenceCode: "B", Name: "n2", Description: "d2", SourceReference: "s2"},
		{ReferenceCode: "A", Name: "n1", Description: "d1", SourceReference: "s1"},
	}}
}

func TestRevision_DeterministicAndOrderIndependent(t *testing.T) {
	a := revCatalog()
	if a.Revision() == "" || a.Revision() != a.Revision() {
		t.Fatal("revision must be non-empty and stable")
	}
	b := Catalog{Controls: []CatalogControl{a.Controls[1], a.Controls[0]}}
	if a.Revision() != b.Revision() {
		t.Fatal("revision must not depend on control order")
	}
}

func TestRevision_DoesNotMutateControls(t *testing.T) {
	c := revCatalog()
	_ = c.Revision()
	if c.Controls[0].ReferenceCode != "B" {
		t.Fatal("Revision sorted the catalog's own slice")
	}
}

func TestRevision_ChangesWithAnyField(t *testing.T) {
	base := revCatalog().Revision()
	mutate := map[string]func(*CatalogControl){
		"code":   func(c *CatalogControl) { c.ReferenceCode += "x" },
		"name":   func(c *CatalogControl) { c.Name += "x" },
		"desc":   func(c *CatalogControl) { c.Description += "x" },
		"source": func(c *CatalogControl) { c.SourceReference += "x" },
	}
	for name, m := range mutate {
		for i := range revCatalog().Controls {
			c := revCatalog()
			m(&c.Controls[i])
			if c.Revision() == base {
				t.Errorf("changing %s of control %d did not change the revision", name, i)
			}
		}
	}
}

func TestRevision_BoundaryInjectionProof(t *testing.T) {
	a := Catalog{Controls: []CatalogControl{{ReferenceCode: "A", Name: "bc", Description: "d", SourceReference: "s"}}}
	b := Catalog{Controls: []CatalogControl{{ReferenceCode: "Ab", Name: "c", Description: "d", SourceReference: "s"}}}
	c := Catalog{Controls: []CatalogControl{{ReferenceCode: "A", Name: "b\x1fc", Description: "d", SourceReference: "s"}}}
	if a.Revision() == b.Revision() || a.Revision() == c.Revision() {
		t.Fatal("shifting a field boundary must change the revision")
	}
}

func TestRevision_EmptyForNoControls(t *testing.T) {
	if got := (Catalog{}).Revision(); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestRevision_EveryAvailableCatalogHasOne(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{12}$`)
	for _, c := range List() {
		if !c.Available {
			continue
		}
		if !re.MatchString(c.Revision()) {
			t.Errorf("catalog %q: revision %q is not 12 hex chars", c.Key, c.Revision())
		}
	}
}
