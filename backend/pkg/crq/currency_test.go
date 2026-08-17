// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crq

import (
	"math"
	"testing"
)

func TestConvert_Currencies(t *testing.T) {
	rt := DefaultRateTable()
	const xaf = 655_957_000.0 // exactly 1,000,000 EUR at the fixed peg
	cases := []struct {
		cur  Currency
		want float64
	}{
		{CurrencyXAF, xaf},
		{CurrencyXOF, xaf}, // same CFA parity
		{CurrencyEUR, 1_000_000.0},
		{CurrencyUSD, round2(xaf / DefaultXAFPerUSD)},
		{CurrencyNGN, round2(xaf / 0.40)},
		{CurrencyZAR, round2(xaf / 33.0)},
	}
	for _, c := range cases {
		got := rt.Convert(xaf, c.cur)
		if math.Abs(got-c.want) > 0.01 {
			t.Fatalf("%s: got %.2f want %.2f", c.cur, got, c.want)
		}
	}
}

func TestConvert_XAFIdentity(t *testing.T) {
	rt := DefaultRateTable()
	if got := rt.Convert(42_000, CurrencyXAF); got != 42_000 {
		t.Fatalf("XAF identity broken: %.2f", got)
	}
	if got := rt.Convert(42_000, ""); got != 42_000 {
		t.Fatalf("empty currency should be XAF identity: %.2f", got)
	}
}

func TestConvert_UnknownCurrencyDefaultsToXAF(t *testing.T) {
	rt := DefaultRateTable()
	// A currency missing from the table falls back to rate 1.0 (XAF).
	if got := rt.Convert(1000, Currency("JPY")); got != 1000 {
		t.Fatalf("unknown currency should default to identity: %.2f", got)
	}
}

func TestIsSupportedAndNormalize(t *testing.T) {
	if !IsSupportedCurrency("eur") || !IsSupportedCurrency(" ZAR ") {
		t.Fatalf("case/space-insensitive support check failed")
	}
	if IsSupportedCurrency("JPY") {
		t.Fatalf("JPY should not be supported")
	}
	if NormalizeCurrency("ghs") != CurrencyGHS {
		t.Fatalf("normalize lower-case failed")
	}
	if NormalizeCurrency("") != CurrencyXAF || NormalizeCurrency("nope") != CurrencyXAF {
		t.Fatalf("normalize fallback to XAF failed")
	}
}

func TestRateFor(t *testing.T) {
	rt := DefaultRateTable()
	if rt.RateFor(CurrencyEUR) != 655.96 {
		t.Fatalf("EUR rate display: got %.2f want 655.96", rt.RateFor(CurrencyEUR))
	}
	if rt.RateFor(CurrencyXAF) != 1.0 {
		t.Fatalf("XAF self-rate should be 1")
	}
}

func TestAmount_AllThreeCurrencies(t *testing.T) {
	p := NewPresenter(CurrencyUSD, DefaultRateTable(), DefaultXAFPerUSD)
	a := p.Amount(1_200_000)
	if a.XAF != 1_200_000 {
		t.Fatalf("XAF preserved: %.2f", a.XAF)
	}
	if a.USD != 2000 { // 1.2M / 600
		t.Fatalf("USD: got %.2f want 2000", a.USD)
	}
	if a.Value != 2000 || a.Currency != CurrencyUSD {
		t.Fatalf("display currency: got %.2f %s", a.Value, a.Currency)
	}
}
