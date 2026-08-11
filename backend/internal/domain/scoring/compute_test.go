// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package scoring

import (
	"math"
	"testing"
	"time"
)

var at = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// Bounds — the property that must hold for every input, however absurd.
// ---------------------------------------------------------------------------

// Swept across the full input space plus deliberately out-of-range and
// non-finite values. Nothing may leave [0,100]; nothing may be NaN.
func TestBounds_RiskScoreAlwaysInRange(t *testing.T) {
	probs := []float64{-1, 0, 0.25, 0.5, 0.999, 1, 1.5, 42, math.NaN(), math.Inf(1), math.Inf(-1)}
	impacts := []float64{-5, 0, 1, 5, 9.99, 10, 11, 1000, math.NaN(), math.Inf(1)}
	crits := []float64{-1, 0, 0.1, 1.5, 3, 3.1, 99, math.NaN()}

	for _, p := range probs {
		for _, i := range impacts {
			for _, c := range crits {
				for _, hasAsset := range []bool{true, false} {
					for _, eff := range []float64{-1, 0, 0.5, 1, 2, math.NaN()} {
						r := ComputeRisk(RiskInput{
							Probability:             p,
							Impact:                  i,
							AssetCriticality:        c,
							HasAsset:                hasAsset,
							MitigationEffectiveness: eff,
						}, at)
						assertResultSane(t, r)
					}
				}
			}
		}
	}
}

func TestBounds_TenantAndAssetAlwaysInRange(t *testing.T) {
	ints := []int{-10, 0, 1, 7, 100, 10_000}
	for _, a := range ints {
		for _, b := range ints {
			for _, c := range ints {
				assertResultSane(t, ComputeTenant(TenantInput{
					CriticalRisks: a, HighRisks: b, TotalRisks: c, HasRiskData: true,
					ImplementedControls: b, ApplicableControls: a, HasComplianceData: true,
					KEVVulnerabilities: c, CriticalVulnerabilities: a, HasVulnData: true,
					OpenIncidents: b, CriticalOpenIncidents: c, HasIncidentData: true,
				}, at))

				assertResultSane(t, ComputeAsset(AssetInput{
					Criticality:         float64(a),
					MaxLinkedRiskScore:  float64(b * 13),
					HasLinkedRisks:      true,
					OpenVulnerabilities: c, MaxCVSS: float64(a), HasVulnData: true,
					InternetFacing: a%2 == 0, HasExposureData: true,
				}, at))
			}
		}
	}
}

// assertResultSane checks every invariant a Result must satisfy, whatever it was
// built from.
func assertResultSane(t *testing.T, r Result) {
	t.Helper()

	for label, v := range map[string]float64{
		"value": r.Value, "inherent": r.Inherent, "residual": r.Residual,
	} {
		if math.IsNaN(v) {
			t.Fatalf("%s is NaN (%+v)", label, r.Inputs)
		}
		if v < MinScore || v > MaxScore {
			t.Fatalf("%s = %v, outside [0,100] (%+v)", label, v, r.Inputs)
		}
	}

	// Residual can never exceed inherent: a mitigation must not add exposure.
	if r.Residual > r.Inherent+1e-9 {
		t.Fatalf("residual %v exceeds inherent %v", r.Residual, r.Inherent)
	}
	if r.MitigationEffectiveness < 0 || r.MitigationEffectiveness > 1 {
		t.Fatalf("effectiveness %v outside [0,1]", r.MitigationEffectiveness)
	}

	// The band always travels with the value it describes — this is the
	// structural fix for label/value desynchronisation.
	if r.Band != BandFor(r.Value) {
		t.Fatalf("band %q does not match value %v (want %q)", r.Band, r.Value, BandFor(r.Value))
	}
	if r.BandLabelI18nKey != r.Band.I18nKey() {
		t.Fatalf("band label key %q does not match band %q", r.BandLabelI18nKey, r.Band)
	}
	if r.InherentBand != BandFor(r.Inherent) || r.ResidualBand != BandFor(r.Residual) {
		t.Fatal("inherent/residual bands must match their own values")
	}
	if r.FormulaVersion != FormulaVersion {
		t.Fatalf("formula version = %q, want %q", r.FormulaVersion, FormulaVersion)
	}
	if len(r.Breakdown) == 0 {
		t.Fatal("a score with no breakdown cannot be explained")
	}
}

