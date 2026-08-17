// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// montecarlo.go is the FAIR-lite quantification core (spec §9). It turns a risk
// into a LOSS DISTRIBUTION, not a single number — a single figure is a false
// certainty. The model, stated plainly so it is defensible to a CISO:
//
//	ALE = LEF × LM
//
//	  LEF — Loss Event Frequency, expected loss events per year (from ARO).
//	  LM  — Loss Magnitude, a 3-point PERT distribution (min / most-likely / max).
//
// A Monte Carlo run draws N loss magnitudes from the PERT distribution, scales
// each by LEF, and reports the P10 / P50 / P90 percentiles plus the mean. The
// run is fully deterministic for a given seed, so the same inputs always yield
// the same band — a hard requirement for an auditable figure.
package crq

import (
	"math"
	"math/rand"
	"sort"
)

// FormulaVersion identifies the quantification model. Bump it on any change to
// the math so stored figures can be traced to the version that produced them.
const FormulaVersion = "fair-lite-1.0.0"

// DefaultIterations / DefaultSeed are the reproducible run parameters. 10k
// iterations converge the percentiles to within a fraction of a percent while
// staying well under a millisecond per risk.
const (
	DefaultIterations = 10_000
	DefaultSeed       = int64(20260801)
)

// DocURL points to the methodology reference surfaced in the explainability panel.
const DocURL = "/docs/financial-quantification.md"

// PERT is a 3-point loss-magnitude distribution: a minimum, a most-likely (mode)
// and a maximum single-event loss, in XAF.
type PERT struct {
	Min  float64 `json:"min"`
	Mode float64 `json:"mode"`
	Max  float64 `json:"max"`
}

// ExpectedValue is the PERT mean: (min + 4·mode + max) / 6. The Monte Carlo mean
// converges to LEF × this value — the property the convergence test asserts.
func (p PERT) ExpectedValue() float64 {
	return (p.Min + 4*p.Mode + p.Max) / 6
}

// normalized returns a sane, ordered (min ≤ mode ≤ max) copy, clamping negatives
// to zero so a malformed input can never produce a negative loss.
func (p PERT) normalized() PERT {
	n := PERT{Min: math.Max(0, p.Min), Mode: math.Max(0, p.Mode), Max: math.Max(0, p.Max)}
	if n.Max < n.Min {
		n.Min, n.Max = n.Max, n.Min
	}
	if n.Mode < n.Min {
		n.Mode = n.Min
	}
	if n.Mode > n.Max {
		n.Mode = n.Max
	}
	return n
}

// SimulationInput is one risk's FAIR-lite parameters.
type SimulationInput struct {
	LEF        float64 // loss event frequency (events/year)
	LM         PERT    // loss magnitude distribution (XAF)
	Iterations int
	Seed       int64
}

// LossDistribution is the Monte Carlo output for one risk (all XAF). The
// percentiles are the headline; the run metadata makes the figure explainable.
type LossDistribution struct {
	P10  float64 `json:"p10"`
	P50  float64 `json:"p50"` // median — the figure to lead with
	P90  float64 `json:"p90"`
	Mean float64 `json:"mean"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`

	Iterations     int     `json:"iterations"`
	Seed           int64   `json:"seed"`
	FormulaVersion string  `json:"formula_version"`
	LEF            float64 `json:"lef"`
}

// Simulate runs the FAIR-lite Monte Carlo. It is pure and deterministic: the same
// SimulationInput (including seed) always yields the same LossDistribution.
func Simulate(in SimulationInput) LossDistribution {
	iters := in.Iterations
	if iters <= 0 {
		iters = DefaultIterations
	}
	lm := in.LM.normalized()
	lef := in.LEF
	if lef < 0 {
		lef = 0
	}

	dist := LossDistribution{
		Iterations:     iters,
		Seed:           in.Seed,
		FormulaVersion: FormulaVersion,
		LEF:            round2(lef),
	}

	// Degenerate magnitude (no spread) → a deterministic point at LEF × mode.
	if lm.Max <= lm.Min {
		v := round2(lef * lm.Mode)
		dist.P10, dist.P50, dist.P90, dist.Mean, dist.Min, dist.Max = v, v, v, v, v, v
		return dist
	}

	rng := rand.New(rand.NewSource(in.Seed))
	alpha, beta := pertBetaParams(lm)

	samples := make([]float64, iters)
	var sum float64
	for i := 0; i < iters; i++ {
		x := sampleBeta(rng, alpha, beta)          // Beta(α,β) ∈ [0,1]
		loss := lm.Min + x*(lm.Max-lm.Min)         // scale to [min,max]
		annual := lef * loss                       // ALE contribution = LEF × LM
		samples[i] = annual
		sum += annual
	}
	sort.Float64s(samples)

	dist.P10 = round2(percentile(samples, 10))
	dist.P50 = round2(percentile(samples, 50))
	dist.P90 = round2(percentile(samples, 90))
	dist.Mean = round2(sum / float64(iters))
	dist.Min = round2(samples[0])
	dist.Max = round2(samples[len(samples)-1])
	return dist
}

