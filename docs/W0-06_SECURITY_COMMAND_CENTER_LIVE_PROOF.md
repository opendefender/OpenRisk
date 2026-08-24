# W0-06 — Security Command Center Live Proof

Everything below was executed against a running stack on 2026-08-24. Nothing in
this record is inferred from reading code; where a claim is *not* backed by a
live run it says so explicitly, under **Not proven live**.

## Environment

| | |
| --- | --- |
| Date | 2026-08-24 |
| Backend | Go 1.25.12, built from this branch, `:8091` |
| Frontend | Vite 7.3.6 dev server, `:5176` (`BACKEND_URL=http://localhost:8091`, `/api` proxied so the browser sees one origin) |
| Database | PostgreSQL 16 on `:5434` (`openrisk`) |
| Cache / bus | Redis on `:6379` |
| Browser | Chromium via Playwright, 1500×1000 |
| Branch | `feat/w0-06-security-command-center-contracts` |
| Commit | `14f7b09` |

Two start-up notes, both pre-existing and unrelated to this wave:

* `DATABASE_URL` in `.env` carries no `sslmode`, so golang-migrate aborts with
  *"SSL is not enabled on the server"* and the process exits. Started with
  `?sslmode=disable` appended.
* `MIGRATIONS_DIR` defaults to `migrations` relative to the process working
  directory, while the migrations live at the repository root. Started with an
  absolute `MIGRATIONS_DIR`.

## Test Tenants

Created by this proof run, through the product's own registration endpoint. No
production data was used at any point.

| | Tenant A | Tenant B |
| --- | --- | --- |
| Organisation | W006 Tenant A | W006 Tenant B |
| Org id | `65621b77-…edda8` | `32d7ea77-…4cc88` |
| User | `w006-a@example.test` | `w006-b@example.test` |

Both accounts enrolled MFA on first login (the role mandates it), via
`/auth/mfa/setup` → `/auth/mfa/verify` — the same flow `tests/e2e/support/auth.ts`
uses. TOTP codes were generated with a stdlib RFC 6238 implementation.

## Test Data

Seeded through the public API, sized so that **every count differs between the
two tenants** — an isolation check that passes by accident is worthless.

| | Tenant A | Tenant B |
| --- | --- | --- |
| Risks | 8 (2 critical · 3 high · 2 medium · 1 low) | 16 (4 · 6 · 4 · 2) |
| Assets | 8 | 13 |
| — categorised | 4 (`server`) | 8 (`server`) |
| — uncategorised | 4 | 5 |
| — untyped | 1 | 1 |

Risk scores are `probability × impact`, chosen to land one or more risks in each
Score Engine band (≥7 critical, ≥4 high, ≥2 medium, <2 low).

**Honest note on the asset counts.** The seed script attempted 15 assets for
tenant B; two `workstation` rows were rejected with
`unknown attribute(s) for this asset category: environment` — that category's
schema has no `environment` attribute. The reconciliation compares the dashboard
against what actually exists (13), so the discrepancy weakens nothing; it is
recorded because the seed script's own counter says 15.

## Security Score

`GET /score?scope=tenant`, tenant A, live:

```
value=37.3  band=medium  inherent=37.3  residual=37.3
formula_version=2.1  computed_at=2026-08-24T08:10:44Z

factor                     weight     raw   contribution
risk_exposure               0.533   70.00          37.30
control_gaps                0.000    0.00           0.00   <- unavailable
incident_pressure           0.200    0.00           0.00
vulnerability_pressure      0.267    0.00           0.00
                                            ------------
                                                   37.30  == value
weights sum: 1.0
inputs: {"total_risks":8,"critical_risks":2,"high_risks":3,
         "applicable_controls":0,"implemented_controls":0,
         "critical_vulnerabilities":0,"kev_vulnerabilities":0,
         "open_incidents":0,"critical_open_incidents":0}
```

Three things this shows, none of which can be established by reading the code:

1. **The contributions sum to the value.** `37.30 == 37.3`, checkable by eye.
2. **`control_gaps` is flagged unavailable and its weight redistributed** — the
   tenant has imported no framework, so the factor is excluded rather than scored
   zero. The remaining three weights renormalise to exactly `1.0`. Scoring an
   unreadable source as zero on a scale where 100 is worst would read as
   *excellent*, which is the most dangerous failure a security score has.
