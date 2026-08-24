# W0-06 — Security Command Center

## Executive Summary

The Home page is not one dashboard. It is a **dispatcher over six persona
dashboards**, chosen from the signed-in member's `business_role`, plus a seventh
display mode (`?view=executive`).

That is why the previous cleanups did not finish the job. `PostureDashboard` —
the default, the one an admin sees — had already been rebuilt on server
aggregates and carries comments about why no KPI may be derived from a page of
the register. **The other five personas had never been given the same
treatment**, and each had found its own way to print a number nobody had read:

* **Viewer** fell back to `risks.filter(...).length` over one page of the register
  whenever `/stats` failed, and rendered a *second* security score.
* **Exec** rendered every value as `?? 0`, so a failed fetch produced a cyber
  score of 0 at grade F and an annual exposure of 0 FCFA.
* **Audit** rendered a failed compliance read as *"No framework — import one to
  start"*.
* **Estate** downloaded the whole inventory, with risk associations preloaded, to
  display four numbers.
* **Analyst** reported no open vulnerabilities and no KEV when its stats call
  failed.

Above the personas, four structural findings:

1. **The trend was computed in the browser from one page of risks**, banded on a
   field the codebase documents as unreliable, plotted cumulatively so it could
   only rise. Its 7/30/90 toggle was local state that changed nothing else.
2. **No deep link carried filter state.** Clicking *Critical — 3* landed on an
   unfiltered register, though the destinations have read filters from the URL
   since the DataTable migration.
3. **Every dashboard and register cache key was global** — `dashboard:stats:month`,
   `risk:list:page:1:sev::status:` — harmless only because `WrapWithCache` never
   evaluates them.
4. **Two security scores shipped at once**, on scales running in opposite
   directions, both labelled "score".

All of it is resolved. 29 widgets were inventoried; 29 now have a contract.

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

Shared, so a persona cannot invent its own behaviour:

| Module | Responsibility |
| --- | --- |
| `features/dashboard/period.ts` | The selected period. URL-backed, mirrors `internal/domain/timeframe`. |
| `features/dashboard/PeriodControl.tsx` | The selector, and the sentence naming what it narrows. |
| `features/dashboard/deepLinks.ts` | Where a tile goes, and what it carries. |
| `features/dashboard/WidgetState.tsx` | The five non-data states. Never renders children with a placeholder. |
| `features/dashboard/useCommandCenter.ts` | The keyed queries. |

---

## Widget Inventory

State: `REAL` — database → aggregation → contract → widget, tenant-scoped and
reconcilable. No widget on Home is `EXPERIMENTAL` or `UNAVAILABLE`.

### PostureDashboard (default persona)

| # | Widget | Contract | Period | Deep link | State |
| --- | --- | --- | --- | --- | --- |
| 1 | Score hero | `GET /score?scope=tenant` | n/a | `/score` | REAL |
| 2 | KPI — total risks | `/stats.total_risks` | stock | `/risks?sort=score:desc` | REAL |
| 3 | KPI — critical | `/stats.risks_by_severity.CRITICAL` | stock | `/risks?f.criticality=critical` | REAL |
| 4 | KPI — in treatment | `/stats.in_progress_risks` | stock | `/risks?f.status=in_progress` | REAL |
| 5 | KPI — mitigated | `/stats.mitigated_risks` | stock | `/risks?f.status=mitigated` | REAL |
| 6 | Opened in period | `/stats.opened_in_period` | **flow** | — | REAL |
| 7 | 5×5 heatmap | `/stats.risk_matrix` | stock | — | REAL |
| 8 | Risk trend | `/stats.risk_trend` | **flow** | — | REAL |
| 9 | Recent activity | `GET /risks` (page 1) | n/a | `/risks?focus=<id>` | REAL |
| 10 | War Room | `GET /incidents` | n/a | `/incidents/:id/war-room` | REAL |
| 11 | Onboarding checklist | `GET /activation/state` | n/a | per step | REAL |

### ViewerDashboard

