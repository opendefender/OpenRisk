// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crq

import "testing"

func f(v float64) *float64 { return &v }

func TestQuantify_Explicit(t *testing.T) {
	q := NewQuantifier(600, DefaultReference())
	got := q.Quantify(f(10_000_000), f(0.5), "high") // SLE 10M × ARO 0.5 = 5M XAF
	if got.Basis != BasisExplicit {
		t.Fatalf("basis = %q, want explicit", got.Basis)
	}
	if got.ALE.XAF != 5_000_000 {
		t.Fatalf("ALE XAF = %v, want 5000000", got.ALE.XAF)
	}
	if got.ALE.USD != 8333.33 { // 5_000_000 / 600, rounded to 2dp
		t.Fatalf("ALE USD = %v, want 8333.33", got.ALE.USD)
	}
}

func TestQuantify_ReferenceFallback(t *testing.T) {
	q := NewQuantifier(600, DefaultReference())
	// No explicit inputs → reference band for "critical" = 50M XAF.
	got := q.Quantify(nil, nil, "critical")
	if got.Basis != BasisReference {
		t.Fatalf("basis = %q, want reference", got.Basis)
	}
	if got.ALE.XAF != 50_000_000 {
		t.Fatalf("ALE XAF = %v, want 50000000", got.ALE.XAF)
	}
	// A zero/partial input also falls back to reference.
	if q.Quantify(f(0), f(2), "low").Basis != BasisReference {
		t.Fatalf("zero SLE should fall back to reference")
	}
	if q.Quantify(f(1_000_000), nil, "low").Basis != BasisReference {
		t.Fatalf("missing ARO should fall back to reference")
	}
}

func TestReferenceBands(t *testing.T) {
	r := DefaultReference()
	cases := map[string]float64{"critical": 50_000_000, "HIGH": 15_000_000, "Medium": 3_000_000, "low": 500_000, "unknown": 3_000_000}
	for crit, want := range cases {
		if got := r.For(crit); got != want {
			t.Errorf("For(%q) = %v, want %v", crit, got, want)
		}
	}
}

func TestNewQuantifier_Defaults(t *testing.T) {
	q := NewQuantifier(0, Reference{}) // zero rate + empty ref → defaults
	if q.XAFPerUSD != DefaultXAFPerUSD {
		t.Fatalf("rate = %v, want %v", q.XAFPerUSD, DefaultXAFPerUSD)
	}
	if q.Reference != DefaultReference() {
		t.Fatalf("reference not defaulted")
	}
}
