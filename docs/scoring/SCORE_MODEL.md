# The OpenRisk score model

**Formula version: `2.1`** · Canonical scale: **0–100** · Source of truth: `backend/internal/domain/scoring/`

This document is the human-readable half of the model. The machine-readable half
is `GET /api/v1/score/model`, and the enforcement half is
`backend/internal/domain/scoring/*_test.go`. If the three ever disagree, the tests
are right — `TestDocumentedBounds` exists to make that disagreement fail loudly.

---

## 1. Why there is only one

Before this model there were, demonstrably, **four incompatible band mappings**
shipping at the same time, each deriving a label on the client from a different
set of thresholds:

| Where | Thresholds | Scale it assumed |
|---|---|---|
| `frontend/src/shared/riskColors.ts` | ≥7 / ≥4 / ≥2 | 0–30 (P×I×AC) |
| `frontend/src/features/risks/RiskRegisterPage.tsx` | ≥15 / ≥8 / ≥4 | 0–30 |
| `docs/AUDIT_MANIFEST.md` (shipped badges) | ≥15 / ≥9 / ≥5 | 0–30 |
| `frontend/src/features/mitigations/MitigationCard.tsx` | ≥40 | 0–100 |

…while the dashboard hero read `stats.global_risk_score` and the sidebar read
`analytics/executive → cyber_score.score` — two different quantities, computed by
two different backends, on two different scales, **pointing in opposite
directions** (one where higher is better, one where higher is worse), both
labelled "score".

That is the whole bug report, and it has three named symptoms:

1. **dashboard ≠ sidebar** — two sources, two answers.
2. **the label is stuck on "low" while the number moves** — the badge came from a
   different mapping, on a different scale, than the number next to it.
3. **the range defect** — scores on the 0–30 scale were rendered as `x / 100`
   (see `RiskCard.tsx`), so a maximal risk displayed as 30 % of the scale; and
   with bands cut at 7, **77 % of the possible range collapsed into a single
   label**, which is *why* the label stopped tracking the number.

The fix is structural, not cosmetic:

- the calculation exists **only** in Go, in `internal/domain/scoring`;
- **the band is computed with the value, server-side**, and travels with it in the
  same response — there is no code path that produces one without the other, so
  desynchronisation is unrepresentable;
- every producer clamps into `[0,100]`, and `Clamp` is the only way in.

> **Note on a deviation from the brief.** The task's example response showed
> `"value": 63, "band": "moderate"`. This model uses the product's existing
> four-band vocabulary — `low` / `medium` / `high` / `critical` — the same words
> used by `domain.AssetCriticality`, the vulnerability tiers, `pkg/scoring`'s
> smart model and the `--low/--medium/--high/--critical` design tokens. So 63
> reads as **`high`**, not `moderate`. Introducing a fifth vocabulary for one more
> surface is how a codebase acquires four incompatible band mappings in the first
> place. The response *shape* is exactly as specified.

---

## 2. The scale and the bands

The canonical scale is **0–100, where 100 is worst** (maximum exposure). Every
scope, every factor and every display uses it. There is no second scale.

| Band | Range | i18n key |
|---|---|---|
| `low` | `[0, 25)` | `score.band.low` |
| `medium` | `[25, 50)` | `score.band.medium` |
| `high` | `[50, 75)` | `score.band.high` |
| `critical` | `[75, 100]` | `score.band.critical` |

Floors are **inclusive**: exactly `25` is `medium`, exactly `50` is `high`,
exactly `75` is `critical`. The nine boundary values the spec requires — `0, 24,
25, 49, 50, 74, 75, 99, 100` — are pinned in `TestBandFor_Boundaries`, and the
three values just below each floor in `TestBandFor_JustBelowFloors`.

Bands are **evenly spaced quartiles**, deliberately. The previous thresholds cut
the range at 23 % of its span, so almost everything real landed in the top band
and the label stopped carrying information. Quartiles make the label move with the
number, which is the only reason to have a label.

---

## 3. Bounds

### Output bounds

`0 ≤ value ≤ 100`, always, for `value`, `inherent` and `residual`.

`Clamp` is the single gate, and it is deliberately opinionated about non-finite
input:

