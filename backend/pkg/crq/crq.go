// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package crq is OpenRisk's Cyber Risk Quantification engine. It turns a risk's
// qualitative posture (or explicit loss inputs) into a monetary figure in both
// XAF (FCFA — the target market's currency) and USD.
//
// Model (classic quantitative risk analysis, à la FAIR/NIST):
//
//	ALE = SLE × ARO
//
// where SLE is the Single Loss Expectancy (the cost of one occurrence, in XAF)
// and ARO is the Annualized Rate of Occurrence (expected events per year, e.g.
// 0.5 = once every two years). When a risk has no explicit SLE/ARO, a
// transparent reference ALE per criticality band is used so every risk still
// carries an order-of-magnitude figure. This is deliberately simple and
// explainable — not an actuarial certainty.
package crq

import (
	"math"
	"strings"
)

// DefaultXAFPerUSD is the fallback XAF→USD rate (≈ 1 USD). Configurable via the
// XAF_USD_RATE env var in the composition root.
const DefaultXAFPerUSD = 600.0

// Basis records how an ALE was derived, so the UI can be honest about it.
type Basis string

const (
	BasisExplicit  Basis = "explicit"  // from the risk's own SLE × ARO
	BasisReference Basis = "reference" // from the per-criticality reference model
)

// Reference holds the fallback annual-loss value (XAF) per criticality band. The
// defaults match board.ExposureModel so per-risk and board figures agree.
type Reference struct {
	Critical, High, Medium, Low float64
}

// DefaultReference returns the built-in reference ALE values (XAF).
func DefaultReference() Reference {
	return Reference{Critical: 50_000_000, High: 15_000_000, Medium: 3_000_000, Low: 500_000}
}

// For returns the reference ALE (XAF) for a criticality string. It accepts both
// vocabularies in use (lower/upper case).
func (r Reference) For(criticality string) float64 {
	switch strings.ToLower(strings.TrimSpace(criticality)) {
	case "critical":
		return r.Critical
	case "high":
		return r.High
	case "medium":
		return r.Medium
	case "low":
		return r.Low
	default:
		return r.Medium
	}
}

// Money is an amount expressed in both currencies.
type Money struct {
	XAF float64 `json:"xaf"`
	USD float64 `json:"usd"`
}

// Quantification is the full monetary view of a single risk.
type Quantification struct {
	SLEXAF *float64 `json:"sle_xaf,omitempty"` // single loss expectancy (XAF), if provided
	ARO    *float64 `json:"aro,omitempty"`     // annualized rate of occurrence, if provided
	ALE    Money    `json:"ale"`               // annual loss expectancy, XAF + USD
	Basis  Basis    `json:"basis"`             // explicit | reference
}

// Quantifier computes ALE and currency conversion with a fixed rate + reference.
type Quantifier struct {
	XAFPerUSD float64
	Reference Reference
}

// NewQuantifier builds a Quantifier, falling back to sane defaults.
func NewQuantifier(xafPerUSD float64, ref Reference) *Quantifier {
	if xafPerUSD <= 0 {
		xafPerUSD = DefaultXAFPerUSD
	}
	if ref == (Reference{}) {
		ref = DefaultReference()
	}
	return &Quantifier{XAFPerUSD: xafPerUSD, Reference: ref}
}

// ALEXAF computes the annual loss expectancy in XAF: SLE × ARO when both are
// present, otherwise the reference value for the criticality band.
func (q *Quantifier) ALEXAF(sleXAF, aro *float64, criticality string) (float64, Basis) {
	if sleXAF != nil && aro != nil && *sleXAF > 0 && *aro > 0 {
		return round2(*sleXAF * *aro), BasisExplicit
	}
	return round2(q.Reference.For(criticality)), BasisReference
}

// ToUSD converts an XAF amount to USD at the configured rate.
func (q *Quantifier) ToUSD(xaf float64) float64 {
	return round2(xaf / q.XAFPerUSD)
}

// Money wraps an XAF amount with its USD conversion.
func (q *Quantifier) Money(xaf float64) Money {
	return Money{XAF: round2(xaf), USD: q.ToUSD(xaf)}
}

// Quantify returns the full monetary view for a risk.
func (q *Quantifier) Quantify(sleXAF, aro *float64, criticality string) Quantification {
	aleXAF, basis := q.ALEXAF(sleXAF, aro, criticality)
	return Quantification{
		SLEXAF: sleXAF,
		ARO:    aro,
		ALE:    q.Money(aleXAF),
		Basis:  basis,
	}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