3. **The scale runs 0–100 where 100 is WORST.** Bands: `<25` low, `≥25` medium,
   `≥50` high, `≥75` critical. 37.3 is therefore "medium", and the UI renders it
   in the medium tone.

### The score that was removed

`/stats.global_risk_score` was `100 − round(avg(score) × 4)`, clamped at 0. On
tenant A's register that is:

```
avg(score) = (2×8.1 + 3×4.8 + 2×3.0 + 1×0.5) / 8 = 4.6375
global_risk_score = 100 − round(4.6375 × 4) = 100 − 19 = 81
```

So the same tenant, at the same instant, had **81 under one field and 37.3 under
the other** — and the two ran in *opposite directions*: 81 meant "good" on a
100-is-safe scale, 37.3 means "medium risk" on a 100-is-worst scale. Both were
labelled "score". The Viewer persona rendered the first; every other surface
rendered the second. The field is now removed.

## Score Provenance

`GET /score/model` serves the model's self-description (scale, bands, weights),
so no client hardcodes a threshold. On screen, the gauge takes a `Score` object —
value *and* band together — and `ScoreGauge` has no prop through which a caller
could supply a bare number and let the component pick a label.

**A direction bug fixed on the way.** The Viewer persona used `ScoreHero`, whose
colour rule is `pct >= 0.7 ? low : pct >= 0.45 ? high : critical` — green for a
high number. Correct for a 100-is-safe score, exactly inverted for this model:
a tenant at 90 (critical risk) would have been painted green. Moving that persona
onto `ScoreGauge`, which colours from the server's band, fixes it.

## Asset Statistics

`GET /assets/statistics`, tenant A, live:

```json
{ "period": { "preset": "all", "to": "2026-08-25T00:00:00Z" },
  "period_applies_to": ["added_in_period"],
  "total": 8,
  "by_criticality": { "CRITICAL": 2, "HIGH": 2, "MEDIUM": 0, "LOW": 4 },
  "by_category": { "server": 4 }, "uncategorised": 4,
  "by_type": { "Server": 4, "Database": 2, "Laptop": 1 }, "untyped": 1,
  "types_truncated": 0, "distinct_types": 3,
  "by_source": { "MANUAL": 8 },
  "added_in_period": 8, "generated_at": "2026-08-24T08:19:20Z" }
```

Every breakdown adds up to the total beside it: `2+2+0+4 = 8`, `4+4 = 8`,
`(4+2+1)+1 = 8`, `8 = 8`.

## Dashboard Reconciliation

A harness (`reconcile.py`) compares every headline number against the count the
underlying register reports for the same tenant, **through the same API the
register screen uses**. 44 checks, both tenants, all passing:

```
=== tenant A ===                              === tenant B ===
[PASS] total risks           8 == 8           [PASS] total risks          16 == 16
[PASS] critical risks        2 == 2           [PASS] critical risks        4 == 4
[PASS] high risks            3 == 3           [PASS] high risks            6 == 6
[PASS] medium risks          2 == 2           [PASS] medium risks          4 == 4
[PASS] low risks             1 == 1           [PASS] low risks             2 == 2
[PASS] mitigated risks       0 == 0           [PASS] mitigated risks       0 == 0
[PASS] matrix covers the register             [PASS] matrix covers the register
[PASS] bands partition the register           [PASS] bands partition the register
[PASS] trend ends at the headline total       [PASS] trend ends at the headline total
[PASS] Σ opened == opened_in_period           [PASS] Σ opened == opened_in_period
[PASS] total assets          8 == 8           [PASS] total assets         13 == 13
[PASS] assets critical       2 == 2           [PASS] assets critical       4 == 4
[PASS] assets high           2 == 2           [PASS] assets high           4 == 4
[PASS] assets medium         0 == 0           [PASS] assets medium         0 == 0
[PASS] assets low            4 == 4           [PASS] assets low            5 == 5
[PASS] Σ by_criticality == total              [PASS] Σ by_criticality == total
[PASS] Σ by_category + uncategorised == total [PASS] Σ by_category + uncat == total
[PASS] Σ by_type + untyped == total           [PASS] Σ by_type + untyped == total
[PASS] Σ by_source == total                   [PASS] Σ by_source == total
[PASS] uncategorised         4 == 4           [PASS] uncategorised         5 == 5
[PASS] untyped               1 == 1           [PASS] untyped               1 == 1

=== tenant isolation ===
[PASS] risk totals differ: A=8 B=16
[PASS] asset totals differ: A=8 B=13

44/44 checks passed
```

