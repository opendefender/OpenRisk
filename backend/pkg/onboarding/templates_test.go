// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package onboarding

import (
	"testing"

	"github.com/opendefender/openrisk/pkg/compliance"
)

// Every framework key we suggest must actually resolve in the catalog registry,
// and must be importable. A typo here is invisible until a newcomer clicks
// "import" on the wizard's framework step and gets a 404 on their second minute
// in the product.
func TestSuggestedFrameworks_KeysExistInCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, sector := range append(KnownSectorKeys(), "other", "") {
		for _, country := range []string{"CM", "SN", "FR", "US", "MA", "ZZ", ""} {
			for _, goal := range []string{"pass_audit", "map_risks", "cobac_compliance", "other", ""} {
				for _, key := range SuggestedFrameworks(sector, country, goal) {
					if seen[key] {
						continue
					}
					seen[key] = true
					cat, ok := compliance.Get(key)
					if !ok {
						t.Errorf("suggested framework %q is not a registered catalog", key)
						continue
					}
					if !cat.Available {
						t.Errorf("suggested framework %q is a placeholder catalog (not importable)", key)
					}
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no framework keys were exercised")
	}
}

// Every sector offers exactly three drafts, each with content and scales the
// Score Engine accepts. A suggestion out of range would produce a 400 the moment
// the newcomer clicks it — the worst possible first experience.
func TestRiskSuggestions_ShapeAndScales(t *testing.T) {
	sectors := append(KnownSectorKeys(), "", "unknown-sector")
	for _, sector := range sectors {
		got := RiskSuggestionsFor(sector)
		if len(got) != 3 {
			t.Fatalf("sector %q: want 3 suggestions, got %d", sector, len(got))
		}
		seen := map[string]bool{}
		for _, s := range got {
			if s.Key == "" || s.Title == "" || s.Description == "" {
				t.Errorf("sector %q: suggestion %+v has empty content", sector, s)
			}
			if seen[s.Key] {
				t.Errorf("sector %q: duplicate suggestion key %q", sector, s.Key)
			}
			seen[s.Key] = true
			if s.Probability < 0 || s.Probability > 1 {
				t.Errorf("sector %q / %q: probability %v outside [0,1]", sector, s.Key, s.Probability)
			}
			if s.Impact < 0 || s.Impact > 10 {
				t.Errorf("sector %q / %q: impact %v outside [0,10]", sector, s.Key, s.Impact)
			}
		}
	}
}

// The three drafts named in the spec are the generic fallback.
func TestRiskSuggestions_GenericFallbackMatchesSpec(t *testing.T) {
	got := RiskSuggestionsFor("")
	want := []string{
		"Compromission d'identifiants d'administrateur",
		"Indisponibilité du système de paiement",
		"Fuite de données clients",
	}
	for i, title := range want {
		if got[i].Title != title {
			t.Errorf("fallback[%d] = %q, want %q", i, got[i].Title, title)
		}
	}
}

func TestSuggestedFrameworks_GoalCountrySectorOrdering(t *testing.T) {
	// COBAC goal in Cameroon, banking: the goal's own frameworks lead, then the
	// country's regimes, then the sector's peers.
	got := SuggestedFrameworks("banking", "CM", "cobac_compliance")
	if len(got) < 4 {
		t.Fatalf("want a populated list, got %v", got)
	}
	if got[0] != "cobac" || got[1] != "bceao" {
		t.Errorf("goal frameworks should lead, got %v", got)
	}
	if !contains(got, "antic-cm") {
		t.Errorf("Cameroon should contribute antic-cm, got %v", got)
	}
	if !contains(got, "pci-dss-4.0") {
		t.Errorf("banking should contribute pci-dss-4.0, got %v", got)
	}
	assertNoDuplicates(t, got)
}

func TestSuggestedFrameworks_EUSectoralOverlay(t *testing.T) {
	got := SuggestedFrameworks("banking", "FR", "")
	if !contains(got, "gdpr-2016-679") {
		t.Errorf("an EU country must suggest GDPR, got %v", got)
	}
	if !contains(got, "dora-2022-2554") {
		t.Errorf("EU banking must suggest DORA, got %v", got)
	}
	if contains(SuggestedFrameworks("banking", "CM", ""), "dora-2022-2554") {
		t.Error("DORA must not leak outside the EU")
	}
}

// A country code typed in lower case is the same country.
func TestSuggestedFrameworks_CountryCaseInsensitive(t *testing.T) {
	upper := SuggestedFrameworks("tech", "SN", "")
	lower := SuggestedFrameworks("tech", "sn", "")
	if len(upper) != len(lower) || upper[0] != lower[0] {
		t.Errorf("case must not change the result: %v vs %v", upper, lower)
	}
	if !contains(lower, "bceao") {
		t.Errorf("Senegal should contribute bceao, got %v", lower)
	}
}

// Never an empty list, whatever the caller passes.
func TestSuggestedFrameworks_NeverEmpty(t *testing.T) {
	for _, tc := range [][3]string{{"", "", ""}, {"nope", "ZZ", "nope"}} {
		got := SuggestedFrameworks(tc[0], tc[1], tc[2])
		if len(got) == 0 {
			t.Fatalf("SuggestedFrameworks%v returned nothing", tc)
		}
		assertNoDuplicates(t, got)
	}
}

// The four goals of the spec exist and every one lands somewhere real.
func TestGoals_SpecCoverage(t *testing.T) {
	want := []string{"pass_audit", "map_risks", "cobac_compliance", "other"}
	got := Goals()
	if len(got) != len(want) {
		t.Fatalf("want %d goals, got %d", len(want), len(got))
	}
	for i, key := range want {
		if got[i].Key != key {
			t.Errorf("goal[%d] = %q, want %q", i, got[i].Key, key)
		}
		if got[i].Landing == "" {
			t.Errorf("goal %q has no landing route", key)
		}
		for _, lang := range []string{"fr", "en"} {
			if got[i].LabelI18n[lang] == "" {
				t.Errorf("goal %q missing %s label", key, lang)
			}
		}
	}
	if LandingForGoal("does-not-exist") != "/" {
		t.Error("an unknown goal must fall back to the dashboard")
	}
}

func TestSectors_Labelled(t *testing.T) {
	for _, s := range Sectors() {
		for _, lang := range []string{"fr", "en"} {
			if s.LabelI18n[lang] == "" {
				t.Errorf("sector %q missing %s label", s.Key, lang)
			}
		}
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func assertNoDuplicates(t *testing.T, list []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, v := range list {
		if seen[v] {
			t.Errorf("duplicate framework %q in %v", v, list)
		}
		seen[v] = true
	}
}