| Input | Clamped to | Why |
|---|---|---|
| `NaN` | `0` (floor) | A score that could not be computed is **not** maximum risk. Rendering `NaN` as a bar width silently draws nothing. |
| `-Inf` | `0` | |
| `+Inf` | `100` | |
| `< 0` | `0` | |
| `> 100` | `100` | |

`TestClampAndBand_NeverEscapeTheScale` is the non-regression test for the range
defect: it sweeps negatives, values above 100, the old 0–30 scale's values, and
all three non-finite values, and asserts nothing escapes `[0,100]` and no band
falls off the ladder.

### Input bounds

These are the accepted ranges of the raw measurements. They match the domain's
existing scales exactly — the Score Engine's `P × I × AssetCriticality` uses the
same three.

| Input | Range | Notes |
|---|---|---|
| `probability` | `[0.0, 1.0]` | Not 1–5. A form offering 1–5 is a form that will be silently clamped. |
| `impact` | `[0.0, 10.0]` | |
| `asset_criticality` | `[0.1, 3.0]` | `domain.AssetCriticality.ScoreFactor()`: low 0.5 · medium 1.5 · high 2.5 · critical 3.0 |
| any factor `raw` | `[0, 100]` | Normalised, 100 = worst |
| `mitigation_effectiveness` | `[0.0, 1.0]` | Clamped; no mitigation may increase exposure |

Out-of-range inputs are **clamped, not rejected**, at the scoring layer — a score
must always be computable. Rejection belongs at the API/validation boundary, where
the user can be told what went wrong.

---

## 4. The formula

Every scope uses the same shape: a **weighted sum of factors**, each measured on
0–100 where 100 is worst.

```
inherent = Σ (normalised_weightᵢ × rawᵢ)      over available factors
residual = inherent × (1 − mitigation_effectiveness)
band     = BandFor(residual)
```

Weighted-additive rather than multiplicative, with the trade-off stated openly: in
a product model a single zero factor zeroes the whole score, which reads well for
"probability = 0" but means one unmeasured dimension can erase a real exposure.
Additive keeps every dimension visible and buys two properties that matter more:

- **the breakdown reproduces the number.** `contribution = weight × raw`, and the
  contributions sum to the value. Anyone reading the response can verify the score
  by hand. A score nobody can reproduce is a score nobody should act on.
- **monotonicity.** All weights are positive, so raising probability or impact can
  never lower the score. This is a test, not a hope:
  `TestMonotonic_ProbabilityNeverLowersTheScore`,
  `TestMonotonic_ImpactNeverLowersTheScore`, and
  `TestMonotonic_BandFollowsTheScore` (the band must not fall while the score
  rises — that is the reported bug, as a property).

### Unavailable factors

A factor whose source could not be consulted is **excluded and its weight
redistributed** across the rest — never scored as zero. Scoring an unknown as zero
means "excellent", which is the most dangerous failure mode a security score has.
`TestUnavailableFactor_DoesNotFlatterTheScore` asserts that removing a signal can
never lower the score.

The factor is still returned, flagged `"available": false`, so the explainer can
say **"not measured"** rather than quietly showing three factors where the model
has four.

### Weights

**Tenant** — the organisation's posture:

| Factor | Weight | Measurement |
|---|---|---|
| `risk_exposure` | 0.40 | Severity-weighted share of the register (`2·critical + high`), floored by an absolute term so 3 critical risks out of 3 is not scored like 3 out of 300 |
| `control_gaps` | 0.25 | `100 − compliance coverage` |
| `vulnerability_pressure` | 0.20 | `20·KEV + 5·critical`, clamped — a known-exploited vulnerability weighs far more than a merely critical one, because one is being used against people today |
| `incident_pressure` | 0.15 | `8·open + 20·critical_open`, clamped |

**Risk** — one entry in the register:

| Factor | Weight | Measurement |
|---|---|---|
| `probability` | 0.40 | normalised over `[0,1]` |
| `impact` | 0.40 | normalised over `[0,10]` |
| `asset_criticality` | 0.20 | normalised over `[0.1,3.0]`; **excluded** when the risk has no linked asset |

**Asset**:

| Factor | Weight | Measurement |
|---|---|---|
| `criticality` | 0.35 | normalised over `[0.1,3.0]` |
| `linked_risk_exposure` | 0.35 | worst inherent score among the risks touching this asset, on the canonical scale |
| `vulnerability_pressure` | 0.20 | `0.7·(maxCVSS/10) + 0.3·log-scaled volume` — severity dominates; the tenth finding on a host matters far less than the first |
| `internet_exposure` | 0.10 | **not wired yet** — see §7 |

