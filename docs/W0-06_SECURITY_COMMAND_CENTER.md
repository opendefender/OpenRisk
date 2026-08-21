# W0-06 — Security Command Center

> Status of this document: **audit inventory** (commit 1 of the wave). Sections
> marked *pending* are filled in as the wave implements them.

## Executive Summary

The Home page is not one dashboard. It is a **dispatcher over six persona
dashboards**, selected from the signed-in member's `business_role`, plus a
seventh display mode (`?view=executive`) that renders the consolidated executive
dashboard in place.

That matters for this wave because the previous cleanups only ever reached one of
the six. `PostureDashboard` — the default, the one an admin sees — was rebuilt on
server aggregates during the bank-grade wave and carries explicit comments about
why no KPI may be derived from a page of the register. **The other five personas
were never given the same treatment.** They still carry the exact patterns the
posture dashboard removed:

* `ViewerDashboard` falls back to `risks.filter(...).length` over **one page** of
  the register whenever `/stats` fails — a page-derived count rendered as a
  tenant total, on the persona least able to notice.
* `EstateDashboard` derives every asset statistic client-side, by downloading the
  **entire inventory with its risk associations preloaded**, to display four
  numbers.
* `ExecDashboard` renders `?? 0` for every value, so a failed
  `/analytics/executive` shows an exposure of **0 FCFA** and a cyber score of
  **0** with no indication that nothing was read.
* `AuditDashboard` renders an API failure as *"No framework — import one to
  start"*, which is a different and much more damaging statement than "we could
  not read your compliance posture".

Beyond the personas, three structural findings apply to the Command Center as a
whole:

1. **The risk-trend widget is computed on the client from one page of risks**,
   bands them on a field the codebase elsewhere documents as unreliable, and
   plots a cumulative-by-`created_at` series that can only ever rise. Its
   7d/30d/90d control is local to the widget and changes nothing else on the
   page — the pattern §15 of the brief names explicitly.
2. **Deep links carry no filter state.** Every KPI on every persona navigates to
   a bare list URL. Clicking *Critical — 3* lands on an unfiltered register,
   although the register's filters have lived in the URL since the DataTable
   migration (`/risks?f.criticality=critical` is a valid, server-filtered link
   today).
3. **The dashboard cache-key model is tenant-unsafe**, and is only harmless
   because the cache is a no-op. `CacheDashboardStatsGET` keys on
   `dashboard:stats:{period}` with no tenant component; `WrapWithCache` currently
   returns the handler unchanged and ignores the key entirely. The keys are dead
   code that would become a cross-tenant disclosure the moment anyone implements
   the wrapper they are written for.

A fourth is smaller but worth naming: `/stats` still ships
`global_risk_score = 100 - avg(score) × 4`, a **second security-score formula**
that competes with the canonical `/score` model. Nothing on Home reads it any
more — the posture hero moved to `useScore('tenant')` — but it remains in the
contract, undocumented, on a different scale, and is exactly the kind of field a
future widget picks up because it is already there.

---

## Current Dashboard Architecture

```
GET /                       (React route, index of the app shell)
  └── DashboardPage         features/dashboard/DashboardPage.tsx
        ├── ?view=executive → ExecutiveDashboard  (features/analytics)
        └── personaFor(user.business_role)
              ├── analyst → AnalystDashboard
              ├── exec    → ExecDashboard
              ├── audit   → AuditDashboard
              ├── estate  → EstateDashboard
              ├── viewer  → ViewerDashboard
              └── default → PostureDashboard      (admin, RSSI, risk roles)
```

`personaFor` is defined in `features/dashboard/dashboardPersona.ts`; an admin or
an unmapped role gets `posture`.

---

## Widget Inventory

State legend: `REAL` (database → aggregation → contract → widget, coherent and
tenant-scoped) · `PARTIAL` (real source, but the number displayed does not mean
what the label says) · `DECEPTIVE` (a value that is not read from the tenant's
data, or an error rendered as data).

### PostureDashboard — default persona

| # | Widget | Source | Endpoint | Tenant scope | Time scope | Deep link | State |
|---|--------|--------|----------|--------------|-----------|-----------|-------|
| 1 | Score hero | `useScore('tenant')` | `GET /score?scope=tenant` | server, from JWT | none (live) | `/score` | `REAL` |
| 2 | KPI — total risks | `/stats` | `GET /stats` | server, fail-closed | none | `/risks` (unfiltered) | `PARTIAL` — link drops the metric |
| 3 | KPI — critical | `/stats.risks_by_severity` | `GET /stats` | server | none | `/risks` (unfiltered) | `PARTIAL` |
| 4 | KPI — in treatment | `/stats.in_progress_risks` | `GET /stats` | server | none | `/risks` (unfiltered) | `PARTIAL` |
| 5 | KPI — mitigated | `/stats.mitigated_risks` | `GET /stats` | server | none | `/risks` (unfiltered) | `PARTIAL` |
| 6 | 5×5 heatmap | `/stats.risk_matrix` | `GET /stats` | server | none | none | `REAL` |
| 7 | Risk trend (7/30/90) | **client, `useRiskStore.risks`** | `GET /risks` (page 1) | server for the page | client-side, widget-local | none | `DECEPTIVE` — page-derived, unreliable band, cumulative-only |
| 8 | Recent activity | `useRiskStore.risks[0..4]` | `GET /risks` (page 1) | server | none | `/risks` (unfiltered, not the row) | `PARTIAL` |
| 9 | War Room | `useIncidents({limit:20})` | `GET /incidents` | server | none | `/incidents/:id/war-room` | `REAL` |
| 10 | Onboarding checklist | `GET /activation/state` | server-derived | server | none | per step | `REAL` |

