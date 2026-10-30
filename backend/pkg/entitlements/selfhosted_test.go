// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import "testing"

// TestSelfHosted_MatchesD040 asserts the self-hosted set row by row against the
// table of #612 / D-040, not a spot check: every limit and every feature in
// AllLimits / AllFeatures must appear here, so a new key cannot slip in unseen.
func TestSelfHosted_MatchesD040(t *testing.T) {
	for _, k := range AllLimits {
		if got := LimitOf(PlanSelfHosted, k); got != Unlimited {
			t.Errorf("limit %s = %d, want unlimited (it is the operator's hardware)", k, got)
		}
	}

	want := map[Feature]Level{
		FeatSmartScore:         LevelOn,
		FeatFinancialQuant:     LevelOn,
		FeatExecutiveDashboard: LevelOn,
		FeatScanner:            LevelOn,       // the agent registers with their own instance
		FeatCTI:                LevelAdvanced, // NVD and CISA KEV are free public feeds
		FeatCompliance:         LevelCustom,
		FeatAutomation:         LevelAdvanced,
		FeatGovernance:         LevelAdvanced,
		FeatAPI:                LevelOn,
		FeatMultiTenant:        LevelOn,
		FeatOnPremise:          LevelOn,
		FeatVendorRisk:         LevelOn,
		FeatAIAdvisor:          LevelAdvanced, // operator's own key, or the template fallback
		FeatSupport:            LevelCommunity,
		FeatSLA:                LevelOff, // a service the owner renders
		FeatSSO:                LevelOn,  // configured by the operator via OAUTH2_*; SAML stays off (#866)
	}
	for _, f := range AllFeatures {
		w, ok := want[f]
		if !ok {
			t.Errorf("feature %s has no row in the self-hosted table — decide it against D-040", f)
			continue
		}
		if got := LevelOf(PlanSelfHosted, f); got != w {
			t.Errorf("feature %s = %q, want %q", f, got, w)
		}
	}
}

// TestSelfHosted_NeverBelowAHostedPlan keeps the set complete as the hosted
// matrix moves: whatever depth a hosted plan grants, self-hosted grants at least
// that, except the rows that are services the owner renders.
func TestSelfHosted_NeverBelowAHostedPlan(t *testing.T) {
	services := map[Feature]bool{FeatSLA: true, FeatSupport: true}
	for _, p := range AllPlans {
		for _, f := range AllFeatures {
			if services[f] {
				continue
			}
			hosted := LevelOf(p, f)
			if hosted.Enabled() && !LevelOf(PlanSelfHosted, f).AtLeast(hosted) {
				t.Errorf("%s grants %s at %q, self-hosted only %q", p, f, hosted, LevelOf(PlanSelfHosted, f))
			}
		}
	}
}

// TestSelfHosted_Unauthorized: no stored or tenant-supplied plan string reaches
// the self-hosted set. Only the deployment flag does.
func TestSelfHosted_Unauthorized(t *testing.T) {
	for _, s := range []string{"self_hosted", "SELF_HOSTED", "self-hosted", "selfhosted"} {
		if got := ParsePlan(s); got != PlanFree {
			t.Errorf("ParsePlan(%q) = %s, want free", s, got)
		}
	}
	for _, p := range AllPlans {
		if p == PlanSelfHosted {
			t.Fatal("PlanSelfHosted must not be a hosted plan: it would get a price and a checkout")
		}
	}
}

func TestParseDeployment_Success(t *testing.T) {
	cases := map[string]Deployment{
		"":              DeploymentHosted,
		"hosted":        DeploymentHosted,
		"SaaS":          DeploymentHosted,
		"self_hosted":   DeploymentSelfHosted,
		" Self-Hosted ": DeploymentSelfHosted,
		"selfhosted":    DeploymentSelfHosted,
	}
	for in, want := range cases {
		got, err := ParseDeployment(in)
		if err != nil || got != want {
			t.Errorf("ParseDeployment(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// TestParseDeployment_Invalid: a typo refuses to boot rather than silently
// capping a self-hoster at Free.
func TestParseDeployment_Invalid(t *testing.T) {
	for _, in := range []string{"self", "selfhost", "on-prem", "true"} {
		if _, err := ParseDeployment(in); err == nil {
			t.Errorf("ParseDeployment(%q) accepted a value it does not know", in)
		}
	}
}