| # | Widget | Contract | Change |
| --- | --- | --- | --- |
| 12 | Score hero | `GET /score?scope=tenant` | was `/stats.global_risk_score` — a different formula, and a gauge that painted high scores green |
| 13 | KPI — risks | `/stats.total_risks` | page-derived fallback removed |
| 14 | KPI — critical | `/stats.risks_by_severity` | page-derived fallback removed |
| 15 | KPI — mitigated | `/stats.mitigated_risks` | page-derived fallback removed |
| 16 | Recent risks | `GET /risks` | now deep-links `?focus=<id>` |

### EstateDashboard

| # | Widget | Contract | Deep link |
| --- | --- | --- | --- |
| 17 | KPI — assets | `/assets/statistics.total` | `/assets` |
| 18 | KPI — critical | `.by_criticality.CRITICAL` | `/assets?f.criticality=critical` |
| 19 | KPI — high | `.by_criticality.HIGH` | `/assets?f.criticality=high` |
| 20 | KPI — types | `.distinct_types` | `/assets` |
| 21 | Added in period | `.added_in_period` | — |
| 22 | Uncategorised / truncated notes | `.uncategorised`, `.types_truncated` | — |
| 23 | Critical assets list | `GET /assets` (needs rows) | `/assets?focus=<id>` |
| 24 | By-criticality bars | `.by_criticality` | `/assets?f.criticality=<band>` |

### AnalystDashboard

| # | Widget | Contract | Deep link |
| --- | --- | --- | --- |
| 25 | KPI band (total/open/P1/KEV) | `GET /vulnerabilities/stats` | `/vulnerabilities?f.tier=P1` etc. |
| 26 | Priority queue | `GET /vulnerabilities?sort_by=priority_score` | `?focus=<id>` |
| 27 | Severity distribution | `.by_severity` | `/vulnerabilities?f.severity=<sev>` |

### ExecDashboard

| # | Widget | Contract |
| --- | --- | --- |
| 28 | Cyber score, ALE, worst case, risks, quantified | `GET /analytics/executive` |
| 29 | KRI band | `.kris` |

### AuditDashboard

| # | Widget | Contract |
| --- | --- | --- |
| 30 | KPI band (frameworks/coverage/controls/gaps) | compliance overview |
| 31 | Coverage per framework | same, row links to the framework |

### Removed

