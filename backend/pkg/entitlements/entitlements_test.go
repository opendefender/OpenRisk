// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package entitlements

import "testing"

func TestParsePlan_LegacyAndUnknown(t *testing.T) {
	cases := map[string]Plan{
		"free": PlanFree, "community": PlanFree, "ce": PlanFree,
		"starter": PlanPro, "pro": PlanPro,
		"professional": PlanBusiness, "business": PlanBusiness,
		"enterprise": PlanEnterprise,
		"":            PlanFree, "nonsense": PlanFree, "  PRO  ": PlanPro,
	}
	for in, want := range cases {
		if got := ParsePlan(in); got != want {
			t.Errorf("ParsePlan(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlanOrdering(t *testing.T) {
	if !PlanBusiness.AtLeast(PlanPro) || !PlanEnterprise.AtLeast(PlanEnterprise) {
		t.Fatal("AtLeast ordering broken")
	}
	if PlanFree.AtLeast(PlanPro) {
		t.Fatal("Free must not be AtLeast Pro")
	}
}

func TestFinancialQuant_OpensAtPro(t *testing.T) {
	// Task §2: Monte-Carlo financial quantification is available "à partir du plan Pro".
	if Has(PlanFree, FeatFinancialQuant) {
		t.Fatal("financial quantification must NOT be on Free")
	}
	for _, p := range []Plan{PlanPro, PlanBusiness, PlanEnterprise} {
		if !Has(p, FeatFinancialQuant) {
			t.Fatalf("financial quantification must be on %s", p)
		}
	}
	if got := MinPlanFor(FeatFinancialQuant); got != PlanPro {
		t.Fatalf("MinPlanFor(financial) = %s, want pro", got)
	}
}

func TestFeatureGates_MatchMatrix(t *testing.T) {
	// A few load-bearing rows from the matrix the UI and enforcement rely on.
	if Has(PlanFree, FeatAutomation) || Has(PlanFree, FeatAIAdvisor) {
		t.Fatal("Free must not have automation/AI")
	}
	if !Has(PlanBusiness, FeatSSO) || Has(PlanPro, FeatSSO) {
		t.Fatal("SSO opens at Business")
	}
	if Has(PlanBusiness, FeatMultiTenant) || !Has(PlanEnterprise, FeatMultiTenant) {
		t.Fatal("multi-tenant is Enterprise-only")
	}
	if LevelOf(PlanBusiness, FeatAutomation) != LevelAdvanced {
		t.Fatal("Business automation should be advanced")
	}
	if LevelOf(PlanEnterprise, FeatCompliance) != LevelCustom {
		t.Fatal("Enterprise compliance should be custom")
	}
}

func TestLimits(t *testing.T) {
	if LimitOf(PlanFree, LimitUsers) != 2 || LimitOf(PlanFree, LimitRisks) != 50 {
		t.Fatal("Free limits wrong")
	}
	if LimitOf(PlanPro, LimitAssets) != Unlimited {
		t.Fatal("Pro assets should be unlimited")
	}
	if LimitOf(PlanBusiness, LimitRisks) != Unlimited {
		t.Fatal("Business risks should be unlimited")
	}
	// Unknown limit key on a known plan → unlimited (never accidentally caps).
	if LimitOf(PlanPro, LimitKey("nope")) != Unlimited {
		t.Fatal("unknown key should be unlimited")
	}
}

func TestWithinLimit(t *testing.T) {
	if !WithinLimit(PlanFree, LimitUsers, 1) {
		t.Fatal("1 user under cap of 2 should be allowed")
	}
	if WithinLimit(PlanFree, LimitUsers, 2) {
		t.Fatal("at cap of 2, creating a 3rd must be refused")
	}
	if !WithinLimit(PlanBusiness, LimitRisks, 10_000) {
		t.Fatal("unlimited must always allow")
	}
}

func TestPricing_PPP(t *testing.T) {
	eu := PriceFor(RegionEU, PlanPro)
	if eu.Amount != 49 || eu.Currency != "EUR" {
		t.Fatalf("EU Pro = %+v, want 49 EUR", eu)
	}
	af := PriceFor(RegionAfrica, PlanBusiness)
	if af.Currency != "XAF" || af.Amount != 39000 {
		t.Fatalf("Africa Business = %+v, want 39000 XAF", af)
	}
	if !PriceFor(RegionEU, PlanEnterprise).Custom {
		t.Fatal("Enterprise must be quote-based (custom)")
	}
	if PriceFor(RegionAfrica, PlanFree).Amount != 0 {
		t.Fatal("Free must be 0")
	}
}

func TestParseRegion(t *testing.T) {
	if ParseRegion("XAF") != RegionAfrica || ParseRegion("cm") != RegionAfrica {
		t.Fatal("Africa region parse")
	}
	if ParseRegion("") != RegionEU || ParseRegion("us") != RegionEU {
		t.Fatal("default region must be EU")
	}
}

// TestMatrixIntegrity guards against a half-edited matrix: every plan defines all
// limit keys, and every feature the matrix references is a known feature.
func TestMatrixIntegrity(t *testing.T) {
	known := map[Feature]bool{}
	for _, f := range AllFeatures {
		known[f] = true
	}
	for _, p := range AllPlans {
		e := For(p)
		if e.Plan != p {
			t.Errorf("matrix[%s].Plan = %s", p, e.Plan)
		}
		for _, k := range AllLimits {
			if _, ok := e.Limits[k]; !ok {
				t.Errorf("plan %s missing limit %s", p, k)
			}
		}
		for f := range e.Features {
			if !known[f] {
				t.Errorf("plan %s references unknown feature %q", p, f)
			}
		}
	}
	// Every marquee feature must be reachable on some plan.
	for _, f := range AllFeatures {
		if MinPlanFor(f) == PlanEnterprise && !Has(PlanEnterprise, f) {
			t.Errorf("feature %q is granted by no plan", f)
		}
	}
}