The critical-risk check is the deep link's own promise checked end to end: the
dashboard tile's number is compared against `GET /risks?criticality=critical`,
which is exactly what the tile's URL asks the register for.

## Time Filtering

`GET /stats`, tenant A, across every supported window:

```
period  total  critical  opened  points  granularity  from
all         8         2       8      54  week         (unbounded)
7d          8         2       8       7  day          2026-08-18T00:00:00Z
30d         8         2       8      30  day          2026-07-26T00:00:00Z
90d         8         2       8      90  day          2026-05-27T00:00:00Z
custom      8         2       8      31  day          2026-08-01 → 2026-09-01
```

**The stock is identical under every window; only the flow moves.** A January
window over the same register returns `total_risks: 8` and `opened_in_period: 0`
— the register did not shrink because a date range was picked.

Note the `all` row: 54 weekly points, not 8 years of daily ones. An unbounded
window counts everything but caps its *series*, and the response reports the
bounds it actually used (`risk_trend.from = 2025-08-24`) rather than the ones
requested — so "all time" never silently means "since last year".

### Validation — rejected, never defaulted

Every malformed period returns **400 with a reason and no payload**:

```
period=6m                                 400  unsupported period "6m": expected one of all, 7d, 30d, 90d…
period=yesterday                          400  unsupported period "yesterday": …
period=custom                             400  period=custom is not a value: send from and to instead
from=not-a-date&to=2026-09-01             400  invalid from: "not-a-date" is not RFC 3339 or YYYY-MM-DD
from=2026-08-01&to=31/12/2026             400  invalid to: "31/12/2026" is not RFC 3339 or YYYY-MM-DD
from=2026-09-01&to=2026-08-01             400  to must be after from (the range is half-open: [from, to))
from=2026-08-01&to=2026-08-01             400  to must be after from …
to=2026-09-01                             400  from is required when to is supplied
from=2020-01-01&to=2026-01-01             400  range too long: 2192 days requested, maximum is 366
period=30d&from=2026-08-01&to=2026-09-01  400  period and from/to are mutually exclusive: send one or the other
from=2025-08-24&to=2026-08-25             200  (exactly at the 366-day cap)
period=30d                                200
```

**A bug found by this run.** The first version of these ten returned status 400
with a *full stats payload in the body* — `total_risks: 8`, the severity
breakdown, the whole heatmap — computed against a zero window. `parsePeriod`
returned the response rather than an error, and Fiber's `JSON()` returns `nil` on
success, so the caller's `if errResp != nil` never fired. Any client rendering
`data` without checking the status would have shown a complete, plausible
dashboard for a period the server had just rejected. Fixed in `3aee160`;
`period_http_test.go` now drives a real Fiber app and asserts on the response
**body** of all ten rejections, not just the status.

## Deep Links

Driven in the browser. The tile's number and the destination's row count match:

| From | Click | URL | Result |
| --- | --- | --- | --- |
| Posture dashboard | "Critiques — 2" | `/risks?f.criticality=critical&sort=score%3Adesc` | 2 rows: *Critical exposure 1*, *Critical exposure 2* |
| Estate dashboard | "Critiques — 2" | `/assets?f.criticality=critical` | 2 rows: *srv-crit-1*, *srv-crit-2* |

Note what the asset link does **not** carry: no `period`. `/assets` has no period
control, and appending one would describe a view the user is not being shown.

### Filter state survives the round trip

```
start   /?period=30d          "30 jours" pressed · "8 ouvert(s) sur la période (30 jours)"
click   → /risks?f.criticality=critical&sort=score:desc
back    → /?period=30d        "30 jours" pressed · "8 ouvert(s) sur la période (30 jours)"
```

And a pasted URL deserialises: loading `/?from=2026-01-01&to=2026-02-01` cold
renders with "Dates" pressed and the range echoed as
`0 ouvert(s) sur la période (2026-01-01 → 2026-02-01)`.

## Period Propagation, on the wire

Network log while the estate persona rendered at `?period=30d`:

