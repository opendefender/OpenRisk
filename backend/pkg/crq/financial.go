// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// financial.go extends the CRQ engine (crq.go) from a bare ALE = SLE × ARO into
// the full quantitative model expected by spec §9 "Quantification financière":
// downtime cost, a composed SLE (downtime + fines + data loss + other direct
// cost), worst-case / average loss modelling (triangular / PERT), remediation
// cost and ROSI (Return on Security Investment). Every function here is pure and
// deterministic so the whole model is unit-testable and explainable to a CFO —
// no actuarial black box.
package crq

import "time"

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

// MethodologyInput is one intrant surfaced in the explainability panel: what
// value went into the model, in what unit, and where it came from.
type MethodologyInput struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Source string  `json:"source"` // "risk-input" | "reference-model" | "derived"
}

// Methodology is the audit trail for a quantified figure (spec §4): the model,
// its exact intrants, assumptions, run parameters and the reference rate used.
// ComputedAt is stamped by the caller (handler/use case), not the pure engine.
type Methodology struct {
	FormulaVersion string             `json:"formula_version"`
	Model          string             `json:"model"`
	Iterations     int                `json:"iterations"`
	Seed           int64              `json:"seed"`
	ComputedAt     time.Time          `json:"computed_at"`
	DocURL         string             `json:"doc_url"`
	Currency       Currency           `json:"currency"`
	FXAsOf         time.Time          `json:"fx_as_of"`
	FXRateXAF      float64            `json:"fx_rate_xaf"` // XAF per 1 unit of Currency
	Inputs         []MethodologyInput `json:"inputs"`
	Assumptions    []string           `json:"assumptions"`
}

