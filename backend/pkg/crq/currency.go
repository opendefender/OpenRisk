// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// currency.go gives the CRQ engine a tenant-level display currency. Amounts are
// always STORED in XAF (FCFA — the target market's base) and converted for
// display at a dated reference rate. The rate table carries an AsOf date so the
// UI can show "1 EUR = 655.96 XAF (as of 2026-08-01)" next to every figure — a
// converted amount without its reference date is not defensible to a CFO.
package crq

import (
	"strings"
	"time"
)

// Currency is an ISO-4217 code the platform can display. The base/store currency
// is always XAF; the others are display targets converted at the reference rate.
type Currency string

const (
	CurrencyXAF Currency = "XAF" // Central African CFA franc (base)
	CurrencyXOF Currency = "XOF" // West African CFA franc
	CurrencyEUR Currency = "EUR"
	CurrencyUSD Currency = "USD"
	CurrencyNGN Currency = "NGN" // Nigerian naira
	CurrencyMAD Currency = "MAD" // Moroccan dirham
	CurrencyGHS Currency = "GHS" // Ghanaian cedi
	CurrencyZAR Currency = "ZAR" // South African rand
)

// SupportedCurrencies is the closed set the tenant may pick (spec §3).
var SupportedCurrencies = []Currency{
	CurrencyXAF, CurrencyXOF, CurrencyEUR, CurrencyUSD,
	CurrencyNGN, CurrencyMAD, CurrencyGHS, CurrencyZAR,
}

// IsSupportedCurrency reports whether code is one the platform can display.
// Comparison is case-insensitive and trims spaces.
func IsSupportedCurrency(code string) bool {
	c := Currency(strings.ToUpper(strings.TrimSpace(code)))
	for _, s := range SupportedCurrencies {
		if s == c {
			return true
		}
	}
	return false
}

// NormalizeCurrency returns a supported currency for an arbitrary string,
// falling back to XAF (the base) when the code is empty or unknown.
func NormalizeCurrency(code string) Currency {
	c := Currency(strings.ToUpper(strings.TrimSpace(code)))
	if IsSupportedCurrency(string(c)) {
		return c
	}
	return CurrencyXAF
}

// RateTable maps each display currency to the number of XAF that equal ONE unit
// of that currency (so "XAFPer[EUR] = 655.957" means 1 EUR = 655.957 XAF). This
// direction keeps XAF as the base and matches DefaultXAFPerUSD. AsOf is the date
// the rates were captured — surfaced next to every converted amount.
type RateTable struct {
	Base   Currency             `json:"base"`   // always XAF
	AsOf   time.Time            `json:"as_of"`  // reference date of the rates
	Source string               `json:"source"` // provider label ("static", "ecb", …)
	XAFPer map[Currency]float64 `json:"xaf_per"` // XAF per 1 unit of currency
}

// referenceRatesAsOf is the capture date of the built-in fallback table. XAF and
// XOF are both pegged to the euro at the fixed parity 1 EUR = 655.957 CFA; the
// others are rounded market approximations. A live FX job (fx worker) overwrites
// these with dated provider rates; until then the app is honest about the date.
var referenceRatesAsOf = time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

// DefaultRateTable returns the built-in reference rates (XAF per 1 unit). It is a
// deterministic fallback used when no live FX rate is available.
func DefaultRateTable() *RateTable {
	return &RateTable{
		Base:   CurrencyXAF,
		AsOf:   referenceRatesAsOf,
		Source: "static-reference",
		XAFPer: map[Currency]float64{
			CurrencyXAF: 1.0,
			CurrencyXOF: 1.0,      // XAF and XOF share the fixed CFA parity
			CurrencyEUR: 655.957,  // fixed BCEAO/BEAC euro peg
			CurrencyUSD: DefaultXAFPerUSD,
			CurrencyNGN: 0.40,     // ~1 NGN
			CurrencyMAD: 60.0,     // ~1 MAD
			CurrencyGHS: 40.0,     // ~1 GHS
			CurrencyZAR: 33.0,     // ~1 ZAR
		},
	}
}

// xafPer returns the XAF-per-unit rate for a currency, defaulting to 1.0 (XAF)
// when the currency is missing from the table.
func (t *RateTable) xafPer(c Currency) float64 {
	if t == nil {
		return 1.0
	}
	if v, ok := t.XAFPer[c]; ok && v > 0 {
		return v
	}
	return 1.0
}

// Convert turns an amount expressed in XAF into the target currency at the
// table's reference rate. XAF→XAF is the identity.
func (t *RateTable) Convert(xaf float64, target Currency) float64 {
	if target == CurrencyXAF || target == "" {
		return round2(xaf)
	}
	return round2(xaf / t.xafPer(target))
}

// RateFor returns the "1 <currency> = N XAF" figure for display next to amounts.
func (t *RateTable) RateFor(c Currency) float64 {
	return round2(t.xafPer(c))
}