```
GET /api/v1/score?scope=tenant                => 200
GET /api/v1/assets                            => 200   (the row list — needs rows, not counts)
GET /api/v1/assets/statistics?period=30d      => 200   (the counters)
```

The period reaches the server verbatim; there is no second representation to
disagree with the URL.

The control states its own scope on screen, in words:

> *"Filtre la tendance et les risques ouverts. Les compteurs (total, critiques, en
> traitement, atténués) donnent l'état actuel du registre, toutes périodes
> confondues."*

and on the estate persona:

> *"Filtre uniquement « ajoutés sur la période ». L'inventaire et la répartition
> par criticité donnent l'état actuel du parc."*

Switching `all → 7d` in the browser changed the trend caption from
*"Risques ouverts par semaine"* to *"par jour"* and the flow line to
*"(7 jours)"*, while the four stock tiles stayed at 8 / 2 / 0 / 0.

## Tenant Switching

Signed out of A through the account menu, signed in as B, same tab (the SPA is
never torn down — this is the path W0-05 found the cross-tenant cache leak on):

| | Tenant A | Tenant B after switch |
| --- | --- | --- |
| Heading | Bonjour, Ada | Bonjour, Bo |
| Risques totaux | 8 | **16** |
| Critiques | 2 | **4** |
| Trend chart (accessible name) | *2 critiques, 3 élevés, 2 moyens. Total 8.* | ***4 critiques, 6 élevés, 4 moyens. Total 16.*** |
| Footer | 8 ouvert(s) · 8 au registre | **16 ouvert(s) · 16 au registre** |
| Residue of A's numbers | — | **none** |

## Security Validation

### Cross-tenant reads

```
A GET /stats               total_risks 8      B GET /stats               total_risks 16
A GET /assets/statistics   total 8            B GET /assets/statistics   total 13
```

### Forged tenant identifiers — all ignored

Tenant A's session, asking for tenant B's id every way the codebase spells it:

```
?tenant_id=<B>        total_risks 8   (16 would be a leak)
?organization_id=<B>  total_risks 8
?org_id=<B>           total_risks 8
?tenantId=<B>         total_risks 8
X-Tenant-ID: <B>      total_risks 8
X-Organization-ID: <B> total_risks 8
X-Org-Id: <B>         total_risks 8
/assets/statistics ?tenant_id=<B>        total 8   (13 would be a leak)
/assets/statistics ?organization_id=<B>  total 8
```

The tenant is taken from the authenticated context and is not a request
parameter, so there is nothing to forge.

### Fail closed

```
no token    GET /stats               401
no token    GET /assets/statistics   401
no token    GET /score?scope=tenant  401
bad token   GET /stats               401
bad token   GET /assets/statistics   401
```

### Cache-collision probe

A and B interleaved five times; each always received its own totals:

```
round 1: A=8 B=16 OK    round 2: A=8 B=16 OK    round 3: A=8 B=16 OK
round 4: A=8 B=16 OK    round 5: A=8 B=16 OK
```

The dashboard/register cache keys are now tenant-scoped by construction — a
`KeyFunc` returning `(string, bool)` cannot produce a key without a resolved
tenant. Nine builders are unit-tested for collision, refusal and stability. Note
that `cache.WrapWithCache` is still a passthrough that never evaluates a key, so
this probe demonstrates the absence of a leak rather than the presence of a
working cache — see *Known limitations*.

## Error States

The two Command Center aggregates were failed at the transport, leaving auth and
every other endpoint working — the widget-level outage, not a whole-app one:

```
error states rendered : 2
retry buttons         : 2
KPI numbers rendered  : 0        <- the assertion that matters
score still rendered  : yes      <- a different endpoint; not collateral damage
critical-asset list   : still rendered (fed by /assets, which was untouched)
```

The copy: *"Données indisponibles — Impossible de lire cet indicateur. Aucune
valeur n'est affichée tant qu'elle n'a pas été lue — réessayez, ou contactez un
administrateur si cela persiste."*

Pressing the widget's own **Réessayer** after restoring the transport recovered
it completely: 0 error states, counters back, `8 ajouté(s) sur la période (90
jours)`. The retry is a real handler, not a decoration.

Screenshot: `docs/evidence/w0-06/w006-error-state.png`.

## Empty States

Selecting January 2026 on a register first populated in August:

```
Tendance des risques · 2026-01-01 → 2026-02-01
Rien sur cette période
Il existe des données en dehors de la fenêtre sélectionnée. Élargissez la période pour les voir.
[ Voir tout l'historique ]
```

Pressing *Voir tout l'historique* returned the URL to `/` (the unbounded default)
and the series came back.

**A second bug found by this run.** Before `14f7b09`, that same screen said
*"Pas encore de tendance — la courbe se construit à mesure que des risques sont
ouverts"* — telling a tenant with eight risks that they had none. The widget
decided between its two empty states using the series' last `cumulative_total`,
which counts risks created *before* each bucket and is therefore zero for every
window in the past. It now reads `total_risks` from the same payload. The unit
tests could not have caught this: they assert the flag's behaviour given its
inputs, and the flag was computed from the wrong input.

## Permission-Denied States

`WidgetState` renders 401/403 as *"Indicateur non accessible — votre rôle ne donne
pas accès à ces données. Un administrateur de l'organisation peut vous les
ouvrir"*, with **no** retry button, distinctly from a 5xx.

**Not proven live.** Both Command Center aggregates are gated on `risks:read` /
`assets:read`, which every business-role preset that reaches a dashboard already
holds, so no realistic account produces a 403 here. The rendering is covered by
`WidgetState.test.tsx` (401 and 403 both asserted to render the permission state,
to omit the retry, and never to render the payload). What *is* proven live is
that both endpoints fail closed with 401 when unauthenticated.

## Accessibility

Read off the live DOM:

* The period control is a `role="group"` with an `aria-labelledby` naming it
  *"Période du tableau de bord"*, and each preset carries `aria-pressed`. Exactly
  one was `true` in every state tested (`Tout`, `7 jours`, `30 jours`, `Dates`),
  so a screen reader can say *which* period is in force, not merely that four
  buttons exist.
* The custom-range inputs have visually-hidden labels naming the convention:
  *"Date de début (incluse)"* / *"Date de fin (exclue)"*.
* The trend chart is `role="img"` with its content as the accessible name, not a
  generic label:
  *"Risques ouverts sur 7 jours : 2 critiques, 3 élevés, 2 moyens. Total 8."*
  It tracks the selected period — at tenant B it read *"…4 critiques, 6 élevés,
  4 moyens. Total 16."*
* Every KPI tile has an `aria-label` carrying both the number and the destination:
  *"Critiques: 2 — Registre filtré sur criticité = critique"*.

**Not proven live**: keyboard traversal and a real screen-reader pass. No axe run
in this session.

## Performance

Warm, mean of 10 requests, all HTTP 200:

```
/stats?period=all        (A,  8 risks)     3.0 ms
/stats?period=90d        (A,  8 risks)     2.8 ms
/stats?period=all        (B, 16 risks)     2.9 ms
/assets/statistics       (A,  8 assets)    2.3 ms
/assets/statistics       (B, 13 assets)    2.2 ms
/score?scope=tenant      (A)               6.6 ms
/assets full collection  (A)               2.4 ms
```

SQL statements per request (mean of 5; ~0.4 is shared per-request overhead):

```
/stats?period=all         3.4     counters + matrix + trend
/stats?period=90d         3.4     identical — the query count does not grow with the window
/assets/statistics        2.4     one grouped pass + one period count
/assets (collection)      2.4     rows + the Risks preload
/score?scope=tenant      17.4     pre-existing; composes four sources
```

**No N+1 in the new code**: neither aggregate's query count grows with the number
of rows or with the length of the period.

The payload is where the estate change pays:

```
tenant A ( 8 assets):  /assets 4,028 bytes   /assets/statistics 391 bytes   10.3x
tenant B (13 assets):  /assets 6,749 bytes   /assets/statistics 383 bytes   17.6x
```

`/assets` grows with the inventory; `/assets/statistics` is constant. Extrapolating
tenant B's ~519 bytes per asset, a 5 000-asset estate would have shipped ~2.5 MB
to the browser to render four numbers. It now ships ~390 bytes.

`/score` at ~17 queries is the heaviest thing on the page. It is pre-existing and
outside this wave's scope, but it is the obvious next candidate — see the
follow-up section of the main document.

## Commands Executed

