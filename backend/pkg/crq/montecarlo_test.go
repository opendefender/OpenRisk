// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crq

import (
	"math"
	"testing"
)

// TestSimulate_Determinism — the same input (including seed) must always yield
// the identical band. An auditable figure cannot wobble between runs.
func TestSimulate_Determinism(t *testing.T) {
	in := SimulationInput{
		LEF:        0.5,
		LM:         PERT{Min: 10_000_000, Mode: 20_000_000, Max: 40_000_000},
		Iterations: 20_000,
		Seed:       12345,
	}
	a := Simulate(in)
	b := Simulate(in)
	if a != b {
		t.Fatalf("non-deterministic: %+v != %+v", a, b)
	}
	// A different seed should move the percentiles (but keep them ordered).
	in.Seed = 999
	c := Simulate(in)
	if a.P50 == c.P50 && a.P10 == c.P10 && a.P90 == c.P90 {
		t.Fatalf("different seed produced identical percentiles — suspicious")
	}
}

// TestSimulate_Bounds — ordering and containment properties that must hold for
// any well-formed input.
func TestSimulate_Bounds(t *testing.T) {
	in := SimulationInput{
		LEF:        1.2,
		LM:         PERT{Min: 5_000_000, Mode: 12_000_000, Max: 30_000_000},
		Iterations: 30_000,
		Seed:       DefaultSeed,
	}
	d := Simulate(in)
	if !(d.P10 <= d.P50 && d.P50 <= d.P90) {
		t.Fatalf("percentiles not ordered: P10=%.0f P50=%.0f P90=%.0f", d.P10, d.P50, d.P90)
	}
	lo := in.LEF * in.LM.Min
	hi := in.LEF * in.LM.Max
	if d.Min < lo-1 || d.Max > hi+1 {
		t.Fatalf("samples out of [LEF·min, LEF·max]=[%.0f,%.0f]: min=%.0f max=%.0f", lo, hi, d.Min, d.Max)
	}
	if d.P10 < d.Min-1 || d.P90 > d.Max+1 {
		t.Fatalf("percentiles outside observed range")
	}
	if d.FormulaVersion != FormulaVersion {
		t.Fatalf("formula version not stamped: %q", d.FormulaVersion)
	}
}

// TestSimulate_Convergence — the Monte Carlo mean must converge to the analytic
// expectation LEF × PERT.ExpectedValue() as iterations grow.
func TestSimulate_Convergence(t *testing.T) {
	lm := PERT{Min: 2_000_000, Mode: 8_000_000, Max: 26_000_000}
	lef := 0.75
	want := lef * lm.ExpectedValue()
	d := Simulate(SimulationInput{LEF: lef, LM: lm, Iterations: 200_000, Seed: DefaultSeed})
	rel := math.Abs(d.Mean-want) / want
	if rel > 0.01 { // within 1%
		t.Fatalf("mean did not converge: got %.0f want %.0f (%.3f%% off)", d.Mean, want, rel*100)
	}
}

// TestSimulate_Degenerate — a zero-spread magnitude collapses to a deterministic
// point at LEF × mode.
func TestSimulate_Degenerate(t *testing.T) {
	lef := 2.0
	mode := 9_000_000.0
	d := Simulate(SimulationInput{LEF: lef, LM: PERT{Min: mode, Mode: mode, Max: mode}, Seed: 1})
	want := lef * mode
	for _, v := range []float64{d.P10, d.P50, d.P90, d.Mean} {
		if v != want {
			t.Fatalf("degenerate case not a point: got %.0f want %.0f", v, want)
		}
	}
}

// TestSimulate_ZeroLEF — no frequency means no loss.
func TestSimulate_ZeroLEF(t *testing.T) {
	d := Simulate(SimulationInput{LEF: 0, LM: PERT{Min: 1e6, Mode: 2e6, Max: 5e6}, Seed: 1})
	if d.P50 != 0 || d.Mean != 0 || d.P90 != 0 {
		t.Fatalf("zero LEF should yield zero loss, got %+v", d)
	}
}