// ---------------------------------------------------------------------------
// Monotonicity — the property the spec names explicitly.
// ---------------------------------------------------------------------------

// Raising the probability may never lower the score. Checked at every impact and
// criticality, in fine steps.
func TestMonotonic_ProbabilityNeverLowersTheScore(t *testing.T) {
	for _, impact := range []float64{0, 2.5, 5, 7.5, 10} {
		for _, crit := range []float64{0.1, 1, 1.5, 3} {
			prev := -1.0
			for p := 0.0; p <= 1.0+1e-9; p += 0.01 {
				got := ComputeRisk(RiskInput{
					Probability: p, Impact: impact, AssetCriticality: crit, HasAsset: true,
				}, at).Inherent
				if got < prev-1e-9 {
					t.Fatalf("score DROPPED raising probability to %v (impact %v, crit %v): %v → %v",
						p, impact, crit, prev, got)
				}
				prev = got
			}
		}
	}
}

// Raising the impact may never lower the score.
func TestMonotonic_ImpactNeverLowersTheScore(t *testing.T) {
	for _, prob := range []float64{0, 0.25, 0.5, 0.75, 1} {
		for _, crit := range []float64{0.1, 1.5, 3} {
			prev := -1.0
			for i := 0.0; i <= 10.0+1e-9; i += 0.1 {
				got := ComputeRisk(RiskInput{
					Probability: prob, Impact: i, AssetCriticality: crit, HasAsset: true,
				}, at).Inherent
				if got < prev-1e-9 {
					t.Fatalf("score DROPPED raising impact to %v (prob %v, crit %v): %v → %v",
						i, prob, crit, prev, got)
				}
				prev = got
			}
		}
	}
}

// The band must be monotonic too — it is what the user actually reads. A score
// that rises while its label falls is the reported bug.
func TestMonotonic_BandFollowsTheScore(t *testing.T) {
	prevScore, prevRank := -1.0, -1
	for p := 0.0; p <= 1.0+1e-9; p += 0.005 {
		r := ComputeRisk(RiskInput{Probability: p, Impact: 6, AssetCriticality: 2, HasAsset: true}, at)
		if r.Inherent < prevScore-1e-9 {
			t.Fatalf("score fell at p=%v", p)
		}
		if r.InherentBand.Rank() < prevRank {
			t.Fatalf("band fell from rank %d to %d at p=%v while the score rose to %v",
				prevRank, r.InherentBand.Rank(), p, r.Inherent)
		}
		prevScore, prevRank = r.Inherent, r.InherentBand.Rank()
	}
}

// More critical risks, worse control coverage, more KEV findings and more open
// incidents each raise the tenant score.
func TestMonotonic_TenantFactors(t *testing.T) {
	base := TenantInput{
		CriticalRisks: 1, HighRisks: 1, TotalRisks: 10, HasRiskData: true,
		ImplementedControls: 50, ApplicableControls: 100, HasComplianceData: true,
		KEVVulnerabilities: 0, CriticalVulnerabilities: 1, HasVulnData: true,
		OpenIncidents: 1, CriticalOpenIncidents: 0, HasIncidentData: true,
	}
	start := ComputeTenant(base, at).Inherent

	worse := base
	worse.CriticalRisks = 5
	if ComputeTenant(worse, at).Inherent < start {
		t.Error("more critical risks must not lower the tenant score")
	}

	worse = base
	worse.ImplementedControls = 10 // coverage down → gap up
	if ComputeTenant(worse, at).Inherent < start {
		t.Error("worse control coverage must not lower the tenant score")
	}

	worse = base
	worse.KEVVulnerabilities = 3
	if ComputeTenant(worse, at).Inherent < start {
		t.Error("more known-exploited vulnerabilities must not lower the tenant score")
	}

	worse = base
	worse.CriticalOpenIncidents = 2
	if ComputeTenant(worse, at).Inherent < start {
		t.Error("more open critical incidents must not lower the tenant score")
	}
}