`features/dashboard/widgets/GlobalScore.tsx` (asserted *"votre posture de sécurité
est optimale"* regardless of the score passed to it) and
`features/dashboard/widgets/RiskHeatmap.tsx` (a five-point matrix of invented
risks). Both unreferenced, both sitting in the dashboard tree ready to be
imported by name.

---

## Security Score

**One model, one endpoint, one number.** `GET /score` serves the canonical score
for the tenant, a risk or an asset, and `useScore(scope, id)` is a single
TanStack query key — so the dashboard hero, the sidebar footer and `/score`
render the same object from the same fetch and cannot disagree.

```
score = Σ (normalised_weightᵢ × rawᵢ)          0–100, where 100 is WORST
bands: <25 low · ≥25 medium · ≥50 high · ≥75 critical

tenant factors
  risk_exposure           the register's criticality mix
  control_gaps            applicable controls vs gaps
  vulnerability_pressure  open / KEV / severity mix
  incident_pressure       open and critical-open incidents
```

### Provenance

Every response carries the working:

| Field | Meaning |
| --- | --- |
| `breakdown[].weight` | normalised share, summing to 1 across *available* factors |
| `breakdown[].raw` | that factor's own measurement, 0–100, 100 always worst |
| `breakdown[].contribution` | `weight × raw` — the parts sum to the whole, checkable by eye |
| `breakdown[].available` | false when the source could not be read |
| `inherent` / `residual` / `mitigation_effectiveness` | exposure before and after treatment |
| `inputs` | the measurements actually used |
| `formula_version`, `computed_at` | which model, and when |

**An unavailable factor is excluded and its weight redistributed**, never scored
zero — on a scale where 100 is worst, zero reads as *excellent*, which is the
most dangerous failure a security score can have. Proven live: with no framework
imported, `control_gaps` reports `available: false`, weight `0.000`, and the other
three renormalise to exactly `1.0`.

`GET /score/model` serves the model's self-description, so no client hardcodes a
threshold. `ScoreGauge` takes a `Score` object — value and band together — and
has no prop through which a caller could pass a bare number and let the component
pick a label.

### The score that was removed

`/stats.global_risk_score` was `100 − round(avg(risks.score) × 4)`, computed
inline from the register alone: no controls, no vulnerabilities, no incidents, no
availability accounting. On tenant A's live register it evaluated to **81** while
the canonical model said **37.3** — and the two ran in opposite directions, since
81 meant "good" on a 100-is-safe scale and 37.3 means "medium risk" on a
100-is-worst one. Both were labelled "score"; the Viewer persona rendered the
first and everything else the second.

Removed rather than deprecated: a competing formula left in a payload is one the
next widget picks up because it was already there.

---

## Asset Statistics

`GET /assets/statistics` — the inventory's shape, in one grouped pass.

| Field | Meaning |
| --- | --- |
| `total` | live assets (soft-deleted excluded) |
| `by_criticality` | the four bands, **always all four present** |
| `by_category` + `uncategorised` | the closed `AssetCategory` vocabulary, plus the rows that have none |
| `by_type` + `untyped` + `types_truncated` + `distinct_types` | free-text labels, capped at 12, with the truncation reported |
| `by_source` | MANUAL / SCANNER / … — "is discovery actually running" |
| `added_in_period` | the one period-scoped field |

`uncategorised` and `untyped` are named counters rather than dropped rows.
Category is empty on everything written before typed attributes shipped and Type
is free text nobody is forced to fill in, so a breakdown that omitted them would
not add up to the total beside it on the same screen — and the reader could not
tell a bug from a business rule.

The four dimensions are grouped together and folded in Go rather than run as four
separate `GROUP BY` queries, so the table we are trying not to load is scanned
once, not four times.

---

## Dashboard Query Strategy

**Option C — a combination**, and deliberately not a single
`/security-command-center` monolith.

| Endpoint | Serves | Why not merged |
| --- | --- | --- |
| `GET /score` | the canonical score, any scope | Merging it would break the shared query key that makes the hero and the sidebar agree. It is used by five surfaces, not one. |
| `GET /stats` | register posture: counters, matrix, trend | The one contract the risk widgets need; already consolidated. |
| `GET /assets/statistics` | inventory shape | A different registry with a different period semantic. |
| `GET /vulnerabilities/stats` | vulnerability posture | Pre-existing, already an aggregate. |
| `GET /analytics/executive` | the consolidated executive view | Pre-existing; the exec persona's whole payload. |

A merged endpoint would have had to recompute the score a third time, or proxy it
— and a third implementation of "what is the score" is exactly what this wave
removed. Each persona makes one or two calls; none makes eight.

---

## Tenant Isolation

The tenant comes from the authenticated context and is **never a request
parameter**, so there is nothing to forge. Proven live against four query-param
spellings and three header spellings; all ignored.

`GET /stats` fails closed: no resolved tenant → `401`, explicitly, rather than
querying every tenant's risks. `ComputeDashboardStats` and the asset statistics
use case both refuse `uuid.Nil` before touching the database, and both are
unit-tested for it.

---

## Time Filtering

One definition, in `internal/domain/timeframe`, pure and taking the caller's
clock — so two widgets fetched a moment apart cannot resolve "last 30 days" to
two different ranges, or at a day boundary to two different days.

```
from INCLUSIVE, to EXCLUSIVE, UTC:  [from, to)
```

Half-open is the only convention under which consecutive windows tile without
overlap and without a gap, so a row belongs to exactly one bucket.

| Parameter | Accepted |
| --- | --- |
| `?period=` | `all` · `7d` · `30d` · `90d` |
| `?from=&to=` | RFC 3339 or `YYYY-MM-DD`, both required together |

* **Default is `all`.** The headline counters are stock quantities; defaulting
  them to a window would put the dashboard permanently at odds with the register
  on first paint, with nothing on screen to say a filter had been applied.
* **A malformed period is a 400 with a reason and no payload**, never a silent
  fallback.
* **Custom ranges are capped at 366 days**, since the series emits one bucket per
  day and an unbounded range is an unbounded response over an unbounded scan.
* **`to` in the future is accepted as asked.** Clamping would rewrite the period
  the response echoes back, which is worse than a range reaching past today over
  rows that do not exist.
* **Granularity** is daily up to 92 days, weekly beyond.
* **An unbounded window counts everything but caps its series** at 366 days, and
  `risk_trend.from` reports the bound actually used — so "all time" never
  silently means "since last year".

Every response echoes `period` and names `period_applies_to`.

---

## Filter Propagation

| Widget | All | 7d | 30d | 90d | Custom | Default | Definition |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Security score | n/a | n/a | n/a | n/a | n/a | — | Point-in-time; the model has no window |
| KPI — total / critical / in treatment / mitigated | — | — | — | — | — | all | **Stock.** Never narrowed |
| Opened in period | ✓ | ✓ | ✓ | ✓ | ✓ | all | **Flow.** `created_at ∈ [from, to)` |
| Risk trend | ✓ | ✓ | ✓ | ✓ | ✓ | all | **Flow**, bucketed; series capped on `all` |
| 5×5 heatmap | — | — | — | — | — | all | Stock |
| Recent activity | n/a | n/a | n/a | n/a | n/a | — | Most recent rows; not a window |
| War Room | n/a | n/a | n/a | n/a | n/a | — | Current open incident |
| Assets — total / by criticality / types | — | — | — | — | — | all | **Stock** |
| Added in period | ✓ | ✓ | ✓ | ✓ | ✓ | all | **Flow** |
| Vulnerability KPIs, exec, audit | n/a | n/a | n/a | n/a | n/a | — | Endpoints take no period yet — see follow-up |

**The control says this out loud**, under itself, because a global period control
on a page of mixed widgets looks like it filters everything:

> *"Filtre la tendance et les risques ouverts. Les compteurs (total, critiques, en
> traitement, atténués) donnent l'état actuel du registre, toutes périodes
> confondues."*

