// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scoring

import (
	"math"
	"testing"
)

// almost compares floats with a small tolerance.
func almost(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestComputeSmart_ZeroInputIsLow(t *testing.T) {
	// An empty risk: no asset, no vulns, no exposure, no compliance data. The only
	// non-zero factor is control maturity (neutral 0.5 when unassessed).
	res := ComputeSmart(SmartInput{}, nil)

	if res.Criticality != CriticalityLow {
		t.Fatalf("expected low criticality, got %s (score %.2f)", res.Criticality, res.Score)
	}
	// With defaults only the two "unknown → neutral" factors contribute: business
	// criticality (unknown → MEDIUM 0.5 × 0.15) + control maturity (unassessed →
	// 0.5 × 0.10) = 12.5 pts. Everything else is 0. Still comfortably Low.
	if res.Score > 15 {
		t.Fatalf("expected a small score for an empty risk, got %.2f", res.Score)
	}
	if len(res.Factors) != len(FactorKeys) {
		t.Fatalf("expected %d factors, got %d", len(FactorKeys), len(res.Factors))
	}
}

func TestComputeSmart_WeightsNormalised(t *testing.T) {
	// Weights that do NOT sum to 1 must be normalised, and the applied weights must
	// sum to 1 across the eight factors.
	res := ComputeSmart(SmartInput{}, FactorWeights{
		FactorVulnerabilities: 10, // arbitrary scale
		FactorExploitability:  10,
	})
	var sum float64
	for _, f := range res.Factors {
		sum += f.Weight
	}
	if !almost(sum, 1.0) {
		t.Fatalf("normalised weights must sum to 1.0, got %.4f", sum)
	}
}

func TestComputeSmart_WorstCaseSaturates(t *testing.T) {
	// Everything maxed: public, critical asset, many high-CVSS vulns, actively
	// exploited, huge financial exposure, no controls, repeated incidents.
	res := ComputeSmart(SmartInput{
		BusinessCriticalityFactor: 3.0,
		InternetExposure:          1.0,
		VulnerabilityCount:        25,
		MaxCVSS:                   10.0,
		ControlMaturity:           0.0,
		ControlsAssessed:          true,
		IncidentCount:             12,
		EPSS:                      0.98,
		KEV:                       true,
		ExploitAvailable:          true,
		ExploitMaturity:           "high",
		ALEXAF:                    500_000_000,
		ActiveThreatSignal:        1.0,
	}, nil)

	if res.Criticality != CriticalityCritical {
		t.Fatalf("expected critical, got %s (score %.2f)", res.Criticality, res.Score)
	}
	if res.Score < 90 {
		t.Fatalf("worst-case score should be near 100, got %.2f", res.Score)
	}
	if res.Score > 100 {
		t.Fatalf("score must never exceed 100, got %.2f", res.Score)
	}
}

func TestComputeSmart_MatureControlsLowerRisk(t *testing.T) {
	base := SmartInput{
		BusinessCriticalityFactor: 2.5,
		VulnerabilityCount:        3,
		MaxCVSS:                   8.0,
		ControlsAssessed:          true,
	}
	weak := base
	weak.ControlMaturity = 0.0
	strong := base
	strong.ControlMaturity = 1.0

	rWeak := ComputeSmart(weak, nil)
	rStrong := ComputeSmart(strong, nil)

	if rStrong.Score >= rWeak.Score {
		t.Fatalf("mature controls must lower the score: weak=%.2f strong=%.2f", rWeak.Score, rStrong.Score)
	}
}

func TestComputeSmart_KEVDrivesExploitabilityAndThreat(t *testing.T) {
	// A KEV-flagged risk on a modest asset should still land meaningfully above a
	// pristine one, driven by exploitability + threat-intel factors.
	pristine := ComputeSmart(SmartInput{BusinessCriticalityFactor: 1.5}, nil)
	exploited := ComputeSmart(SmartInput{
		BusinessCriticalityFactor: 1.5,
		KEV:                       true,
		ExploitAvailable:          true,
		ActiveThreatSignal:        1.0,
	}, nil)

	if exploited.Score <= pristine.Score {
		t.Fatalf("KEV/exploit/threat must raise the score: pristine=%.2f exploited=%.2f", pristine.Score, exploited.Score)
	}
	// exploitability factor value should be high (>=0.9).
	var expVal float64
	for _, f := range exploited.Factors {
		if f.Key == FactorExploitability {
			expVal = f.Value
		}
	}
	if expVal < 0.9 {
		t.Fatalf("KEV should push exploitability value near 1.0, got %.2f", expVal)
	}
}

func TestComputeSmart_FinancialFactorScalesWithReference(t *testing.T) {
	// The same ALE against a smaller reference contributes more.
	small := ComputeSmart(SmartInput{ALEXAF: 20_000_000, ALEReferenceXAF: 20_000_000}, nil)
	large := ComputeSmart(SmartInput{ALEXAF: 20_000_000, ALEReferenceXAF: 200_000_000}, nil)

	var fSmall, fLarge float64
	for _, f := range small.Factors {
		if f.Key == FactorFinancialValue {
			fSmall = f.Value
		}
	}
	for _, f := range large.Factors {
		if f.Key == FactorFinancialValue {
			fLarge = f.Value
		}
	}
	if !almost(fSmall, 1.0) {
		t.Fatalf("ALE == reference should saturate the financial factor to 1.0, got %.2f", fSmall)
	}
	if fLarge >= fSmall {
		t.Fatalf("a larger reference must lower the financial factor: small=%.2f large=%.2f", fSmall, fLarge)
	}
}

func TestComputeSmart_ContributionsSumToScore(t *testing.T) {
	res := ComputeSmart(SmartInput{
		BusinessCriticalityFactor: 3.0,
		InternetExposure:          0.6,
		VulnerabilityCount:        4,
		MaxCVSS:                   7.5,
		ControlsAssessed:          true,
		ControlMaturity:           0.4,
		IncidentCount:             2,
		EPSS:                      0.3,
		ALEXAF:                    30_000_000,
		ActiveThreatSignal:        0.4,
	}, nil)

	var sum float64
	for _, f := range res.Factors {
		sum += f.Contribution
	}
	if !almost(sum, res.Score) {
		t.Fatalf("factor contributions (%.2f) must sum to the score (%.2f)", sum, res.Score)
	}
}

func TestDefaultFactorWeights_SumToOneAndIsolated(t *testing.T) {
	w := DefaultFactorWeights()
	var sum float64
	for _, k := range FactorKeys {
		sum += w[k]
	}
	if !almost(sum, 1.0) {
		t.Fatalf("default weights should sum to 1.0, got %.4f", sum)
	}
	// Mutating the returned copy must not affect the package defaults.
	w[FactorVulnerabilities] = 99
	if DefaultFactorWeights()[FactorVulnerabilities] == 99 {
		t.Fatal("DefaultFactorWeights must return an isolated copy")
	}
}

func TestExposureFromLabel(t *testing.T) {
	cases := map[string]float64{
		"public":          1.0,
		"internet-facing": 1.0,
		"dmz":             0.6,
		"internal":        0.1,
		"whatever":        0.5,
	}
	for label, want := range cases {
		if got := ExposureFromLabel(label); !almost(got, want) {
			t.Fatalf("ExposureFromLabel(%q) = %.2f, want %.2f", label, got, want)
		}
	}
}
