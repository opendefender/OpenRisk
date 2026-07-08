// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import "testing"

// TestNoOrphanControls is the "aucun contrôle orphelin" consistency check ROADMAP.md M2
// calls for: every control in every catalog must be complete and traceable to a source,
// and reference codes must be unique within their catalog. A control failing this is either
// a data-entry mistake or missing its citation — either way it shouldn't ship.
func TestNoOrphanControls(t *testing.T) {
	for _, cat := range List() {
		t.Run(cat.Key, func(t *testing.T) {
			if !cat.Available {
				if len(cat.Controls) != 0 {
					t.Errorf("catalog %q is marked unavailable but has %d controls — either flip Available to true or remove the content", cat.Key, len(cat.Controls))
				}
				return
			}

			if len(cat.Controls) == 0 {
				t.Errorf("catalog %q is marked available but has no controls", cat.Key)
			}

			seen := make(map[string]bool, len(cat.Controls))
			for i, c := range cat.Controls {
				if c.ReferenceCode == "" {
					t.Errorf("control at index %d has no reference code", i)
				}
				if c.Name == "" {
					t.Errorf("control %q has no name", c.ReferenceCode)
				}
				if c.SourceReference == "" {
					t.Errorf("control %q %q has no source_reference — every catalog control must cite where it comes from", c.ReferenceCode, c.Name)
				}
				if seen[c.ReferenceCode] {
					t.Errorf("duplicate reference code %q in catalog %q", c.ReferenceCode, cat.Key)
				}
				seen[c.ReferenceCode] = true
			}
		})
	}
}

func TestISO27001_2022_HasExpectedControlCount(t *testing.T) {
	cat, ok := Get("iso27001-2022")
	if !ok {
		t.Fatal("iso27001-2022 catalog not registered")
	}
	// ISO/IEC 27001:2022 Annex A: 37 Organizational + 8 People + 14 Physical + 34 Technological = 93.
	const want = 93
	if got := len(cat.Controls); got != want {
		t.Errorf("expected %d controls in ISO 27001:2022, got %d", want, got)
	}
}

func TestGet_UnknownKey(t *testing.T) {
	if _, ok := Get("does-not-exist"); ok {
		t.Error("expected ok=false for an unregistered catalog key")
	}
}

func TestList_SortedByKey(t *testing.T) {
	list := List()
	for i := 1; i < len(list); i++ {
		if list[i].Key < list[i-1].Key {
			t.Errorf("List() not sorted: %q comes after %q", list[i].Key, list[i-1].Key)
		}
	}
}