Why the stock is never narrowed: *"how many critical risks do we have"* does not
become a different question when a date range is picked. Filtering it by
`created_at` would answer *"how many did we open"*, print it under a label saying
otherwise, and break the reconciliation this wave exists to hold.

---

## Deep-Link Semantics

`deepLink(destination, options)` builds every dashboard link. Each destination
declares what it understands, once, next to the facet keys the screen registers:

| Destination | Facets | Sorts | Period control |
| --- | --- | --- | --- |
| `/risks` | `criticality`, `status`, `phase`, `category_id`, `source` | `score`, `name`, `criticality`, `status`, `updated_at` | no |
| `/vulnerabilities` | `tier`, `severity`, `status`, `kev` | `priority_score`, `cvss_score`, `cve_id`, `severity` | no |
| `/assets` | `type`, `criticality` | — | no |
| `/incidents`, `/compliance`, `/compliance/gaps` | — | — | no |

**A filter is only appended to a destination that honours it.** Unknown facets
and unsupported sorts are dropped, and the period is withheld from every screen
without a period control — a period in the URL of a screen that ignores it
describes a view the user is not being shown. The dropping is unit-tested, so a
caller's mistake fails in CI rather than shipping as a silently unfiltered list.

Row links use the `?focus=<id>` contract universal search already established, so
a row opens *that* row's drawer rather than the top of a list.

---

## Data Reconciliation

Asserted at three levels.

**In the aggregate itself** (`dashboard_stats_test.go`):

```
Σ risks_by_severity            == total_risks     (the bands partition the register)
Σ risk_matrix[].count          == total_risks     (every risk lands in one cell)
last point cumulative_total    == total_risks     (the line ends at the headline)
Σ points[].opened              == opened_in_period
```