`TestWeightsSumToOne` asserts each set sums to 1 and that no weight is zero or
negative (a zero-weight factor is dead code; a negative one breaks monotonicity).

---

## 5. Inherent vs residual

Every response carries both, because an auditor asks for both and a product that
shows one is answering half the question.

- **inherent** — exposure **before** any mitigation credit. It does not move when
  a treatment plan advances. That is the point: it is the size of the problem.
- **residual** — what remains **after** applied mitigations:
  `residual = inherent × (1 − effectiveness)`.
- **`value`** — the number surfaces display: the **residual** score.

`mitigation_effectiveness ∈ [0,1]` is resolved in this order:

1. the risk's explicitly declared `MitigationEffectiveness` (the CRQ field an
   analyst sets deliberately) — a stated figure beats an inferred one;
2. otherwise, the mean completion of the risk's mitigation plans, **excluding
   cancelled ones** (counting them would let someone lower a residual score by
   planning work and abandoning it), **capped at 0.90** — a treatment plan reduces
   exposure, it does not eliminate the risk, so the residual score stays honestly
   non-zero however complete the plan.

At tenant scope the credit is the **mean** declared effectiveness across the
register, not the best: one well-treated risk does not mitigate an organisation.

---

## 6. The API

```
GET  /api/v1/score?scope=tenant|risk|asset&id=<uuid>
POST /api/v1/score/preview        # live form preview, persists nothing
GET  /api/v1/score/model          # this document, machine-readable
```

> The brief wrote these as `/api/score…`. They are mounted under the
> application's existing `/api/v1` prefix, alongside every other endpoint.

Response:

```json
{
  "scope": "tenant",
  "value": 63,
  "band": "high",
  "band_label_i18n_key": "score.band.high",
  "inherent": 70,
  "inherent_band": "high",
  "residual": 63,
  "residual_band": "high",
  "mitigation_effectiveness": 0.1,
  "computed_at": "2026-08-10T12:00:00Z",
  "formula_version": "2.1",
  "inputs": { "critical_risks": 4, "applicable_controls": 100, "…": "…" },
  "breakdown": [
    { "factor": "risk_exposure", "weight": 0.4, "raw": 71, "contribution": 28.4,
      "label_i18n_key": "score.factor.risk_exposure", "available": true }
  ]
}
```

`inputs` echoes the measurements the calculation actually used. That is what lets
a user say *"that impact figure is wrong"* instead of *"the score feels wrong"*.

`POST /score/preview` runs the **same** model as the persisted score. A preview
computed by a different formula would be a lie told at exactly the moment the user
is deciding. The client debounces it at 300 ms.

---

## 7. Honest limits

- **`internet_exposure` is not wired.** `domain.Asset` carries no reachability
  signal today — no tags, and `Type` alone does not settle it (a "Server" may be
  air-gapped or public). The factor is reported as `"available": false` and its
  weight redistributed. Putting a guess there would be a number on screen that
  nothing in the database supports.
- **The classic Score Engine is untouched.** `pkg/scoring`'s `P × I ×
  AssetCriticality` on 0–30 remains the invariant behind `Risk.Score` and the
  stored value. This model is what the UI *displays*; it consumes the same inputs
  but is not a replacement, and `Risk.Score` was not rewritten.
- **Band re-calibration changes what existing risks are called.** A risk that was
  "critical" under the old ≥7-of-30 cut may now read `medium`. That is the
  correction, not a regression: the old cut called 77 % of the range critical.
- **The tenant score's direction is flipped from the old sidebar figure.** The
  sidebar used to show a *cyber score* where higher was better (a grade). This is
  an *exposure* score where higher is worse. One number, one direction, one
  meaning — but the sidebar's colour semantics change with it.

---

## 8. Changing the model

1. Edit `internal/domain/scoring/` — nowhere else.
2. **Bump `FormulaVersion`.** It travels in every response so a stored or cached
   score can always be traced to the model that produced it.
3. Update the tables above, and run the tests: `TestDocumentedBounds`,
   `TestWeightsSumToOne` and the boundary table are there to catch drift between
   this document and the code.
