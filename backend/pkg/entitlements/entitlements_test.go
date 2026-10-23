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
		"":           PlanFree, "nonsense": PlanFree, "  PRO  ": PlanPro,
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
	// No plan sells SSO while there is no tenant SSO to sell (#866, #1045).
	for _, p := range AllPlans {
		if Has(p, FeatSSO) {
			t.Fatalf("%s grants SSO, which no tenant can configure", p)
		}
	}
	if Has(PlanBusiness, FeatMultiTenant) || !Has(PlanEnterprise, FeatMultiTenant) {
		t.Fatal("multi-tenant is Enterprise-only")
	}
	if LevelOf(PlanBusiness, FeatAutomation) != LevelAdvanced {
		t.Fatal("Business automation should be advanced")
	}
}

// TestCompliance_IsOneModuleCappedByFrameworkCount pins D-076: every plan gets
// the same compliance module, with no tier, and the plan caps how many
// frameworks are active at once. Changing a number here is a pricing change.
func TestCompliance_IsOneModuleCappedByFrameworkCount(t *testing.T) {
	want := map[Plan]int{PlanFree: 1, PlanPro: 3, PlanBusiness: 10, PlanEnterprise: Unlimited}
	for _, p := range AllPlans {
		if got := LevelOf(p, FeatCompliance); got != LevelOn {
			t.Errorf("%s grants compliance at %q, want %q: D-076 removed the tiers", p, got, LevelOn)
		}
		if got := LimitOf(p, LimitFrameworks); got != want[p] {
			t.Errorf("%s caps frameworks at %d, want %d (D-076)", p, got, want[p])
		}
	}
	if !WithinLimit(PlanFree, LimitFrameworks, 0) || WithinLimit(PlanFree, LimitFrameworks, 1) {
		t.Error("Free admits its first framework and refuses the second")
	}
	if !WithinLimit(PlanEnterprise, LimitFrameworks, 1000) {
		t.Error("Enterprise is uncapped")
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
	// Every marquee feature must be reachable on some plan, except the ones no
	// plan sells yet, each with the reason.
	notSold := map[Feature]string{
		FeatSSO: "SAML is off since #866; tenant SSO comes back with #1045",
	}
	for _, f := range AllFeatures {
		if _, ok := notSold[f]; ok {
			continue
		}
		if MinPlanFor(f) == PlanEnterprise && !Has(PlanEnterprise, f) {
			t.Errorf("feature %q is granted by no plan", f)
		}
	}
}

// TestLevelOrdering is the one declaration of the depth order, as a table:
// off < limited < basic < on = standard < advanced < custom (#613).
func TestLevelOrdering(t *testing.T) {
	ladder := []Level{LevelOff, LevelLimited, LevelBasic, LevelOn, LevelAdvanced, LevelCustom}
	for i, lo := range ladder {
		for j, hi := range ladder {
			if got, want := hi.AtLeast(lo), j >= i; got != want {
				t.Errorf("%q.AtLeast(%q) = %v, want %v", hi, lo, got, want)
			}
		}
	}

	// on and standard are the same depth, named for rows with and without tiers.
	if !LevelOn.AtLeast(LevelStandard) || !LevelStandard.AtLeast(LevelOn) {
		t.Error("on and standard must rank together")
	}
	if !LevelStandard.AtLeast(LevelBasic) || LevelStandard.AtLeast(LevelAdvanced) {
		t.Error("standard must sit between basic and advanced")
	}
}

// TestLevelDescriptors_AreNotOrdered keeps support and SLA values out of the
// ordering: they describe a plan, they are not depths anything can gate on.
func TestLevelDescriptors_AreNotOrdered(t *testing.T) {
	for _, d := range []Level{LevelCommunity, LevelEmail, LevelPriority, LevelDedicated, Level("99.5"), Level("99.9")} {
		if d.Ordered() {
			t.Errorf("%q must be a descriptor, not a depth", d)
		}
		if d.AtLeast(LevelOff) || LevelCustom.AtLeast(d) {
			t.Errorf("%q must compare to nothing", d)
		}
	}
	// Every level the matrix grants outside support/SLA is a depth.
	for _, p := range AllPlans {
		for f, l := range For(p).Features {
			if f == FeatSupport || f == FeatSLA {
				continue
			}
			if !l.Ordered() {
				t.Errorf("%s / %s grants %q, which is not in the depth ordering", p, f, l)
			}
		}
	}
}

func TestMinPlanForLevel(t *testing.T) {
	cases := []struct {
		f    Feature
		min  Level
		want Plan
	}{
		{FeatCompliance, LevelOn, PlanFree},
		{FeatCompliance, LevelAdvanced, PlanEnterprise},
		{FeatAIAdvisor, LevelAdvanced, PlanBusiness},
		{FeatFinancialQuant, LevelBasic, PlanPro},
		{FeatFinancialQuant, LevelOn, PlanPro},
		{FeatAutomation, LevelAdvanced, PlanBusiness},
		{FeatAPI, LevelOn, PlanPro},
	}
	for _, tc := range cases {
		if got := MinPlanForLevel(tc.f, tc.min); got != tc.want {
			t.Errorf("MinPlanForLevel(%s, %q) = %s, want %s", tc.f, tc.min, got, tc.want)
		}
	}
}

// TestFinancialDashboard_OpensAtPro pins the floor of the financial dashboard
// (#613): its routes need the Monte-Carlo depth ("on"), and no plan below Pro
// may grant that depth. A matrix edit giving Free a basic grant (#415) keeps
// the dashboard at Pro; one giving Free "on" fails here and needs the owner.
func TestFinancialDashboard_OpensAtPro(t *testing.T) {
	if got := MinPlanForLevel(FeatFinancialQuant, LevelOn); got != PlanPro {
		t.Fatalf("the Monte-Carlo depth opens at %s, want pro", got)
	}
	for _, p := range AllPlans {
		if !p.AtLeast(PlanPro) && LevelOf(p, FeatFinancialQuant).AtLeast(LevelOn) {
			t.Errorf("%s grants financial quantification at %q: the dashboard would open below Pro",
				p, LevelOf(p, FeatFinancialQuant))
		}
	}
}