### ViewerDashboard

| # | Widget | Source | State |
|---|--------|--------|-------|
| 11 | Score hero | `/stats.global_risk_score` via `ScoreHero` | `PARTIAL` — the *other* score formula, not `/score` |
| 12 | KPI — risks | `stats?.total_risks ?? (total \|\| risks.length)` | `DECEPTIVE` — page-derived fallback on API failure |
| 13 | KPI — critical | `… ?? risks.filter(critOf === 'critical').length` | `DECEPTIVE` |
| 14 | KPI — mitigated | `… ?? risks.filter(/mitigat\|resolv\|closed\|accept/).length` | `DECEPTIVE` |
| 15 | Recent risks | `risks.slice(0,6)` | `PARTIAL` |

### EstateDashboard

| # | Widget | Source | State |
|---|--------|--------|-------|
| 16 | KPI — assets | `assets.length` over the full `GET /assets` collection | `PARTIAL` — correct total, but no aggregate and no error state |
| 17 | KPI — critical / high | client `reduce` over the same collection | `PARTIAL` |
| 18 | KPI — types | `new Set(assets.map(a => a.type)).size` | `PARTIAL` |
| 19 | Critical assets list | client sort/slice | `PARTIAL` — deep-links `?focus=` correctly |
| 20 | By-criticality bars | client `reduce` | `PARTIAL` |

`GET /assets` returns the whole inventory **with `Preload("Risks")`**, by design
(the Asset Universe graph needs every node). Using it as the source of four
counters means the browser downloads the estate and its many-to-many risk join to
render a number that one `GROUP BY` would have produced.

### AnalystDashboard

| # | Widget | Source | State |
|---|--------|--------|-------|
| 21 | KPI — vulns / open / P1 / KEV | `GET /vulnerabilities/stats` (SQL aggregate) | `REAL` — but `?? 0` on failure, and links are unfiltered |
| 22 | Priority queue | `GET /vulnerabilities?limit=6&sort_by=priority_score` | `REAL` |
| 23 | Severity distribution | `stats.by_severity` | `REAL` |

### ExecDashboard

| # | Widget | Source | State |
|---|--------|--------|-------|
| 24 | Cyber score A–F | `GET /analytics/executive.cyber_score` | `PARTIAL` — `?? 0` renders a failure as score 0 |
| 25 | ALE / worst case | `.financial` | `PARTIAL` — `?? 0` renders a failure as 0 FCFA |
| 26 | Risks / quantified | `.financial` | `PARTIAL` |
| 27 | KRI band | `.kris` | `PARTIAL` — empty on failure and on genuinely empty alike |

### AuditDashboard

| # | Widget | Source | State |
|---|--------|--------|-------|
| 28 | KPI — frameworks / coverage / controls / gaps | `useComplianceOverview()` | `PARTIAL` — no error state |
| 29 | Coverage per framework | same | `PARTIAL` — API failure renders "no framework, import one" |

### Orphans

`features/dashboard/widgets/GlobalScore.tsx` and
`features/dashboard/widgets/RiskHeatmap.tsx` are **unreferenced fixture
components**: a hardcoded five-point heatmap (`Database Injection`, `Phishing`,
`DDOS`…) and a gauge that states *"Votre posture de sécurité est optimale"*
regardless of the score passed to it. Not rendered anywhere, but live in the
dashboard tree ready to be re-imported.

---

## Security Score — provenance

**Two score contracts are in the codebase, on two scales, both labelled
"score".**

### 1. The canonical model — `GET /score` (`internal/domain/scoring`)

This is what the posture hero, the sidebar and `/score` all render, and they
render the same object: `useScore(scope, id)` is one TanStack query key, so the
surfaces cannot disagree.

```
score(tenant) = 100 − Σ (normalised_weightᵢ × rawᵢ)   [0..100, 100 = safe]

factors (tenant scope)
  risk_exposure          register criticality mix
  control_gaps           applicable controls vs gaps
  vulnerability_pressure open / KEV / severity mix
  incident_pressure      open and critical-open incidents
```

Each factor carries `weight`, `raw` (0–100, 100 always worst), `contribution`
(`weight × raw`, so the parts sum to the whole and can be checked by eye) and
`available`. **A source that cannot be read is flagged unavailable and its weight
is redistributed** — deliberately not scored zero, which on a safety scale would
read as *excellent* and is the most dangerous failure mode a security score has.
The response also carries `inherent` / `residual` / `mitigation_effectiveness`,
`formula_version`, `computed_at` and the `inputs` actually used.