// FinancialAssessment is the full monetary view of a single risk — the object a
// CISO/CFO dashboard renders. Every monetary field carries both currencies.
type FinancialAssessment struct {
	// --- FAIR-lite loss distribution (spec §2: P10/P50/P90, never one number) ---
	Distribution *DistributionAmounts `json:"distribution,omitempty"`
	// --- Explainability (spec §4) ---
	Methodology *Methodology `json:"methodology,omitempty"`

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

// Assess runs the full financial model for one risk in XAF only (no tenant
// currency). Kept for callers that don't present a currency; delegates to AssessP
// with a plain XAF presenter.
func (q *Quantifier) Assess(in FinancialInputs, criticality string) FinancialAssessment {
	return q.AssessP(in, criticality, NewPresenter(CurrencyXAF, DefaultRateTable(), q.XAFPerUSD))
}

// SimulationInputFor derives the FAIR-lite Monte Carlo parameters (LEF + PERT
// loss-magnitude band) from a risk's stored drivers. LEF is the ARO (falling back
// to 1 loss/year when unknown so the reference band reads as an annual figure);
// the PERT band is the composed/explicit SLE as the mode with the loss-band
// best/worst as the bounds.
func (q *Quantifier) SimulationInputFor(in FinancialInputs, criticality string) SimulationInput {
	downtime := DowntimeCostXAF(in.DowntimeHours, in.HourlyDowntimeCostXAF)
	sleXAF, _ := q.effectiveSLE(in, downtime, criticality)
	best, worst := q.lossBand(in, sleXAF)
	lef := 1.0
	if in.ARO != nil && *in.ARO > 0 {
		lef = *in.ARO
	}
	return SimulationInput{
		LEF:        lef,
		LM:         PERT{Min: best, Mode: sleXAF, Max: worst},
		Iterations: DefaultIterations,
		Seed:       DefaultSeed,
	}
}

// AssessP runs the full financial model for one risk and presents every figure in
// the tenant's display currency via p. It never errors: absent inputs degrade
// gracefully to the reference model so every risk still carries an
// order-of-magnitude figure, exactly like Quantify.
func (q *Quantifier) AssessP(in FinancialInputs, criticality string, p Presenter) FinancialAssessment {
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
	// Annualized worst/average loss preserve the single-event loss-band ratio
	// applied to ALE. For explicit/composed risks this equals worst×ARO /
	// avg×ARO; for reference-only risks (no ARO) it keeps a sensible envelope
	// around the reference ALE instead of collapsing to 0.
	aleWorst := aleXAF
	aleAvg := aleXAF
	if sleXAF > 0 {
		aleWorst = round2(aleXAF * worst / sleXAF)
		aleAvg = round2(aleXAF * avg / sleXAF)
	}

	// 5. Treatment / investment → residual ALE, benefit, ROSI.
	eff := clamp01(in.MitigationEffectiveness)
	aleAfter := round2(aleXAF * (1 - eff))
	reduction := round2(aleXAF - aleAfter)
	remediation := 0.0
	if in.RemediationCostXAF != nil && *in.RemediationCostXAF > 0 {
		remediation = *in.RemediationCostXAF
	}
	rosi, rosiOK := ROSI(aleXAF, aleAfter, remediation)

	// 6. FAIR-lite loss distribution (P10/P50/P90) + explainability metadata.
	sim := q.SimulationInputFor(in, criticality)
	dist := p.Present(Simulate(sim))
	method := q.methodology(in, sim, p, sleBasis)

	return FinancialAssessment{
		Distribution: &dist,
		Methodology:  method,
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

// methodology assembles the explainability payload for one risk: the model, the
// exact intrants that fed it (with their provenance), the assumptions, the run
// parameters and the reference FX rate. ComputedAt is left zero here (pure
// engine) and stamped by the caller.
func (q *Quantifier) methodology(in FinancialInputs, sim SimulationInput, p Presenter, sleBasis Basis) *Methodology {
	inputs := []MethodologyInput{
		{Key: "lef", Label: "Loss Event Frequency", Value: round2(sim.LEF), Unit: "events/year", Source: aroSource(in)},
		{Key: "lm_min", Label: "Loss Magnitude — min", Value: round2(sim.LM.Min), Unit: "XAF", Source: "derived"},
		{Key: "lm_mode", Label: "Loss Magnitude — most likely", Value: round2(sim.LM.Mode), Unit: "XAF", Source: sleSource(sleBasis)},
		{Key: "lm_max", Label: "Loss Magnitude — max", Value: round2(sim.LM.Max), Unit: "XAF", Source: "derived"},
	}
	// Surface the specific loss drivers that were actually provided.
	if v := val(in.DowntimeHours); v > 0 {
		inputs = append(inputs, MethodologyInput{Key: "downtime_hours", Label: "Downtime", Value: v, Unit: "hours", Source: "risk-input"})
	}
	if v := val(in.HourlyDowntimeCostXAF); v > 0 {
		inputs = append(inputs, MethodologyInput{Key: "hourly_downtime_cost", Label: "Downtime cost", Value: v, Unit: "XAF/hour", Source: "risk-input"})
	}
	if v := val(in.FinesXAF); v > 0 {
		inputs = append(inputs, MethodologyInput{Key: "fines", Label: "Regulatory fines", Value: v, Unit: "XAF", Source: "risk-input"})
	}
	if v := val(in.DataLossCostXAF); v > 0 {
		inputs = append(inputs, MethodologyInput{Key: "data_loss", Label: "Data-loss cost", Value: v, Unit: "XAF", Source: "risk-input"})
	}

	assumptions := []string{
		"ALE = LEF × LM ; LM follows a 3-point PERT distribution (min / most-likely / max).",
		"Loss magnitude sampled by Monte Carlo; percentiles are P10 / P50 (median) / P90.",
	}
	if sleBasis == BasisReference {
		assumptions = append(assumptions, "No explicit loss inputs on this risk — the reference band for its criticality was used.")
	}
	if in.SLEBestXAF == nil || in.SLEWorstXAF == nil {
		assumptions = append(assumptions, "Loss band derived from the point estimate (best = ×0.5, worst = ×2.0).")
	}

	return &Methodology{
		FormulaVersion: FormulaVersion,
		Model:          "FAIR-lite: ALE = LEF × LM (PERT), Monte Carlo",
		Iterations:     sim.Iterations,
		Seed:           sim.Seed,
		DocURL:         DocURL,
		Currency:       p.Currency,
		FXAsOf:         p.Rates.AsOf,
		FXRateXAF:      p.Rates.RateFor(p.Currency),
		Inputs:         inputs,
		Assumptions:    assumptions,
	}
}

// aroSource labels where the LEF came from.
func aroSource(in FinancialInputs) string {
	if in.ARO != nil && *in.ARO > 0 {
		return "risk-input"
	}
	return "reference-model"
}

// sleSource labels where the loss-magnitude mode came from.
func sleSource(b Basis) string {
	switch b {
	case BasisExplicit:
		return "risk-input"
	case BasisComposed:
		return "derived"
	default:
		return "reference-model"
	}
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