**In the asset aggregate** (`gorm_asset_statistics_test.go`):

```
Σ by_criticality               == total
Σ by_category + uncategorised  == total
Σ by_type + untyped            == total
Σ by_source                    == total
```

**Against the live registers** (`reconcile.py`, 44 assertions, two tenants, all
passing) — each dashboard number compared to what the register reports through
the same API the register screen uses, including the tile's own filter.

---

## Cache Strategy

### Server

Cache keys are tenant-scoped **by construction**. A `KeyFunc` returns
`(string, bool)`; a builder that cannot resolve the tenant returns `false`, and
the contract turns that into *do not cache* rather than *cache under a key
everyone shares*.

```
dashboard:stats:t:<tenant>:period:<p>:from:<f>:to:<t>
dashboard:matrix:t:<tenant>
dashboard:timeline:t:<tenant>:days:<n>
risk:list:t:<tenant>:page:<p>:sev:<s>:status:<st>
risk:search:t:<tenant>:<hash>
risk:id:t:<tenant>:<id>
connector:list / connector:id / marketplace:app   (all t:<tenant>-prefixed)
```

Before this wave none of the nine carried a tenant. Changing the signature
surfaced three more that had been missed — connector and marketplace builders
that failed to typecheck. Nine builders are unit-tested for collision between
tenants, refusal without one, variation with the period, and stability.

**`cache.WrapWithCache` is a passthrough** and says so at the top of its doc
comment. It evaluates nothing and caches nothing, and never has — but the call
sites in `main.go` read exactly like cached routes. It is kept because the keys
are now correct, so whoever implements response caching inherits a safe starting
point.

### Client

TanStack Query, 30 s stale time (the same as `useScore`, so the hero and the
tiles beside it age together). **The period is part of the key** — two windows
over the same register are two different answers.

The tenant is deliberately **not** in the key: it is never a request parameter,
and a client that could name a tenant in a key could name someone else's. What
keeps one organisation's numbers out of another's cache is `lib/sessionScope.ts`,
which drops the whole QueryClient at both ends of a session transition. Putting a
tenant id in the key would look like isolation while providing none.

The replaced `useStats` was `useState` + `useEffect(…, [])`: one fetch per mount,
no key, no invalidation, no refetch, and two mounting components could hold two
different answers.

---

## Loading, Empty, Error and Permission States

`WidgetState` renders five conditions, and **never renders children with a
placeholder substituted in** — there is no third rendering in which a zero stands
in for an answer.

| Condition | Rendering |
| --- | --- |
| loading | skeleton sized to the widget it stands in for |
| error (5xx, timeout, network) | *"Data unavailable — nothing is shown until it has been read"* + **Retry** |
| 401 / 403 | *"Metric not available to you — an administrator can grant it"*, **no** retry |
| empty because of the period | *"Nothing in this period — there is data outside the selected window"* + **widen** |
| empty | first-use copy: what the widget is for, and the action that fills it |

The last distinction is the one people skip and the one that matters most on a
page with a period control: *"no risks were opened in the last 7 days"* and
*"you have no risks"* are opposite facts, and only one of them is a reason to
press a Create button.

---

## API Contracts

### `GET /stats`

| | |
| --- | --- |
| Auth | session (cookie or bearer) |
| Authorization | authenticated member |
| Tenant | from the authenticated context; **not** a parameter |
| Input | `period=all\|7d\|30d\|90d` **or** `from=&to=` |
| Response | `period`, `period_applies_to`, `total_risks`, `high_risks`, `mitigated_risks`, `in_progress_risks`, `quantified_risks`, `risks_by_severity`, `risk_matrix[]`, `opened_in_period`, `risk_trend{from,to,granularity,points[]}`, `generated_at` |
| Errors | `400 invalid_period` (with `message`, no payload) · `401` unresolved tenant · `500` aggregation failure |
| Cache | key carries tenant + period; wrapper is a passthrough |
| Audit | none — a read changes nothing |