// ---------------------------------------------------------------------------
// Explainability — the breakdown must actually reproduce the number.
// ---------------------------------------------------------------------------

// "Zero lies": the contributions must sum to the inherent value, and each must
// equal weight × raw. If the explainer cannot reproduce the score, it is
// decoration.
func TestBreakdown_ReproducesTheValue(t *testing.T) {
	results := []Result{
		ComputeRisk(RiskInput{Probability: 0.7, Impact: 8, AssetCriticality: 2.4, HasAsset: true}, at),
		ComputeRisk(RiskInput{Probability: 0.2, Impact: 3, HasAsset: false}, at),
		ComputeAsset(AssetInput{
			Criticality: 3, MaxLinkedRiskScore: 62, HasLinkedRisks: true,
			OpenVulnerabilities: 4, MaxCVSS: 9.8, HasVulnData: true,
			InternetFacing: true, HasExposureData: true,
		}, at),
		ComputeTenant(TenantInput{
			CriticalRisks: 2, HighRisks: 4, TotalRisks: 20, HasRiskData: true,
			ImplementedControls: 40, ApplicableControls: 100, HasComplianceData: true,
			KEVVulnerabilities: 1, CriticalVulnerabilities: 3, HasVulnData: true,
			OpenIncidents: 2, CriticalOpenIncidents: 1, HasIncidentData: true,
		}, at),
	}

	for _, r := range results {
		var sumContrib, sumWeight float64
		for _, f := range r.Breakdown {
			if !f.Available {
				if f.Weight != 0 || f.Contribution != 0 {
					t.Errorf("%s: unavailable factor %q must carry no weight", r.Scope, f.Key)
				}
				continue
			}
			sumContrib += f.Contribution
			sumWeight += f.Weight
			if want := f.Weight * f.Raw; math.Abs(want-f.Contribution) > 0.15 {
				t.Errorf("%s/%s: contribution %v ≠ weight×raw %v", r.Scope, f.Key, f.Contribution, want)
			}
			if f.LabelI18nKey != f.Key.I18nKey() {
				t.Errorf("%s/%s: label key %q is wrong", r.Scope, f.Key, f.LabelI18nKey)
			}
		}
		if math.Abs(sumWeight-1) > 0.02 {
			t.Errorf("%s: available weights sum to %v, want 1", r.Scope, sumWeight)
		}
		if math.Abs(sumContrib-r.Inherent) > 0.2 {
			t.Errorf("%s: contributions sum to %v but inherent is %v — the explainer would be lying",
				r.Scope, sumContrib, r.Inherent)
		}
	}
}

// The breakdown is ordered by contribution, so the explainer's bars do not
// reshuffle between two identical requests.
func TestBreakdown_StableOrdering(t *testing.T) {
	in := TenantInput{
		CriticalRisks: 3, HighRisks: 2, TotalRisks: 30, HasRiskData: true,
		ImplementedControls: 10, ApplicableControls: 100, HasComplianceData: true,
		KEVVulnerabilities: 2, CriticalVulnerabilities: 1, HasVulnData: true,
		OpenIncidents: 1, HasIncidentData: true,
	}
	first := ComputeTenant(in, at).Breakdown
	second := ComputeTenant(in, at).Breakdown

	for i := range first {
		if first[i].Key != second[i].Key {
			t.Fatalf("ordering is not deterministic at %d: %q vs %q", i, first[i].Key, second[i].Key)
		}
		if i > 0 && first[i-1].Contribution < first[i].Contribution {
			t.Errorf("breakdown is not sorted by contribution: %v before %v",
				first[i-1].Contribution, first[i].Contribution)
		}
	}
}

