// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crq

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestDowntimeCostXAF(t *testing.T) {
	if got := DowntimeCostXAF(f(8), f(500_000)); got != 4_000_000 {
		t.Fatalf("downtime = %v, want 4000000", got)
	}
	// Missing either driver → 0.
	if got := DowntimeCostXAF(nil, f(500_000)); got != 0 {
		t.Fatalf("downtime(nil hours) = %v, want 0", got)
	}
	if got := DowntimeCostXAF(f(8), nil); got != 0 {
		t.Fatalf("downtime(nil rate) = %v, want 0", got)
	}
}

func TestROSI(t *testing.T) {
	// ALE before 10M, after 2M (80% effective), remediation 3M.
	// ROSI = (10M − 2M − 3M) / 3M = 5M/3M = 1.6667.
	got, ok := ROSI(10_000_000, 2_000_000, 3_000_000)
	if !ok {
		t.Fatal("ROSI not computable, want computable")
	}
	if !approx(got, 1.67) {
		t.Fatalf("ROSI = %v, want ~1.67", got)
	}
	// Zero remediation cost → undefined.
	if _, ok := ROSI(10_000_000, 2_000_000, 0); ok {
		t.Fatal("ROSI with 0 cost should be non-computable")
	}
	// Negative ROSI: control costs more than it saves.
	got, ok = ROSI(1_000_000, 500_000, 2_000_000)
	if !ok || got >= 0 {
		t.Fatalf("expected negative ROSI, got %v (ok=%v)", got, ok)
	}
}

// SLE composed from its components: downtime + fines + data loss + other.
func TestAssess_ComposedSLE(t *testing.T) {
	q := NewQuantifier(600, DefaultReference())
	a := q.Assess(FinancialInputs{
		DowntimeHours:         f(10),
		HourlyDowntimeCostXAF: f(200_000), // 2M downtime
		FinesXAF:              f(3_000_000),
		DataLossCostXAF:       f(1_000_000),
		OtherDirectCostXAF:    f(500_000),
		ARO:                   f(2),
	}, "high")

	if a.SLEBasis != BasisComposed {
		t.Fatalf("SLE basis = %q, want composed", a.SLEBasis)
	}
	if a.DowntimeCost.XAF != 2_000_000 {
		t.Fatalf("downtime = %v, want 2000000", a.DowntimeCost.XAF)
	}
	// SLE = 2M + 3M + 1M + 0.5M = 6.5M.
	if a.SLE.XAF != 6_500_000 {
		t.Fatalf("SLE = %v, want 6500000", a.SLE.XAF)
	}
	// ALE = composed SLE × ARO = 6.5M × 2 = 13M (annualized from composed SLE).
	if a.ALE.XAF != 13_000_000 {
		t.Fatalf("ALE = %v, want 13000000", a.ALE.XAF)
	}
	// USD conversion at 600.
	if !approx(a.SLE.USD, 10_833.33) {
		t.Fatalf("SLE USD = %v, want ~10833.33", a.SLE.USD)
	}
}

// Explicit SLE wins over components and drives the ROSI computation.
func TestAssess_ExplicitSLEAndROSI(t *testing.T) {
	q := NewQuantifier(600, DefaultReference())
	a := q.Assess(FinancialInputs{
		SLEXAF:                  f(20_000_000),
		ARO:                     f(0.5), // ALE = 10M
		RemediationCostXAF:      f(3_000_000),
		MitigationEffectiveness: f(0.8),
	}, "critical")

	if a.SLEBasis != BasisExplicit {
		t.Fatalf("SLE basis = %q, want explicit", a.SLEBasis)
	}
	if a.ALE.XAF != 10_000_000 {
		t.Fatalf("ALE = %v, want 10000000", a.ALE.XAF)
	}
	// ALE after = 10M × (1 − 0.8) = 2M.
	if a.ALEAfter.XAF != 2_000_000 {
		t.Fatalf("ALE after = %v, want 2000000", a.ALEAfter.XAF)
	}
	// Risk reduction = 8M.
	if a.RiskReduction.XAF != 8_000_000 {
		t.Fatalf("risk reduction = %v, want 8000000", a.RiskReduction.XAF)
	}
	// ROSI = (10M − 2M − 3M) / 3M = 1.6667.
	if !a.ROSIComputable || !approx(a.ROSI, 1.67) {
		t.Fatalf("ROSI = %v (ok=%v), want ~1.67", a.ROSI, a.ROSIComputable)
	}
}

// Worst / average loss modelling (derived band).
func TestAssess_LossBand(t *testing.T) {
	q := NewQuantifier(600, DefaultReference())
	a := q.Assess(FinancialInputs{SLEXAF: f(10_000_000), ARO: f(1)}, "high")
	// Derived band: best = 5M, worst = 20M.
	if a.SLEWorst.XAF != 20_000_000 {
		t.Fatalf("SLE worst = %v, want 20000000", a.SLEWorst.XAF)
	}
	// PERT average = (5M + 4×10M + 20M)/6 = 65M/6 = 10.83M.
	if !approx(a.SLEAverage.XAF, 10_833_333.33) {
		t.Fatalf("SLE average = %v, want ~10833333.33", a.SLEAverage.XAF)
	}
	// Annualized worst (ARO=1) equals worst single loss.
	if a.ALEWorst.XAF != 20_000_000 {
		t.Fatalf("ALE worst = %v, want 20000000", a.ALEWorst.XAF)
	}
}

// Explicit worst/best bounds override the derived band and stay coherent.
func TestAssess_ExplicitBand(t *testing.T) {
	q := NewQuantifier(600, DefaultReference())
	a := q.Assess(FinancialInputs{
		SLEXAF:      f(10_000_000),
		SLEWorstXAF: f(40_000_000),
		SLEBestXAF:  f(8_000_000),
		ARO:         f(1),
	}, "high")
	if a.SLEWorst.XAF != 40_000_000 {
		t.Fatalf("explicit worst = %v, want 40000000", a.SLEWorst.XAF)
	}
	// A worst bound below the point estimate is coerced up to it.
	b := q.Assess(FinancialInputs{SLEXAF: f(10_000_000), SLEWorstXAF: f(1_000_000)}, "high")
	if b.SLEWorst.XAF != 10_000_000 {
		t.Fatalf("incoherent worst = %v, want floored to 10000000", b.SLEWorst.XAF)
	}
}

// No inputs at all → reference band, ROSI non-computable, effectiveness clamped.
func TestAssess_ReferenceFallback(t *testing.T) {
	q := NewQuantifier(600, DefaultReference())
	a := q.Assess(FinancialInputs{}, "critical")
	if a.SLEBasis != BasisReference {
		t.Fatalf("SLE basis = %q, want reference", a.SLEBasis)
	}
	if a.SLE.XAF != 50_000_000 {
		t.Fatalf("SLE = %v, want reference 50000000", a.SLE.XAF)
	}
	if a.ROSIComputable {
		t.Fatal("ROSI should be non-computable with no remediation cost")
	}
	// Effectiveness clamps into [0,1].
	over := q.Assess(FinancialInputs{SLEXAF: f(1_000_000), ARO: f(1), MitigationEffectiveness: f(1.7)}, "low")
	if over.Effectiveness != 1 {
		t.Fatalf("effectiveness = %v, want clamped to 1", over.Effectiveness)
	}
}