// pertBetaParams derives the standard Beta-PERT shape parameters (λ = 4).
func pertBetaParams(p PERT) (alpha, beta float64) {
	span := p.Max - p.Min
	alpha = 1 + 4*(p.Mode-p.Min)/span
	beta = 1 + 4*(p.Max-p.Mode)/span
	return alpha, beta
}

// sampleBeta draws from Beta(α,β) via two Gamma draws: B = G(α)/(G(α)+G(β)).
func sampleBeta(rng *rand.Rand, alpha, beta float64) float64 {
	ga := sampleGamma(rng, alpha)
	gb := sampleGamma(rng, beta)
	if ga+gb == 0 {
		return 0.5
	}
	return ga / (ga + gb)
}

// sampleGamma draws from Gamma(shape, 1) using the Marsaglia–Tsang method. Valid
// for shape > 0; shapes < 1 are boosted then corrected.
func sampleGamma(rng *rand.Rand, shape float64) float64 {
	if shape < 1 {
		// Boost: Gamma(shape) = Gamma(shape+1) · U^(1/shape).
		u := rng.Float64()
		if u == 0 {
			u = math.SmallestNonzeroFloat64
		}
		return sampleGamma(rng, shape+1) * math.Pow(u, 1/shape)
	}
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9*d)
	for {
		var x, v float64
		for {
			x = rng.NormFloat64()
			v = 1 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.0331*x*x*x*x {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

// percentile returns the p-th percentile (0–100) of a sorted slice using linear
// interpolation between order statistics.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(n-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}

// Amount is a currency-aware monetary value: the canonical XAF store, its USD
// convenience conversion, and the tenant's chosen display currency + value.
type Amount struct {
	XAF      float64  `json:"xaf"`
	USD      float64  `json:"usd"`
	Value    float64  `json:"value"`    // amount in Currency
	Currency Currency `json:"currency"` // tenant display currency
}

// Presenter converts XAF figures into currency-aware Amounts at a dated rate.
// Built per request from the tenant's currency + the live rate table.
type Presenter struct {
	Rates     *RateTable
	Currency  Currency
	XAFPerUSD float64
}

// NewPresenter builds a Presenter, defaulting the rate table and currency.
func NewPresenter(currency Currency, rates *RateTable, xafPerUSD float64) Presenter {
	if rates == nil {
		rates = DefaultRateTable()
	}
	if !IsSupportedCurrency(string(currency)) {
		currency = CurrencyXAF
	}
	if xafPerUSD <= 0 {
		xafPerUSD = DefaultXAFPerUSD
	}
	return Presenter{Rates: rates, Currency: currency, XAFPerUSD: xafPerUSD}
}

// Amount wraps an XAF figure with its USD and tenant-currency conversions.
func (p Presenter) Amount(xaf float64) Amount {
	return Amount{
		XAF:      round2(xaf),
		USD:      round2(xaf / p.XAFPerUSD),
		Value:    p.Rates.Convert(xaf, p.Currency),
		Currency: p.Currency,
	}
}

// DistributionAmounts is the loss band expressed as currency-aware Amounts.
type DistributionAmounts struct {
	P10  Amount `json:"p10"`
	P50  Amount `json:"p50"`
	P90  Amount `json:"p90"`
	Mean Amount `json:"mean"`

	Iterations     int     `json:"iterations"`
	Seed           int64   `json:"seed"`
	FormulaVersion string  `json:"formula_version"`
	LEF            float64 `json:"lef"`
}

// Present converts a raw XAF LossDistribution into currency-aware Amounts.
func (p Presenter) Present(d LossDistribution) DistributionAmounts {
	return DistributionAmounts{
		P10:            p.Amount(d.P10),
		P50:            p.Amount(d.P50),
		P90:            p.Amount(d.P90),
		Mean:           p.Amount(d.Mean),
		Iterations:     d.Iterations,
		Seed:           d.Seed,
		FormulaVersion: d.FormulaVersion,
		LEF:            d.LEF,
	}
}