### `GET /assets/statistics`

| | |
| --- | --- |
| Authorization | `assets:read` |
| Input | same period parameters |
| Response | `period`, `period_applies_to`, `total`, `by_criticality`, `by_category`, `uncategorised`, `by_type`, `untyped`, `types_truncated`, `distinct_types`, `by_source`, `added_in_period`, `generated_at` |
| Errors | `400 invalid_period` · `401` · `403` · `500` |

### `GET /score?scope=tenant`

Unchanged by this wave; documented here because the dashboard depends on it.
Returns `value`, `band`, `inherent`, `residual`, `mitigation_effectiveness`,
`computed_at`, `formula_version`, `inputs`, `breakdown[]`.

---

## Database Changes

**None.** No migration, no new table, no new index.

Both aggregates run on columns that already exist and are already indexed by
`tenant_id`. A materialised rollup would have been premature at the volumes
measured (2–3 ms, 2–3 queries) and would have added a staleness question the
dashboard does not currently have.

---

## Test Strategy

### Unit — Go

| Suite | Covers |
| --- | --- |
| `internal/domain/timeframe` | presets, half-open bounds, tiling, rejection of unknown/malformed/excessive/contradictory input, granularity, trend capping, unbounded rendering |
| `internal/handler/dashboard_stats_test.go` | the real `ComputeDashboardStats`: Score Engine bands, matrix coverage, stock-vs-flow, cumulative baseline, bucket tiling, capped bounds, empty tenant, tenant scoping, fail-closed |
| `internal/handler/period_http_test.go` | the seam: ten rejections through a real Fiber app, asserted on the **body** |
| `internal/handler/cache_key_test.go` | nine builders × collision / refusal / period variation / stability |
| `internal/infrastructure/repository/gorm_asset_statistics_test.go` | reconciliation invariants, tenant scoping, soft deletes, period bounds, type cap, determinism, fail-closed |

### Unit — frontend

| Suite | Covers |
| --- | --- |
| `period.test.ts` | default, presets, rejection, precedence, key distinctness, URL round trip, foreign-param preservation, labels |
| `deepLinks.test.ts` | facet carriage, multi-value, dropping unknown facets/sorts/periods, `?focus=`, facet keys pinned against the tables |
| `WidgetState.test.tsx` | children never rendered in any of six non-data states; 403 vs 500 distinction; period-empty vs empty |

### Integration / API and E2E

Driven live rather than scripted into the Playwright suite — see the live-proof
record. 44 reconciliation assertions, ten period rejections, seven forged-tenant
probes, five fail-closed probes, a cache-collision probe, plus the browser
journey: login → dashboard → period switch → deep link → back → estate persona →
transport failure → retry → tenant switch.

---

## Reconciliation Test Matrix

| Invariant | Where asserted | Live |
| --- | --- | --- |
| `dashboard.total_risks == register total` | reconcile.py | ✓ |
| `dashboard.critical == register ?criticality=critical` | reconcile.py | ✓ |
| `Σ risks_by_severity == total_risks` | Go unit + reconcile.py | ✓ |
| `Σ risk_matrix == total_risks` | Go unit + reconcile.py | ✓ |
| `last cumulative == total_risks` | Go unit + reconcile.py | ✓ |
| `Σ opened == opened_in_period` | Go unit + reconcile.py | ✓ |
| `dashboard.totalAssets == inventory count` | reconcile.py | ✓ |
| `Σ by_criticality == total` | Go unit + reconcile.py | ✓ |
| `Σ by_category + uncategorised == total` | Go unit + reconcile.py | ✓ |
| `Σ by_type + untyped == total` | Go unit + reconcile.py | ✓ |
| `tenant A ≠ tenant B` on both aggregates | Go unit + reconcile.py | ✓ |

---

## Security Validation

