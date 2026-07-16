// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package vulnprio

import (
	"strings"
	"testing"
)

func TestCompute_KEVFloorsToP1(t *testing.T) {
	// A medium CVSS with no asset context would score low — but CISA-KEV must
	// floor it into P1 ("patch now").
	r := Compute(Input{CVSS: 5.0, KEV: true, AssetCriticalityFactor: 0.5, AffectedAssets: 1})
	if r.Score < kevFloor {
		t.Errorf("KEV should floor score to >= %.0f, got %.2f", kevFloor, r.Score)
	}
	if r.Tier != "P1" {
		t.Errorf("KEV vuln should be P1, got %s", r.Tier)
	}
	if !strings.Contains(r.Explanation, "KEV") {
		t.Errorf("explanation should mention KEV: %q", r.Explanation)
	}
}

func TestCompute_CriticalEverything_IsP1(t *testing.T) {
	r := Compute(Input{CVSS: 9.8, EPSS: 0.9, ExploitAvailable: true, AssetCriticalityFactor: 3.0, AffectedAssets: 20})
	if r.Tier != "P1" || r.Score < 90 {
		t.Errorf("critical-everything should be a high P1, got tier=%s score=%.2f", r.Tier, r.Score)
	}
}

func TestCompute_LowEverything_IsP4(t *testing.T) {
	r := Compute(Input{CVSS: 2.0, EPSS: 0.0, AssetCriticalityFactor: 0.5, AffectedAssets: 1})
	if r.Tier != "P4" {
		t.Errorf("low-everything should be P4, got tier=%s score=%.2f", r.Tier, r.Score)
	}
}

func TestCompute_BusinessCriticalityMatters(t *testing.T) {
	// Same CVE, different asset criticality → the critical asset must rank higher.
	base := Input{CVSS: 7.5, EPSS: 0.1, AffectedAssets: 1}
	low := base
	low.AssetCriticalityFactor = 0.5
	crit := base
	crit.AssetCriticalityFactor = 3.0
	if Compute(crit).Score <= Compute(low).Score {
		t.Error("a vuln on a CRITICAL asset must outrank the same vuln on a LOW asset")
	}
}

func TestCompute_BlastRadiusMatters(t *testing.T) {
	base := Input{CVSS: 6.0, AssetCriticalityFactor: 1.5}
	one := base
	one.AffectedAssets = 1
	many := base
	many.AffectedAssets = 50
	if Compute(many).Score <= Compute(one).Score {
		t.Error("more affected assets must raise the priority")
	}
}

func TestCompute_ExploitAvailableRaisesScore(t *testing.T) {
	base := Input{CVSS: 7.0, AssetCriticalityFactor: 1.5, AffectedAssets: 1}
	no := base
	yes := base
	yes.ExploitAvailable = true
	if Compute(yes).Score <= Compute(no).Score {
		t.Error("a public exploit must raise the priority")
	}
}

func TestCompute_ScoreBounded(t *testing.T) {
	r := Compute(Input{CVSS: 10, EPSS: 1, KEV: true, ExploitAvailable: true, ExploitMaturity: "high", AssetCriticalityFactor: 3.0, AffectedAssets: 10000})
	if r.Score > 100 {
		t.Errorf("score must be capped at 100, got %.2f", r.Score)
	}
	empty := Compute(Input{})
	if empty.Score < 0 {
		t.Errorf("score must be >= 0, got %.2f", empty.Score)
	}
}

func TestTierBoundaries(t *testing.T) {
	cases := []struct {
		score float64
		tier  string
	}{{80, "P1"}, {79.99, "P2"}, {60, "P2"}, {59.99, "P3"}, {40, "P3"}, {39.99, "P4"}, {0, "P4"}}
	for _, c := range cases {
		if got := tierFor(c.score); got != c.tier {
			t.Errorf("tierFor(%.2f) = %s, want %s", c.score, got, c.tier)
		}
	}
}