```bash
# stack
go build -o <scratch>/openrisk-w006 ./cmd/server
DATABASE_URL='postgres://…@localhost:5434/openrisk?sslmode=disable' \
  MIGRATIONS_DIR="$PWD/migrations" PORT=8091 <scratch>/openrisk-w006
BACKEND_URL=http://localhost:8091 npx vite --port 5176 --strictPort

# tenants (registration → MFA enrolment → session)
POST /auth/register  ×2
POST /auth/login → POST /auth/mfa/setup → POST /auth/mfa/verify   ×2

# data
POST /risks   ×24     POST /assets  ×21 (2 rejected, see Test Data)

# proof
python3 reconcile.py            # 44 checks
curl /stats?period={all,7d,30d,90d}         and 10 malformed variants
curl /assets/statistics{,?period=…}
curl with forged tenant_id / organization_id / org_id / tenantId
curl with X-Tenant-ID / X-Organization-ID / X-Org-Id
curl with no token and with a malformed token
Playwright: login → dashboard → period switch → deep link → back →
            estate persona → transport failure → retry → tenant switch
```

## Results

| Check | Result |
| --- | --- |
| Dashboard ↔ registry reconciliation (44 assertions, 2 tenants) | PASS |
| Security score provenance sums to the displayed value | PASS |
| Unavailable factor excluded and weights renormalised | PASS |
| Asset statistics breakdowns add up to the total | PASS |
| Period: stock invariant, flow varies | PASS |
| Period: 10 malformed inputs rejected 400, no payload | PASS |
| Period: URL round trip, reload, back button | PASS |
| Deep links: tile count == destination row count | PASS |
| Deep links: period withheld from destinations that lack the control | PASS |
| Tenant isolation on both aggregates | PASS |
| Forged tenant id (4 query params, 3 headers) ignored | PASS |
| Unauthenticated / malformed token fail closed | PASS |
| Cache-collision probe (interleaved A/B ×5) | PASS |
| API failure → error state, zero fabricated numbers | PASS |
| Retry recovers | PASS |
| Empty-period state distinguished from empty-register | PASS |
| Tenant switch leaves no residue | PASS |
| Permission-denied rendering | UNIT TEST ONLY (see above) |
| Accessibility: names, roles, aria-pressed, chart alt text | PASS (inspected) |
| Accessibility: keyboard traversal, screen-reader pass, axe | NOT RUN |
| Performance: latency, query counts, payload size | PASS (measured) |

## Screenshots / Evidence

| File | What it shows |
| --- | --- |
| `docs/evidence/w0-06/w006-dashboard-all.png` | Posture dashboard, tenant A, unbounded window |
| `docs/evidence/w0-06/w006-deeplink-critical.png` | `/risks?f.criticality=critical` — the 2 rows the tile counted |
| `docs/evidence/w0-06/w006-estate-persona.png` | Estate persona on `/assets/statistics`, incl. "4 sans catégorie" |
| `docs/evidence/w0-06/w006-error-state.png` | Aggregates failing: error states and retries, no numbers |
| `docs/evidence/w0-06/w006-tenant-b.png` | Tenant B after an in-tab switch — 16 risks, no residue |

## Known Limitations

1. **Permission-denied is unit-tested, not driven live.** No realistic account
   produces a 403 on these two endpoints.
2. **`WrapWithCache` remains a passthrough.** The cache keys are correct and
   tested, but nothing is cached, so the collision probe proves the absence of a
   leak rather than a working cache. Implementing response caching is follow-up
   work, and the `KeyFunc` contract makes the tenant impossible to omit when it
   happens.
3. **The MFA challenge path does not refresh `business_role`.** After changing a
   member's business role, a login through `/auth/mfa/challenge` returned a token
   pair whose profile carried an empty `business_role`, so the persona did not
   change. Pre-existing and unrelated to this wave; for the estate-persona
   capture the role was set in the cached profile, while all data on screen came
   from the live API under the real session.
4. **No axe run, no keyboard traversal pass, no screen-reader pass.**
5. **Small dataset.** 8 and 16 risks, 8 and 13 assets. Enough to prove every
   invariant and every isolation property, not enough to characterise behaviour at
   thousands of rows; the payload extrapolation above is arithmetic, not a
   measurement.
6. **`src/__tests__/App.integration.test.tsx` fails**, on this branch and on its
   parent. Verified pre-existing by stashing this wave's changes and re-running.