| Check | Result |
| --- | --- |
| Cross-tenant reads on both aggregates | isolated (8/16 risks, 8/13 assets) |
| Forged `tenant_id` / `organization_id` / `org_id` / `tenantId` | ignored |
| Forged `X-Tenant-ID` / `X-Organization-ID` / `X-Org-Id` | ignored |
| Unauthenticated / malformed token | 401, fail closed |
| Unresolved tenant in the aggregate | 401 (not a cross-tenant query) |
| Cache-key collision between tenants | impossible by signature; unit-tested |
| Excessive date range | 400 at 366 days |
| Error messages leaking data | none — rejections carry a reason, no payload |
| Fixture fallback on failure | none — no widget renders a number it did not read |

---

## Accessibility

* Period control: `role="group"` + `aria-labelledby`, `aria-pressed` on each
  preset, so a screen reader can say which period is in force.
* Custom range inputs carry visually-hidden labels naming the convention —
  *"Date de début (incluse)"* / *"Date de fin (exclue)"*.
* The trend chart is `role="img"` whose accessible name **is its content**:
  *"Risques ouverts sur 7 jours : 2 critiques, 3 élevés, 2 moyens. Total 8."*
* KPI tiles carry both the value and the destination:
  *"Critiques: 2 — Registre filtré sur criticité = critique"*.

Not covered: keyboard traversal, a screen-reader pass, an axe run.

---

## Performance

Measured live (mean of 10 warm requests):

| Endpoint | Latency | SQL/request |
| --- | --- | --- |
| `/stats?period=all` | 3.0 ms | 3 |
| `/stats?period=90d` | 2.8 ms | 3 |
| `/assets/statistics` | 2.2–2.3 ms | 2 |
| `/score?scope=tenant` | 6.6 ms | ~17 (pre-existing) |

**No N+1**: neither aggregate's query count grows with rows or with the length of
the period.

Payload, which is where the estate change pays:

```
 8 assets:  /assets 4,028 B   /assets/statistics 391 B   10.3×
13 assets:  /assets 6,749 B   /assets/statistics 383 B   17.6×
```

`/assets` grows with the inventory; the aggregate is constant. A 5 000-asset
estate would have shipped roughly 2.5 MB to render four numbers.

---

## Live Proof

`docs/W0-06_SECURITY_COMMAND_CENTER_LIVE_PROOF.md`.

---

## Known Limitations

1. **Permission-denied rendering is unit-tested, not driven live.** No realistic
   account produces a 403 on these endpoints.
2. **`WrapWithCache` remains a passthrough.** Keys are correct and tested;
   nothing is cached.
3. **Only `/stats` and `/assets/statistics` accept a period.**
   `/vulnerabilities/stats`, `/analytics/executive` and the compliance overview do
   not, so the analyst, exec and audit personas show no period control. That is
   stated by the absence of the control rather than papered over with one that
   would not work.
4. **The trend's band split uses each risk's *current* criticality**, not the band
   it carried on the day it was opened. The register does not version criticality
   per day, so a per-day band would have to be reconstructed and would be wrong
   for every risk created before `risk_histories` existed. The contract says
   "current band" and the UI repeats it.
5. **`/score` costs ~17 queries.** Pre-existing, and now the heaviest thing on the
   page.
6. **No axe run, no keyboard traversal, no screen-reader pass.**
7. **`src/__tests__/App.integration.test.tsx` fails**, on this branch and its
   parent alike (verified by stashing). Pre-existing, unrelated, left alone so a
   future regression there stays visible.
8. **The MFA challenge path does not refresh `business_role`** — pre-existing, and
   it complicated the persona capture in the live proof.

---

## Follow-up Work

* Extend the period to `/vulnerabilities/stats`, `/analytics/executive` and the
  compliance overview, then give the analyst/exec/audit personas the control.
* Implement `WrapWithCache` for real; the `KeyFunc` gate makes the tenant
  impossible to omit.
* Reduce `/score`'s query count.
* Add the two contracts to `docs/openapi.yaml` and regenerate the client types —
  the frontend types are currently hand-written (strictly typed, zero `any`, but
  not contract-first).
* Fold the reconciliation harness into the Playwright suite so it runs in CI
  rather than by hand.