// An absent source must not read as a good one. Dropping the compliance signal
// must not LOWER the tenant score — that would reward not measuring.
func TestUnavailableFactor_DoesNotFlatterTheScore(t *testing.T) {
	measured := TenantInput{
		CriticalRisks: 2, HighRisks: 2, TotalRisks: 20, HasRiskData: true,
		ImplementedControls: 0, ApplicableControls: 100, HasComplianceData: true,
		HasVulnData: true, HasIncidentData: true,
	}
	blind := measured
	blind.HasComplianceData = false

	withData := ComputeTenant(measured, at)
	without := ComputeTenant(blind, at)

	if without.Inherent > withData.Inherent {
		t.Errorf("removing a signal raised the score (%v → %v)", withData.Inherent, without.Inherent)
	}

	// The factor is still reported, flagged unavailable, so the explainer can say
	// "not measured" instead of showing three factors where the model has four.
	var found bool
	for _, f := range without.Breakdown {
		if f.Key == FactorControlGaps {
			found = true
			if f.Available {
				t.Error("control gaps should be flagged unavailable")
			}
		}
	}
	if !found {
		t.Error("an unavailable factor must still appear in the breakdown")
	}
}

// ---------------------------------------------------------------------------
// Inherent vs residual
// ---------------------------------------------------------------------------

func TestResidual_AppliesMitigationEffectiveness(t *testing.T) {
	in := RiskInput{Probability: 0.8, Impact: 9, AssetCriticality: 2.5, HasAsset: true}

	none := ComputeRisk(in, at)
	if none.Residual != none.Inherent {
		t.Errorf("with no mitigation, residual must equal inherent (%v vs %v)", none.Residual, none.Inherent)
	}

	in.MitigationEffectiveness = 0.5
	half := ComputeRisk(in, at)
	if want := Round1(none.Inherent * 0.5); math.Abs(half.Residual-want) > 0.15 {
		t.Errorf("residual = %v, want %v", half.Residual, want)
	}
	if half.Inherent != none.Inherent {
		t.Error("mitigations must not change the INHERENT score — that is the whole point of reporting both")
	}
	if half.Value != half.Residual {
		t.Error("the displayed value is the residual score")
	}

	in.MitigationEffectiveness = 1
	full := ComputeRisk(in, at)
	if full.Residual != 0 || full.Band != BandLow {
		t.Errorf("fully mitigated should read 0/low, got %v/%q", full.Residual, full.Band)
	}

	// Out-of-range effectiveness is clamped, not trusted.
	in.MitigationEffectiveness = 5
	if over := ComputeRisk(in, at); over.Residual != 0 {
		t.Errorf("effectiveness > 1 must clamp, got residual %v", over.Residual)
	}
	in.MitigationEffectiveness = -3
	if under := ComputeRisk(in, at); under.Residual != under.Inherent {
		t.Errorf("negative effectiveness must clamp to 0, got %v", under.Residual)
	}
}

// ---------------------------------------------------------------------------
// Model integrity
// ---------------------------------------------------------------------------

func TestWeightsSumToOne(t *testing.T) {
	for name, set := range map[string]map[FactorKey]float64{
		"tenant": tenantWeights, "risk": riskWeights, "asset": assetWeights,
	} {
		var sum float64
		for key, w := range set {
			if w <= 0 {
				t.Errorf("%s/%s: weight %v must be positive (a zero-weight factor is dead code, a negative one breaks monotonicity)", name, key, w)
			}
			sum += w
		}
		if math.Abs(sum-1) > 1e-9 {
			t.Errorf("%s weights sum to %v, want 1", name, sum)
		}
	}
}

