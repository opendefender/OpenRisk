// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// financial.go extends the CRQ engine (crq.go) from a bare ALE = SLE × ARO into
// the full quantitative model expected by spec §9 "Quantification financière":
// downtime cost, a composed SLE (downtime + fines + data loss + other direct
// cost), worst-case / average loss modelling (triangular / PERT), remediation
// cost and ROSI (Return on Security Investment). Every function here is pure and
// deterministic so the whole model is unit-testable and explainable to a CFO —
// no actuarial black box.
package crq

// Loss-band spread factors used when a risk only carries a most-likely SLE (no
// explicit min/max). They give a defensible worst/best envelope around the point
// estimate: best = SLE × 0.5, worst = SLE × 2.0. Documented and deliberately
// conservative — a real deployment tunes these per sector.
const (
	DefaultBestFactor  = 0.5
	DefaultWorstFactor = 2.0
)

// FinancialInputs are the raw, per-risk monetary drivers. Every field is a
// pointer so "not provided" is distinct from "zero"; the engine composes what it
// can and falls back to the reference model otherwise. Amounts are XAF (FCFA).
type FinancialInputs struct {
	// Single Loss Expectancy, when known explicitly (overrides the composed SLE).
	SLEXAF *float64
	// Annualized Rate of Occurrence (events/year, e.g. 0.5 = once every 2 years).
	ARO *float64

	// SLE components (spec: "coût des interruptions, amendes, perte de données").
	DowntimeHours         *float64 // business hours lost per incident
	HourlyDowntimeCostXAF *float64 // cost of one hour of downtime (XAF/h)
	DataLossCostXAF       *float64 // data recovery / breach-notification cost
	FinesXAF              *float64 // regulatory fines / penalties
	OtherDirectCostXAF    *float64 // any other direct per-incident cost

	// Optional explicit loss band (worst / best single-event loss). When absent,
	// the band is derived from SLE via the spread factors above.
	SLEBestXAF  *float64
	SLEWorstXAF *float64

	// Treatment / investment.
	RemediationCostXAF      *float64 // budget to fix the vuln / deploy the control
	MitigationEffectiveness *float64 // [0,1] share of ALE the control removes
}

// FinancialAssessment is the full monetary view of a single risk — the object a
// CISO/CFO dashboard renders. Every monetary field carries both currencies.
type FinancialAssessment struct {
	// --- Single-event loss magnitude ---
	SLE          Money `json:"sle"`           // effective single loss expectancy
	SLEAverage   Money `json:"sle_average"`   // PERT-expected single loss
	SLEWorst     Money `json:"sle_worst"`     // worst-case single loss
	DowntimeCost Money `json:"downtime_cost"` // downtime hours × hourly cost
	SLEBasis     Basis `json:"sle_basis"`     // explicit | composed | reference

	// --- Frequency ---
	ARO float64 `json:"aro"`

	// --- Annualized loss ---
	ALE        Money `json:"ale"`         // SLE × ARO (or reference)
	ALEAverage Money `json:"ale_average"` // average single loss × ARO
	ALEWorst   Money `json:"ale_worst"`   // worst single loss × ARO
	ALEBasis   Basis `json:"ale_basis"`   // explicit | reference

	// --- Treatment / investment (ROSI) ---
	RemediationCost Money   `json:"remediation_cost"`
	Effectiveness   float64 `json:"mitigation_effectiveness"` // [0,1]
	ALEAfter        Money   `json:"ale_after"`                // residual ALE post-control
	RiskReduction   Money   `json:"risk_reduction"`           // ALE − ALEAfter (benefit)
	ROSI            float64 `json:"rosi"`                     // ratio, e.g. 2.5 = +250%
	ROSIComputable  bool    `json:"rosi_computable"`          // false when remediation cost ≤ 0
}

const (
	// BasisComposed marks an SLE built from its components rather than supplied
	// as a single figure or taken from the reference band.
	BasisComposed Basis = "composed"
)

// DowntimeCostXAF returns the interruption cost for one incident: hours × hourly
// rate. Missing either driver yields 0.
func DowntimeCostXAF(hours, hourlyCostXAF *float64) float64 {
	if hours == nil || hourlyCostXAF == nil || *hours <= 0 || *hourlyCostXAF <= 0 {
		return 0
	}
	return round2(*hours * *hourlyCostXAF)
}

// ROSI computes the Return on Security Investment:
//
//	ROSI = (ALE_before − ALE_after − remediationCost) / remediationCost
//
// which is equivalently (riskReduction − remediationCost) / remediationCost.
// A ROSI of 1.0 means the control returns 100% on its cost over a year. The bool
// is false when remediationCost ≤ 0 (ratio undefined — no investment to divide by).
func ROSI(aleBefore, aleAfter, remediationCost float64) (float64, bool) {
	if remediationCost <= 0 {
		return 0, false
	}
	return round2((aleBefore - aleAfter - remediationCost) / remediationCost), true
}

