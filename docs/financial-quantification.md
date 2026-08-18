# Financial Risk Quantification — methodology (FAIR-lite)

`formula_version: fair-lite-1.0.0`

OpenRisk expresses cyber exposure in money using a **FAIR-lite** model. It is
deliberately simple, fully documented and reproducible — not an actuarial black
box. Every figure in the product links back to this page.

## 1. The model

```
ALE = LEF × LM
```

- **LEF — Loss Event Frequency**: the expected number of loss events per year.
  Sourced from the risk's ARO (Annualized Rate of Occurrence). When a risk has
  no explicit ARO, LEF defaults to `1` and the loss magnitude falls back to the
  reference band for the risk's criticality, so every risk still carries an
  order-of-magnitude figure.
- **LM — Loss Magnitude**: a **3-point PERT distribution** (`min` / most-likely
  / `max`) of the single-event loss, in XAF.
  - `most-likely` = the effective SLE: an explicit SLE if provided, otherwise the
    sum of the loss components (downtime `hours × cost/hour`, regulatory fines,
    data-loss cost, other direct cost), otherwise the reference band.
  - `min` / `max` = the loss band. Explicit bounds win; otherwise they are
    derived from the point estimate (`best = ×0.5`, `worst = ×2.0`).

## 2. Output: a distribution, never a single number

A single figure is a false certainty. The engine runs a **Monte Carlo**
simulation: it draws `N` loss magnitudes from the PERT distribution (via the
standard Beta-PERT with shape λ = 4), scales each by LEF, and reports:

- **P10 / P50 (median) / P90** percentiles — the band shown in the UI, median
  emphasised.
- the mean, which converges to `LEF × (min + 4·mode + max) / 6`.

Default run: **`10 000` iterations**, fixed **seed `20260801`**. The run is fully
**deterministic** — the same inputs always yield the same band.

### Portfolio band

The dashboard headline is **one shared simulation across all risks**: on each
iteration it sums every risk's sampled annual loss. This correctly captures that
not every risk hits its worst case in the same year (diversification), so the
portfolio P90 is *not* the naive sum of per-risk P90s.

## 3. Currency

Amounts are stored in **XAF (FCFA)** and converted to the tenant's chosen display
currency (XAF, XOF, EUR, USD, NGN, MAD, GHS, ZAR) at a **dated reference rate**.
The reference date (`fx_as_of`) is shown next to every converted amount. Rates
are refreshed by a daily FX job; the built-in fallback table uses the fixed CFA
euro peg (1 EUR = 655.957 XAF) and rounded market approximations for the rest.
The currency is chosen at onboarding and changeable later (admin).

## 4. Explainability

Every amount is clickable → a **Methodology** panel listing: the model, the exact
intrants used and their provenance (`input` / `derived` / `reference`), the
assumptions, the iteration count, the seed, the calc date, the `formula_version`,
the FX rate + date, and a link back to this document.

## 5. ROSI — investment simulator

```
ROSI = (ALE_before − ALE_after − Cost) / Cost
```

with `ALE_after = ALE_before × (1 − effectiveness)`, effectiveness ∈ [0, 1].
The simulator turns three inputs (budget, effectiveness, and the concrete measure
name) into one plain-language decision sentence, including the payback period.
ROSI is undefined when the cost is 0 (nothing to divide by) — the UI says so
rather than showing a misleading number.

## 6. Honest limits

- The PERT band factors (×0.5 / ×2.0) are defensible defaults; a real deployment
  tunes them per sector.
- Reference bands per criticality are order-of-magnitude, not measured losses.
- LEF sampling is treated as a deterministic annual multiplier on the magnitude
  distribution (the uncertainty lives in LM), which keeps the median meaningful
  for low-frequency risks rather than collapsing it to zero.