// The bounds written in code must be the ones documented for humans. A drift
// here is how a form starts accepting values the engine silently clamps.
func TestDocumentedBounds(t *testing.T) {
	cases := []struct {
		name     string
		min, max float64
	}{
		{"probability", MinProbability, MaxProbability},
		{"impact", MinImpact, MaxImpact},
		{"asset_criticality", MinAssetCriticality, MaxAssetCriticality},
		{"score", MinScore, MaxScore},
	}
	for _, tc := range cases {
		if tc.max <= tc.min {
			t.Errorf("%s: max %v must exceed min %v", tc.name, tc.max, tc.min)
		}
	}
	if MinScore != 0 || MaxScore != 100 {
		t.Fatal("the canonical scale is 0–100; changing it invalidates every stored score")
	}
	if BandMediumFloor != 25 || BandHighFloor != 50 || BandCriticalFloor != 75 {
		t.Fatal("band floors are quartiles of the canonical scale; see docs/scoring/SCORE_MODEL.md")
	}
}

// Determinism: the same inputs and the same clock produce byte-identical output.
// Two surfaces asking at the same instant must never disagree.
func TestDeterministic(t *testing.T) {
	in := RiskInput{Probability: 0.63, Impact: 7.2, AssetCriticality: 1.8, HasAsset: true, MitigationEffectiveness: 0.3}
	a, b := ComputeRisk(in, at), ComputeRisk(in, at)

	if a.Value != b.Value || a.Band != b.Band || a.Inherent != b.Inherent || a.Residual != b.Residual {
		t.Fatal("the same inputs produced different scores")
	}
	if len(a.Breakdown) != len(b.Breakdown) {
		t.Fatal("breakdown length differs")
	}
	for i := range a.Breakdown {
		if a.Breakdown[i] != b.Breakdown[i] {
			t.Fatalf("breakdown differs at %d", i)
		}
	}
}

// A risk with no linked asset drops that factor and redistributes its weight —
// it must not be scored as though it touched the least important asset possible.
func TestRisk_WithoutAssetRedistributesWeight(t *testing.T) {
	withAsset := ComputeRisk(RiskInput{Probability: 0.5, Impact: 5, AssetCriticality: 0.1, HasAsset: true}, at)
	without := ComputeRisk(RiskInput{Probability: 0.5, Impact: 5, HasAsset: false}, at)

	if without.Inherent <= withAsset.Inherent {
		t.Errorf("an unlinked risk (%v) should not score below one pinned to the least critical asset (%v)",
			without.Inherent, withAsset.Inherent)
	}

	var sum float64
	for _, f := range without.Breakdown {
		sum += f.Weight
	}
	if math.Abs(sum-1) > 0.02 {
		t.Errorf("weights should renormalise to 1 without the asset factor, got %v", sum)
	}
}

// The canonical example from the spec: weight × raw = contribution.
func TestBreakdown_ArithmeticIsCheckableByEye(t *testing.T) {
	r := ComputeTenant(TenantInput{
		CriticalRisks: 4, HighRisks: 3, TotalRisks: 10, HasRiskData: true,
		ImplementedControls: 30, ApplicableControls: 100, HasComplianceData: true,
		KEVVulnerabilities: 1, CriticalVulnerabilities: 2, HasVulnData: true,
		OpenIncidents: 2, CriticalOpenIncidents: 1, HasIncidentData: true,
	}, at)

	for _, f := range r.Breakdown {
		if f.Key != FactorRiskExposure {
			continue
		}
		if math.Abs(f.Weight-0.40) > 0.01 {
			t.Errorf("risk_exposure weight = %v, want 0.40", f.Weight)
		}
		if math.Abs(f.Weight*f.Raw-f.Contribution) > 0.15 {
			t.Errorf("%v × %v ≠ %v", f.Weight, f.Raw, f.Contribution)
		}
		return
	}
	t.Fatal("risk_exposure factor missing from the tenant breakdown")
}