`GET /score/model` serves the model's self-description (scale, bands, weights)
so the explainer never hardcodes a threshold.

### 2. The legacy formula — `GET /stats.global_risk_score`

```
global_risk_score = 100 − round(avg(risks.score) × 4)   , clamped at 0
                  = 100 when the register is empty
```

Computed inline in `dashboard_handler.go` over `AVG(score)` of the register
only — no controls, no vulnerabilities, no incidents, no availability
accounting. It is on the same 0–100 scale as the canonical score and will
disagree with it for any tenant that has compliance or vulnerability data.

`ViewerDashboard` still renders this one, so **two personas of the same product
show two different "security scores" for the same tenant.**

*Disposition: pending (§7 of the brief).*

---

## Asset Statistics

*Pending.* Finding: there is no asset-statistics contract. Every number on the
estate persona is a client-side reduction over the full `GET /assets` collection.

## Dashboard Query Strategy

*Pending (§12).*

## Tenant Isolation

`GET /stats` fails closed: no resolved tenant → `401`, explicitly rather than
querying every tenant's risks. `GET /score`, `/analytics/executive`,
`/vulnerabilities/stats` and `/assets` all take the tenant from the authenticated
context and never from a request parameter.

The **cache-key model does not**, and is examined under *Cache Strategy*.

## Time Filtering

*Pending (§14).* Today: no contract on Home accepts a period. The only temporal
control in the Command Center is `TrendCard`'s 7/30/90 toggle, which is local
component state, drives one client-side computation, and is not reflected in the
URL.

## Filter Propagation

*Pending (§15).*

## Deep-Link Semantics

*Pending (§16).* Today: every dashboard KPI navigates to a bare list route. The
destinations already read filters from the URL (`?f.<facet>=v1,v2`, `?q=`,
`?sort=`, `?focus=`), so no destination work is required to fix this — only the
links.

Facets available at each destination:

| Destination | Facet keys | Filtering |
|-------------|-----------|-----------|
| `/risks` | `f.criticality`, `f.status`, `f.phase`, `f.category_id`, `f.source` | server-side |
| `/vulnerabilities` | `f.tier`, `f.severity`, `f.status`, `f.kev` | server-side |
| `/assets` | `f.type`, `f.criticality` | client-side (collection endpoint) |

## Data Reconciliation

*Pending (§19, §34).*

## Cache Strategy

**Finding — latent cross-tenant cache keys.**

`internal/handler/cache_integration.go` builds cache keys for the dashboard and
the register with no tenant component:

```go
dashboard:stats:{period}          // CacheDashboardStatsGET
dashboard:matrix:all              // CacheDashboardMatrixGET
dashboard:timeline:{days}         // CacheDashboardTimelineGET
risk:list:page:{p}:sev:{s}:status:{st}
```

`GET /stats`, `/stats/risk-distribution`, `/stats/mitigation-metrics` and
`/stats/top-vulnerabilities` are all registered through `CacheDashboardStatsGET`.

This is **not** a live disclosure, for one reason only:

```go
// pkg/cache/decoration.go
func (d *CacheDecoration) WrapWithCache(handler fiber.Handler, _ func(*fiber.Ctx) string, _ time.Duration) fiber.Handler {
	return handler
}
```

The wrapper discards the key builder and the TTL and returns the handler
unchanged. Every dashboard cache key above is dead code — written, registered,
and never evaluated. The risk is that it does not *look* dead at the call site in
`main.go`, and the day someone implements the wrapper the dashboard becomes a
cross-tenant leak on the first cache hit.

*Disposition: pending (§20).*

Client-side: the score family uses TanStack Query with a 30 s stale time; the
whole client cache, the Zustand stores and the user-scoped `localStorage` keys
are dropped by `lib/sessionScope.ts` at both ends of a session transition
(login/adoptSession and logout), added in W0-05. That clearing is keyed on the
**user**, not the tenant.

`useDashboardStats` is not in the query cache at all: it is a bare
`useEffect(..., [])` + `useState`, so it fetches once per mount, never refetches,
and has no key to invalidate.

## Loading / Empty / Error / Permission States

*Pending (§23–§26).* Current coverage:

| Persona | Loading | Empty | Error | Permission-denied |
|---------|---------|-------|-------|-------------------|
| Posture | yes (skeletons) | yes (per widget) | yes (heatmap) | — |
| Viewer | no | no | **no** (falls back to page counts) | — |
| Estate | list only | list only | **no** | — |
| Analyst | list only | list only | **no** | — |
| Exec | **no** | partial | **no** (renders 0) | — |
| Audit | list only | list only | **no** (renders "no framework") | — |

## API Contracts

*Pending (§31).*

## Database Changes

*Pending (§30).*

## Test Strategy · Reconciliation Test Matrix · Security Validation · Accessibility · Performance

*Pending (§33–§37).*

## Live Proof

See `docs/W0-06_SECURITY_COMMAND_CENTER_LIVE_PROOF.md`.

## Known Limitations

*Pending.*

## Follow-up Work

*Pending.*