// Assess runs the full financial model for one risk. It never errors: absent
// inputs degrade gracefully to the reference model so every risk still carries an
// order-of-magnitude figure, exactly like Quantify.
func (q *Quantifier) Assess(in FinancialInputs, criticality string) FinancialAssessment {
	downtime := DowntimeCostXAF(in.DowntimeHours, in.HourlyDowntimeCostXAF)

	// 1. Effective single-loss expectancy (XAF) + how we got it.
	sleXAF, sleBasis := q.effectiveSLE(in, downtime, criticality)

	// 2. Loss band around the point estimate → average (PERT) and worst case.
	best, worst := q.lossBand(in, sleXAF)
	// PERT expected value: (best + 4×mostLikely + worst) / 6.
	avg := round2((best + 4*sleXAF + worst) / 6)

	// 3. Frequency.
	aro := 0.0
	if in.ARO != nil && *in.ARO > 0 {
		aro = *in.ARO
	}

	// 4. Annualized loss. Reuse the canonical ALE (explicit SLE×ARO or reference)
	//    so per-risk figures stay consistent with the rest of the app.
	aleXAF, aleBasis := q.ALEXAF(in.SLEXAF, in.ARO, criticality)
	// When SLE was composed (not explicit) but we do have an ARO, annualize the
	// composed SLE rather than falling back to the reference band.
	if aleBasis == BasisReference && aro > 0 && sleXAF > 0 {
		aleXAF = round2(sleXAF * aro)
		aleBasis = BasisExplicit
	}
	aleWorst := round2(worst * aro)
	aleAvg := round2(avg * aro)

	// 5. Treatment / investment → residual ALE, benefit, ROSI.
	eff := clamp01(in.MitigationEffectiveness)
	aleAfter := round2(aleXAF * (1 - eff))
	reduction := round2(aleXAF - aleAfter)
	remediation := 0.0
	if in.RemediationCostXAF != nil && *in.RemediationCostXAF > 0 {
		remediation = *in.RemediationCostXAF
	}
	rosi, rosiOK := ROSI(aleXAF, aleAfter, remediation)

	return FinancialAssessment{
		SLE:          q.Money(sleXAF),
		SLEAverage:   q.Money(avg),
		SLEWorst:     q.Money(worst),
		DowntimeCost: q.Money(downtime),
		SLEBasis:     sleBasis,

		ARO: aro,

		ALE:        q.Money(aleXAF),
		ALEAverage: q.Money(aleAvg),
		ALEWorst:   q.Money(aleWorst),
		ALEBasis:   aleBasis,

		RemediationCost: q.Money(remediation),
		Effectiveness:   eff,
		ALEAfter:        q.Money(aleAfter),
		RiskReduction:   q.Money(reduction),
		ROSI:            rosi,
		ROSIComputable:  rosiOK,
	}
}

// effectiveSLE resolves the single-loss expectancy: an explicit figure wins;
// otherwise the components (downtime + fines + data loss + other) are summed;
// if nothing was supplied it falls back to the reference band for the criticality.
func (q *Quantifier) effectiveSLE(in FinancialInputs, downtime float64, criticality string) (float64, Basis) {
	if in.SLEXAF != nil && *in.SLEXAF > 0 {
		return round2(*in.SLEXAF), BasisExplicit
	}
	composed := downtime + val(in.DataLossCostXAF) + val(in.FinesXAF) + val(in.OtherDirectCostXAF)
	if composed > 0 {
		return round2(composed), BasisComposed
	}
	return round2(q.Reference.For(criticality)), BasisReference
}

// lossBand returns (best, worst) single-event loss. Explicit bounds win; missing
// ones are derived from the point estimate via the spread factors.
func (q *Quantifier) lossBand(in FinancialInputs, sleXAF float64) (best, worst float64) {
	best = round2(sleXAF * DefaultBestFactor)
	worst = round2(sleXAF * DefaultWorstFactor)
	if in.SLEBestXAF != nil && *in.SLEBestXAF > 0 {
		best = round2(*in.SLEBestXAF)
	}
	if in.SLEWorstXAF != nil && *in.SLEWorstXAF > 0 {
		worst = round2(*in.SLEWorstXAF)
	}
	// Keep the band coherent: worst must not sit below the point estimate, best
	// not above it.
	if worst < sleXAF {
		worst = sleXAF
	}
	if best > sleXAF {
		best = sleXAF
	}
	return best, worst
}

// val safely dereferences an optional positive amount (nil / negative → 0).
func val(p *float64) float64 {
	if p == nil || *p <= 0 {
		return 0
	}
	return *p
}

// clamp01 coerces an optional effectiveness into [0,1] (nil → 0).
func clamp01(p *float64) float64 {
	if p == nil {
		return 0
	}
	v := *p
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