// TestSimulate_UnorderedInput — a malformed (min>max, mode outside) input is
// normalized rather than producing garbage or a negative loss.
func TestSimulate_UnorderedInput(t *testing.T) {
	d := Simulate(SimulationInput{LEF: 1, LM: PERT{Min: 40_000_000, Mode: -5, Max: 10_000_000}, Iterations: 5000, Seed: 3})
	if d.Min < 0 || d.P10 < 0 {
		t.Fatalf("normalization failed, negative loss: %+v", d)
	}
	if !(d.P10 <= d.P50 && d.P50 <= d.P90) {
		t.Fatalf("percentiles not ordered after normalization")
	}
}

// TestSimulatePortfolio — the portfolio band is deterministic, ordered, and its
// mean equals the sum of per-risk analytic means (diversification affects the
// spread, not the mean).
func TestSimulatePortfolio(t *testing.T) {
	inputs := []SimulationInput{
		{LEF: 0.5, LM: PERT{Min: 5e6, Mode: 10e6, Max: 20e6}},
		{LEF: 1.0, LM: PERT{Min: 2e6, Mode: 4e6, Max: 9e6}},
		{LEF: 0.2, LM: PERT{Min: 20e6, Mode: 40e6, Max: 80e6}},
	}
	a := SimulatePortfolio(inputs, 100_000, DefaultSeed)
	b := SimulatePortfolio(inputs, 100_000, DefaultSeed)
	if a != b {
		t.Fatalf("portfolio not deterministic")
	}
	if !(a.P10 <= a.P50 && a.P50 <= a.P90) {
		t.Fatalf("portfolio percentiles not ordered: %+v", a)
	}
	var want float64
	for _, in := range inputs {
		want += in.LEF * in.LM.ExpectedValue()
	}
	if rel := math.Abs(a.Mean-want) / want; rel > 0.01 {
		t.Fatalf("portfolio mean off: got %.0f want %.0f (%.2f%%)", a.Mean, want, rel*100)
	}
	// Empty portfolio → zero band, still versioned.
	e := SimulatePortfolio(nil, 1000, 1)
	if e.P50 != 0 || e.FormulaVersion != FormulaVersion {
		t.Fatalf("empty portfolio wrong: %+v", e)
	}
}

func TestPERT_ExpectedValue(t *testing.T) {
	p := PERT{Min: 0, Mode: 10, Max: 20}
	if got := p.ExpectedValue(); got != 10 {
		t.Fatalf("PERT mean: got %.4f want 10", got)
	}
	p = PERT{Min: 1_000_000, Mode: 2_000_000, Max: 5_000_000}
	want := (1_000_000.0 + 4*2_000_000 + 5_000_000) / 6
	if got := p.ExpectedValue(); math.Abs(got-want) > 0.01 {
		t.Fatalf("PERT mean: got %.2f want %.2f", got, want)
	}
}

// TestPresenter_PresentDistribution — the band is converted into the tenant
// currency at the reference rate, with XAF preserved.
func TestPresenter_PresentDistribution(t *testing.T) {
	d := Simulate(SimulationInput{LEF: 1, LM: PERT{Min: 1e6, Mode: 2e6, Max: 4e6}, Seed: DefaultSeed})
	p := NewPresenter(CurrencyEUR, DefaultRateTable(), DefaultXAFPerUSD)
	got := p.Present(d)
	if got.P50.XAF != d.P50 {
		t.Fatalf("XAF not preserved: %.2f vs %.2f", got.P50.XAF, d.P50)
	}
	wantEUR := round2(d.P50 / 655.957)
	if got.P50.Value != wantEUR || got.P50.Currency != CurrencyEUR {
		t.Fatalf("EUR conversion wrong: got %.2f %s want %.2f EUR", got.P50.Value, got.P50.Currency, wantEUR)
	}
}
