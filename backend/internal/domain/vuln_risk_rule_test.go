// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func enabledRule() *VulnRiskRule {
	r := DefaultVulnRiskRule(uuid.New())
	r.Enabled = true
	return r
}

func vulnOn(assetCrit string, cvss float64) *Vulnerability {
	id := uuid.New()
	return &Vulnerability{
		CVSSScore: cvss, AssetID: &id, AssetCriticality: assetCrit,
	}
}

// A tenant who never configured the rule must not have risks appearing in their
// register. Automatic creation is a change to somebody's workload; it is asked
// for, not assumed.
func TestVulnRiskRule_DisabledByDefault(t *testing.T) {
	r := DefaultVulnRiskRule(uuid.New())
	if r.Enabled {
		t.Fatal("the default rule must be disabled")
	}
	d := EvaluateVulnRiskRule(r, vulnOn("CRITICAL", 10), true)
	if d.Create {
		t.Error("a disabled rule must never create")
	}
	if d.Reason == "" {
		t.Error("even a no-op decision must explain itself")
	}
}

// A nil rule is the same as no rule: never create.
func TestVulnRiskRule_NilIsSafe(t *testing.T) {
	if EvaluateVulnRiskRule(nil, vulnOn("CRITICAL", 10), true).Create {
		t.Error("a nil rule must never create")
	}
	if EvaluateVulnRiskRule(enabledRule(), nil, true).Create {
		t.Error("a nil vulnerability must never create")
	}
}

func TestVulnRiskRule_AllConditionsMustHold(t *testing.T) {
	r := enabledRule()
	r.MinCVSS = 7
	r.MinAssetCriticality = CriticalityHigh

	if d := EvaluateVulnRiskRule(r, vulnOn("CRITICAL", 9.8), false); !d.Create {
		t.Errorf("should create: %s", d.Reason)
	}
	// CVSS below the floor.
	if d := EvaluateVulnRiskRule(r, vulnOn("CRITICAL", 6.9), false); d.Create {
		t.Error("CVSS below the floor must not create")
	} else if !strings.Contains(d.Reason, "CVSS") {
		t.Errorf("the reason must name the condition that decided: %q", d.Reason)
	}
	// Criticality below the floor.
	if d := EvaluateVulnRiskRule(r, vulnOn("MEDIUM", 9.8), false); d.Create {
		t.Error("asset criticality below the floor must not create")
	}
}

// An asset whose criticality nobody set must NOT satisfy a "HIGH or above"
// floor — that would fire the rule on precisely the assets least understood.
func TestVulnRiskRule_UnknownCriticalityRanksBelowLow(t *testing.T) {
	r := enabledRule()
	r.MinCVSS = 0
	r.MinAssetCriticality = CriticalityHigh

	if d := EvaluateVulnRiskRule(r, vulnOn("", 9.8), false); d.Create {
		t.Error("an asset with no criticality must not satisfy a HIGH floor")
	}
	r.MinAssetCriticality = CriticalityLow
	if d := EvaluateVulnRiskRule(r, vulnOn("", 9.8), false); d.Create {
		t.Error("an asset with no criticality must not satisfy a LOW floor either")
	}
}

func TestVulnRiskRule_RequireAsset(t *testing.T) {
	r := enabledRule()
	r.MinCVSS = 0
	r.MinAssetCriticality = ""
	r.RequireAsset = true

	v := &Vulnerability{CVSSScore: 9.8} // no asset
	d := EvaluateVulnRiskRule(r, v, false)
	if d.Create {
		t.Error("an unattributed finding must not create a risk when an asset is required")
	}
	if !strings.Contains(d.Reason, "actif") {
		t.Errorf("the reason should point at the missing asset: %q", d.Reason)
	}

	r.RequireAsset = false
	if d := EvaluateVulnRiskRule(r, v, false); !d.Create {
		t.Errorf("with the requirement off it should create: %s", d.Reason)
	}
}

func TestVulnRiskRule_RequireKEVAndExposure(t *testing.T) {
	r := enabledRule()
	r.MinCVSS = 0
	r.MinAssetCriticality = ""
	r.RequireKEV = true
	r.RequireInternetExposure = true

	v := vulnOn("HIGH", 5.0)
	if EvaluateVulnRiskRule(r, v, true).Create {
		t.Error("a non-KEV finding must not create when KEV is required")
	}
	v.KEV = true
	if EvaluateVulnRiskRule(r, v, false).Create {
		t.Error("an unexposed asset must not create when exposure is required")
	}
	d := EvaluateVulnRiskRule(r, v, true)
	if !d.Create {
		t.Errorf("both conditions met, should create: %s", d.Reason)
	}
	if len(d.MatchedConditions) < 2 {
		t.Errorf("both satisfied conditions should be reported: %v", d.MatchedConditions)
	}
}

// "Why did this NOT create a risk?" is the question a tuned rule gets asked
// most, so a negative decision must be as explained as a positive one.
func TestVulnRiskRule_NegativeDecisionsAreExplained(t *testing.T) {
	r := enabledRule()
	for _, v := range []*Vulnerability{
		{CVSSScore: 9.8},                      // no asset
		vulnOn("LOW", 9.8),                    // criticality too low
		vulnOn("CRITICAL", 1.0),               // CVSS too low
	} {
		d := EvaluateVulnRiskRule(r, v, false)
		if d.Create {
			t.Fatalf("unexpected create for %+v", v)
		}
		if strings.TrimSpace(d.Reason) == "" {
			t.Errorf("no reason given for %+v", v)
		}
	}
}

// A rule that cannot possibly fire must be rejected at save time rather than
// silently doing nothing forever.
func TestVulnRiskRule_ValidateRejectsUnsatisfiableRules(t *testing.T) {
	r := DefaultVulnRiskRule(uuid.New())
	r.RequireAsset = false
	r.MinAssetCriticality = CriticalityHigh
	if err := r.Validate(); err == nil {
		t.Error("a criticality floor without requiring an asset is unsatisfiable and must be rejected")
	}

	r = DefaultVulnRiskRule(uuid.New())
	r.RequireAsset = false
	r.MinAssetCriticality = ""
	r.RequireInternetExposure = true
	if err := r.Validate(); err == nil {
		t.Error("requiring exposure without requiring an asset must be rejected")
	}

	r = DefaultVulnRiskRule(uuid.New())
	r.MinCVSS = 11
	if err := r.Validate(); err == nil {
		t.Error("a CVSS floor above 10 must be rejected")
	}

	r = DefaultVulnRiskRule(uuid.New())
	r.MinAssetCriticality = "SEVERE"
	if err := r.Validate(); err == nil {
		t.Error("an unknown criticality must be rejected")
	}

	if err := DefaultVulnRiskRule(uuid.New()).Validate(); err != nil {
		t.Errorf("the shipped default must be valid: %v", err)
	}
}
