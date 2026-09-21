// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// The accents a tenant may choose must be exactly the variants the design
// system declares — for BOTH themes, since each one is contrast-checked per
// theme. A preset without its CSS would silently render the default accent; a
// variant without a preset could never be chosen.
func TestAccentPresets_MatchDesignTokens(t *testing.T) {
	css, err := os.ReadFile("../../../frontend/src/styles/tokens.css")
	if err != nil {
		t.Skipf("design tokens not available from this checkout: %v", err)
	}
	dark := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^:root\[data-variant='([a-z]+)'\] \{`).FindAllStringSubmatch(string(css), -1) {
		dark[m[1]] = true
	}
	light := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^:root\[data-theme='light'\]\[data-variant='([a-z]+)'\] \{`).FindAllStringSubmatch(string(css), -1) {
		light[m[1]] = true
	}
	var declared []string
	for v := range dark {
		if light[v] {
			declared = append(declared, v)
		}
	}
	sort.Strings(declared)
	presets := AccentPresets()
	sort.Strings(presets)
	if len(declared) == 0 || len(declared) != len(presets) {
		t.Fatalf("presets %v, design-token variants with both themes %v", presets, declared)
	}
	for i := range presets {
		if presets[i] != declared[i] {
			t.Fatalf("presets %v, design-token variants %v", presets, declared)
		}
	}
}

func TestOrganizationProfilePatch_Accent(t *testing.T) {
	for _, ok := range []string{"azure", "iris", ""} {
		v := ok
		if err := (&OrganizationProfilePatch{Accent: &v}).Normalize(); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"#ff0000", "red", "AZURE ", "neon"} {
		v := bad
		if err := (&OrganizationProfilePatch{Accent: &v}).Normalize(); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}
