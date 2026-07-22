// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

// TestExpectedControlCounts locks the size of each new international catalog so a
// truncation (a dropped control block) is caught in CI rather than shipping a
// silently-incomplete framework. Counts are the framework's own public structure.
func TestExpectedControlCounts(t *testing.T) {
	cases := map[string]int{
		"nist-csf-2.0":   22, // 6 Functions → 22 Categories
		"cis-v8":         18, // 18 Critical Security Controls
		"pci-dss-4.0":    12, // 12 core requirements
		"hipaa-security": 22, // Administrative(9)+Physical(4)+Technical(5)+Organizational(2)+Docs(2)
		"soc2-tsc":       51, // Common Criteria(33)+A(3)+C(2)+PI(5)+P(8)
		// International frameworks added for the "5. Conformité" spec — full target list.
		"iso27005-2022":  19, // Process clauses 5,6,7,8,9,10 activities
		"iso31000-2018":  22, // 8 principles + 6 framework components + 8 process activities
		"nist-800-53-r5": 20, // 20 control families (AC…SR)
		"gdpr-2016-679":  22, // key operational articles (principles, rights, security, DPO, transfers)
		"dora-2022-2554": 19, // 5 pillars — key articles
		"nis2-2022-2555": 12, // governance + the 10 art.21 measures + notification
		"sox-2002":       10, // 6 statutory sections + 4 ITGC domains
	}
	for key, want := range cases {
		t.Run(key, func(t *testing.T) {
			cat, ok := Get(key)
			if !ok {
				t.Fatalf("catalog %q not registered", key)
			}
			if got := len(cat.Controls); got != want {
				t.Errorf("expected %d controls in %q, got %d", want, key, got)
			}
		})
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
