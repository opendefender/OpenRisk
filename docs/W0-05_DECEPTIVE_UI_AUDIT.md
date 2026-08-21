# W0-05 — Deceptive UI Audit

## Executive Summary

OpenRisk had already been through two cleanups of this kind. The `dead-controls`
sweep (2026-08-07) removed thirteen inert controls, and the bank-grade hardening
wave replaced the leaderboard's seven invented colleagues, the dashboard's
pre-populated risk matrix, the bell's four fictional incidents and the
simulations page's 8.4/10 gauge with honest states. A custom ESLint rule,
`openrisk/no-mock-data`, was added to stop fabricated data returning by name.

So this wave did not find a product full of fake screens. It found the residue
the previous sweeps could not see, because none of it is named like a fixture and
none of it is a button without a handler:

* **Settings › Integrations listed six providers from a literal, three of them
  shown as connected**, on every tenant, with toggles that persisted nothing —
  while four *real* integration surfaces exist elsewhere in the product.
* **Settings › Notifications wrote eight preferences to `localStorage`** while a
  real per-user, per-tenant preferences API sat unused, and while no producer in
  the backend consulted a preference before sending anything.
* **Two "policy" toggles claimed to control server-side enforcement that has no
  configuration knob at all.**
* **The War Room's chat composer accepted coordination notes and delivered them
  nowhere**, in the one screen where a message that silently fails is most
  costly.
* **`/risks/:id/timeline` crashed** on any risk that has history, and rendered an
  API failure as "no changes recorded".
* **Nothing cleared the client cache at logout**, so the next person to sign in
  on the same browser was shown the previous tenant's data.
* **`/governance/audit-trail` opened a different screen**, and refused a
  non-admin by rendering another tab's empty state.

That last one is the reason the wave matters. Every other finding invents data
that belongs to nobody. This one displays *another organisation's real data* to
someone who is not entitled to it, and it does so on every screen at once.

Fourteen surfaces were classified `DECEPTIVE`, and all fourteen are resolved.
Six became `REAL` by being wired to backends that already existed and were
simply never connected; the rest became honest `UNAVAILABLE` states or were
removed.

Two of them were found only by driving the product against a running server,
which is worth naming because no amount of reading would have caught either:
the notification-preferences endpoint answered `{"DisableAllNotifications":…}`
while accepting `{"disable_all_notifications":…}`, so no client could read a
preference and write it back; and `/governance/audit-trail` opened the
*Approvals* tab, showing a non-admin "nothing to approve" where the honest
answer was "you may not read this".

---

## Product Rule

> Every capability visible in OpenRisk is either end-to-end real, explicitly
> experimental, or explicitly unavailable.

Nothing may be presented as real when it is not. Specifically forbidden in any
production route:

| Forbidden | Because |
| --- | --- |
| A KPI computed from a fixture or a literal | The number is the product. A wrong number is worse than no number. |
| An integration shown as connected without a configuration behind it | The user stops watching a channel they believe is being watched. |
| A chart with a silent fixture fallback on API error | An outage renders as a healthy posture. |
| A primary action with no handler, a `TODO`, or a `setTimeout` success | Teaches the user that the screen does not mean what it says. |
| A "success" toast raised before the API confirms | Same, with data loss attached. |
| A route that renders an empty list because it is unimplemented | Indistinguishable from "you have no data". |
| A counter hardcoded to preserve a layout | The layout is not worth the lie. |

The permitted honest states are `EXPERIMENTAL` and `UNAVAILABLE`, each of which
must say what it is and offer a next action.

**In a security tool, a control that lies is a trust vulnerability.** An operator
calibrates judgement on what the interface shows. A permanently green light, a
Slack toggle that routes nothing, or a timeline that reports "no changes" when it
actually failed to load, all teach the same lesson: do not believe this screen —
including when the screen is telling the truth.

---

## Scope

`frontend/src` (all reachable modules), plus the backend paths that had to change
for a surface to become real.

Reachability was computed rather than assumed: the import graph is walked from
`src/main.tsx`, and only modules on that graph can be rendered by a user. **277
modules are reachable; 110 are orphaned** — older duplicate pages
(`pages/Marketplace.tsx`, `pages/AnalyticsDashboard.tsx`,
`pages/RealTimeAnalyticsDashboard.tsx`, `features/settings/IntegrationsTab.tsx`,
`api/scoreEngineService.ts` and others) that no route reaches. The orphans are
where most of the remaining fabricated literals and `catch → return fixture`
fallbacks live. They are **not** user-facing and are therefore out of this wave's
disposition matrix, but they are recorded under
[Fixture Leakage](#fixture-leakage) because they are a reservoir: the next
developer who greps for "how do we render an integrations grid" finds the fake
one first.

The audit script lives at `frontend/scripts/audit-surfaces.mjs` and is wired into
the automated checks so the reachable/orphan split is recomputed rather than
remembered.

---

## Route Inventory

Every route declared in `frontend/src/App.tsx`, with the source that fills it.

### Public

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/login`, `/register` | Authentication | `POST /auth/login`, `/auth/register` | REAL |
| `/forgot-password`, `/reset-password` | Password reset | `POST /auth/password/*` | REAL |
| `/status` | Public health | `GET /api/v1/status` (raw fetch, no session by design) | REAL |
| `/invitations/accept` | Invitation acceptance | `GET/POST /invitations/*` | REAL |
| `/maintenance`, `*` | System pages | static by design | REAL |

### Onboarding (authenticated, outside the shell)

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/onboarding/{organization,profile,goal,framework,team}` | Signup wizard | `GET/PUT /onboarding/*` | REAL |

### Risks

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/` | Dashboard (5 role personas) | `/stats`, `/risks`, `/incidents`, `/analytics/executive` | REAL |
| `/risks` | Risk register | `GET /risks` (server sort/page/facets) | REAL |
| `/risks/import` | CSV import | `POST /risks/import` | REAL |
| `/risks/unmapped` | Mapping backlog | `GET /risks/unmapped` | REAL |
| `/risks/weighting` | Smart-score weights | `GET/PUT /risk-scoring/weights` | REAL |
| `/risks/:riskId/timeline` | Risk history | hand-rolled `fetch`, wrong envelope, no cookie creds | **DECEPTIVE → fixed (D8)** |
| `/risks/mitigations` | Mitigation board | `GET /mitigations` | REAL |
| `/risks/mitigations/:id` | Mitigation detail | `GET /mitigations/:id` | REAL |

### Threats

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/vulnerabilities` | Vulnerability register | `GET /vulnerabilities` | REAL |
| `/vulnerabilities/unassigned` | Unattributed findings | `GET /vulnerabilities?unassigned` | REAL |
| `/vulnerabilities/risk-rule` | Vuln→risk rule + drafts | `GET/PUT /vulnerabilities/risk-rule` | REAL |
| `/threat-map` | Threat intel | `GET /cti/*` | REAL |
| `/ai/emerging-risks` | Emerging risks | `POST /ai/emerging-risks` | REAL |
| `/simulations` | Risk digital twin | none — no engine exists | EXPERIMENTAL (already honest) |

### Incidents

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/incidents` | Incident register | `GET /incidents` | REAL |
| `/incidents/sources` | Origin help | `GET /incidents/origins` (real per-tenant counts) | REAL |
| `/incidents/:id/war-room` | Incident console | timeline/close real; **chat fabricated**, task board declared unavailable | **DECEPTIVE → fixed (D6, D7)** |

### Compliance

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/compliance` | Framework grid | `GET /compliance/frameworks` | REAL |
| `/compliance/gaps` | Gap analysis | `GET /compliance/gap-analysis` | REAL |
| `/compliance/evidence`, `/evidence/missing` | Evidence library | `GET /compliance/evidence*` | REAL |
| `/compliance/audits`, `/audits/:id` | Audits | `GET /compliance/audits*` | REAL |
| `/compliance/remediation`, `/remediation/:id` | Remediation plans | `GET /compliance/remediations*` | REAL |
| `/compliance/frameworks/:id`, `/:id/gaps` | Framework detail | `GET /compliance/frameworks/:id/controls` | REAL |

### Assets

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/assets` | Inventory | `GET /assets` | REAL |
| `/assets/schemas` | Asset schemas | `GET/PUT /asset-schemas` | REAL |
| `/assets/topology` | Dependency graph | `GET /assets`, `GET /asset-dependencies` | REAL |
| `/infrastructure` | Scanner configs/agents | `GET /scanner/*` | REAL |
| `/infrastructure/scans/:jobId` | Scan preview | `GET /scanner/jobs/:id/preview` | REAL |

### Analytics / reports

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/score` | Security score | `GET /score` | REAL |
| `/analytics/financial` | FAIR quantification | `GET /analytics/financial` | REAL |
| `/leaderboard` | Risk champions | none — no ranking backend | EXPERIMENTAL (already honest) |
| `/reports` | Report templates | `POST /reports/*`; **scheduled-reports peek fabricated** | **DECEPTIVE → fixed (D5)** |
| `/reports/library`, `/reports/jobs/:id`, `/reports/board` | Report engine | `GET /reports/*` | REAL |
| `/recommendations` | AI advisor | `POST /ai/assistant/query` | REAL |

### Automation / governance / settings

| Route | Capability | Source | Status |
| --- | --- | --- | --- |
| `/automation` | SOAR rules, SLA, channels | `GET /automation/*` | REAL |
| `/governance` | Approvals, delegations | `GET /governance/*` | REAL |
| `/governance/audit-trail` | Audit trail | opened the **Approvals** tab; refused non-admins with an empty state | **DECEPTIVE → fixed (D13, D14)** |
| `/settings` (general) | Org profile | `GET /organization` + **2 fake policy toggles** | **DECEPTIVE → fixed (D3)** |
| `/settings/members` | Members & roles | `GET /rbac/members`, `/invitations` | REAL |
| `?tab=tokens` | API tokens | `GET /auth/pat` | REAL |
| `?tab=orgs` | Organisations | `GET /rbac/tenants` | REAL |
| `?tab=fields` | Custom fields | `GET /custom-fields`; **inert "New field" CTA** | **DECEPTIVE → fixed (D4)** |
| `?tab=integrations` | Integrations | **hardcoded literal, 3 shown connected** | **DECEPTIVE → fixed (D1)** |
| `?tab=notif` | Notifications | **localStorage only** | **DECEPTIVE → fixed (D2)** |
| `?tab=security` | Auth policy | descriptive text + real sessions panel | REAL / UNAVAILABLE (documented) |
| `?tab=billing` | Plan & entitlements | `GET /billing`, `/entitlements` | REAL |
| `?tab=danger` | Org deletion | `DELETE /rbac/tenants/:id` | REAL |

**No route path matches `/demo`, `/test`, `/mock`, `/placeholder`, `/coming-soon`
or `/experimental`.** The legacy paths that remain (`/mitigations`, `/roles`,
`/audit-logs`, `/users`, `/tenants`, `/tokens`, `/marketplace`,
`/custom-fields`, `/analytics`, `/risk-management`, `/bulk-operations`,
`/assets/universe`) are all `<Navigate replace>` redirects to a real destination
— deep links that land somewhere useful, not placeholder pages.

---

## Dashboard Audit

The dashboard dispatches on `business_role` into five personas
(`dashboardPersona.ts`). Every KPI on all five was traced to its source.

| KPI | Persona | Source | Status |
| --- | --- | --- | --- |
| Total / critical / mitigating / resolved risks | posture | `GET /stats` counts | REAL |
| 5×5 probability × impact matrix | posture | `GET /stats` `risk_matrix`, banded in SQL | REAL |
| Risk trend | posture | derived from the loaded register | REAL |
| Recent activity | posture | `GET /risks` newest-first | REAL |
| War Room card | posture | newest open/in-progress incident, live duration | REAL |
| Security score | posture, viewer | `GET /score` | REAL |
| Open / P1 / KEV vulnerabilities, priority queue | analyst | `GET /vulnerabilities/stats`, `/vulnerabilities` | REAL |
| Cyber score A–F, ALE, worst case, KRI band | exec | `GET /analytics/executive` | REAL |
| Coverage per framework, gaps | audit | `GET /compliance` overview | REAL |
| Asset totals, criticality distribution | estate | `GET /assets` | REAL |

**No fabricated KPI was found on any dashboard.** The heatmap literal and the
hardcoded incident that earlier audits recorded are gone, and the code carries
comments recording why. One deliberate omission is worth naming: the KPI cards
show **no period-over-period delta**, because the API exposes no history to
compute one — an absent delta rather than an invented one.

Cell tinting on the heatmap is computed from the grid's own coordinates
(`bucket × bucket`, 1–25) and is documented in place as a legend, not a score
band, so it is not mistaken for the Score Engine's thresholds.

---

## KPI Audit

Beyond the dashboard, every rendered metric was traced. The sidebar footer's risk
and mitigation badges (previously the literals `12` and `3`) now render a number
only when one was measured, and render nothing otherwise — fixed in W0-04 and
re-verified here.

The single remaining `openrisk/no-mock-data` build error was a **false positive**:
`FinancialDashboard.tsx` passes `seed: b.seed`, the Monte-Carlo RNG seed returned
by the backend, and the rule matches the word `seed` in any identifier. Suppressed
at that one line with the reason recorded, which is the rule's documented escape
hatch — the guard stays at `error` for everything else.

---

## Cards and Charts Audit

Every chart in the reachable set draws from a query. No chart has a fixture
fallback. Verified by inspection of each `Recharts` and SVG call site:

* `ExecDashboard` / executive view — `GET /analytics/executive`
* `FinancialDashboard` — `GET /analytics/financial`; the 5-year projection is
  computed from the tenant's own ALE and residual ALE, not a sample curve
* `SmartRiskRadar` — `GET /risks/:id/smart-score`
* `TopologyView` — `GET /assets` + `GET /asset-dependencies`
* `RiskHeatmap` (dashboard) — `GET /stats`

The pattern `data || fixture`, `response ?? demoData` and `catch → return
fixture` appears **only in orphaned modules** (`api/scoreEngineService.ts` has
nine such fallbacks). No reachable module has one.

---

## Modal and Menu Audit

All modals in the reachable set submit to an API and surface loading, error and
success states from the response. The `DangerConfirm` primitive (impact
radiography + alternatives) guards the vital deletions: member revocation, API
token revocation, framework deletion, organisation deletion.

`<DataTable>` makes decorative table affordances structurally inexpressible: a
column is sortable only if it declares *how* to sort, a facet only if it declares
*how* to filter, a row action only if it has an `onSelect`. That is why the
sweep for inert controls found nothing in tables.

---

## Primary Action Audit

Automated sweep of every `<button>` opening tag in the reachable `.tsx` set,
parsed across line breaks, looking for tags with no `onClick` / `onMouseDown` /
`onPointerDown` / `type="submit"` / spread props.

**Result: zero inert buttons.** (One match was a `<button>` mentioned inside a
code comment.)

Handler-present-but-effectless actions do not show up in that sweep, so the
primary CTAs were also traced by hand:

| CTA | Effect | Status |
| --- | --- | --- |
| New risk | `POST /risks`, register refetch | REAL |
| Add asset | `POST /assets` | REAL |
| Invite member | `POST /invitations` | REAL |
| Run scan | `POST /scanner/configs/:id/scan` | REAL |
| Generate report | `POST /reports`, job page | REAL |
| Export CSV | client-side from the loaded view, formula-injection escaped | REAL |
| Add control / import catalog | `POST /compliance/*` | REAL |
| Connect integration | **rendered a literal grid; toggles persisted nothing** | **DECEPTIVE → fixed (D1)** |
| New custom field | **`toast('coming soon')` only** | **DECEPTIVE → fixed (D4)** |
| Send War Room note | **local state only** | **DECEPTIVE → fixed (D6)** |

---

## Dead-End Workflow Audit

| Workflow | Before | After |
| --- | --- | --- |
| Configure an integration | Toggle flips → nothing persists → reload shows the literal again | Real state from three APIs; each card routes to where it is genuinely configured |
| Set a notification preference | Writes `localStorage` → no delivery changes → carries to the next user on the browser | `PATCH /notifications/preferences`, per user and tenant, and **enforced at the single send choke point** |
| Add a custom field | Primary CTA → toast → nothing | CTA removed; the state explains the API is read-only from this screen and links to the docs |
| Coordinate in the War Room | Type → bubble appears → nobody receives it | Composer replaced by the real incident action board (`POST /incidents/:id/actions`) |
| Open a risk's timeline | Crash, or "0 changes" on an auth failure | Redirects to the register's real Timeline tab |
| Schedule a report | Blurred panel showing three schedules the tenant does not have | Honest description of the capability, no invented rows |

---

## Fixture Leakage

`openrisk/no-mock-data` (ESLint, `error`) forbids identifiers named for
fabrication (`mock`, `fake`, `dummy`, `stub`, `fixture`, `sample`, `demo`,
`seed`, `lorem`, `faker`) and TanStack `placeholderData` with a literal, across
all of `src/` except tests. It was already in place; this wave leaves it at
`error` with the one documented suppression above.

The rule keys on **names**, which is the reliable signal but not a complete one.
This wave adds a second, structural check that the name-based rule cannot make:
`frontend/scripts/audit-surfaces.mjs` walks the import graph from `main.tsx` and
fails if a module under `dev/fixtures/`, `tests/`, or any `__tests__` directory
is reachable from production code. Demonstration data lives in `dev/fixtures/*.json`,
outside `src/` and outside the bundle, loaded by the Go seeder under `DEMO_MODE`
only — with `DemoBanner` rendering a non-dismissible banner whenever the server
reports that mode.

**Orphaned fixture reservoir (not user-facing, recorded not removed):** 110
modules are unreachable, including `pages/Marketplace.tsx`,
`pages/AnalyticsDashboard.tsx`, `pages/RealTimeAnalyticsDashboard.tsx`,
`pages/ThreatMap.tsx`, `features/settings/IntegrationsTab.tsx` (which simulates a
connection with `await new Promise(r => setTimeout(r, 2000))`) and
`api/scoreEngineService.ts`. Deleting them is a larger change than this wave's
remit and would collide with in-flight branches; the audit script reports the
count so it can only shrink, and the number is asserted in
[Automated Checks](#automated-checks).

---

## Placeholder Routes

None. See the Route Inventory note above: every legacy path is a redirect to a
real destination, and no route path matches the placeholder patterns.

The two screens with no backend — `/simulations` and `/leaderboard` — are
`EXPERIMENTAL`: badged, explaining what the capability will do, and routing to
the real analyses that answer part of the same question today (dependency map and
financial quantification for simulations; the risk register and compliance for
the leaderboard's scoring rules). Neither renders a gauge, a standing or a run
history.

---

## Feature Flags

| Flag | Owner | Default | UI effect | Backend effect |
| --- | --- | --- | --- | --- |
| `DEMO_MODE` | server | off | `DemoBanner`, non-dismissible | seeds `dev/fixtures/*.json` |
| Entitlements (`plan`) | server | `free` | `FeatureGate` blurs the **real** paid UI with an explaining upsell | `402` on the paid endpoint |
| `CTI_SYNC_ENABLED` | server | off | none | periodic NVD/CISA sync (manual sync always available) |
| `VULN_LIVEPULL_ENABLED` | server | off | none | scheduled live pulls |

`FeatureGate` is presentation only and blurs components that render **real**
tenant data — the gate is a paywall, not a fiction, and the backend enforces it
independently. This is a legitimate use of a blur; `PremiumPeek` blurring
*invented* rows was not, which is finding D5.

---

## Loading States

Every reachable list and detail screen renders a skeleton (`SkeletonRows`,
`Skeleton`) while its query is in flight — never a static value that would imply
the number is already known. Route chunks render `RouteFallback` (a labelled
`role="status"` spinner) while lazy-loading.

---

## Empty States

`shared/EmptyState` distinguishes `first-use` (nothing exists yet, with a CTA to
create the first one) from `no-results` (filters excluded everything, with a
reset). The core registers — risks, vulnerabilities, assets, incidents,
compliance — all carry an actionable CTA rather than a bare "no data".

The distinction that matters most is between *empty* and *failed*, which is what
D8 got wrong: `/risks/:id/timeline` rendered an authentication failure as "Total
changes: 0".

---

## Error States

`shared/HistoryTimeline` and the DataTable both take an explicit `error` prop and
render a retry rather than an empty body. The axios layer distinguishes a
refreshable `TOKEN_EXPIRED` (one silent refresh, then replay) from a revoked or
invalid token (redirect to login), and does not log the user out on a 403 or a
permission 401 — the bug that used to eject users with perfectly valid sessions.

No reachable module falls back to a fixture on error.

---

## Permission States

`RoutePermissionGuard` reads the required permission from the route tree — the
same declaration the breadcrumb and the sidebar read, so the guard, the trail and
the nav cannot disagree — and renders `AccessDenied` (naming the missing
permission) rather than a blank or a fabricated empty list. Unmapped paths fail
open to the backend, which enforces regardless.

`AccessDenied` ("you don't have permission") is a distinct screen from
`EmptyState` ("nothing here yet") and from the `EXPERIMENTAL`/`UNAVAILABLE`
states, which is the distinction §18 of the brief asks for.

The frontend never decides authorisation on its own: the nav hides what the JWT's
permissions do not cover, and the API returns 403 independently.

---

## Tenant Isolation

Backend isolation was swept exhaustively in a previous wave
(`audit/tenant-isolation-sweep`), which found and fixed six live cross-tenant
leaks. This wave audited the **client** side, which that sweep did not cover.

**Finding D9 — the client cache survived a change of user.** `logout()` cleared
the auth store, the in-memory access token and the `auth_user` key, but not:

* the TanStack Query cache (`staleTime: 30s`, `gcTime: 24h`), holding every
  tenant-scoped response the session had fetched;
* the Zustand data stores (risks, assets, mitigations, compliance);
* `localStorage` keys carrying user-scoped state.

Because login and logout are both **soft SPA navigations** — `navigate('/login')`
then `navigate(landing)`, with no page reload in between — the cache was fully
intact across the change of identity. The next user to sign in on that browser
was painted the previous tenant's risks, assets, members and compliance posture
until each refetch landed, and, for any query still inside its 30-second stale
window, without a refetch at all.

Fixed by clearing the query cache and the client stores at logout **and** at
login (defence in depth: the second clear covers a session that ended by
expiry, by a 401 redirect, or by a crash rather than by the logout button).

Query keys were also reviewed. They remain global (`['risks']`, `['members']`)
rather than tenant-scoped, which is safe **given** the unconditional clear —
and the clear is the stronger guarantee, since a key scheme only protects the
queries someone remembered to key.

---

## Automated Checks

Three new mechanical guards, run by `npm run audit:surfaces` and asserted by
`frontend/src/__tests__/deceptive-ui.test.ts`:

1. **Placeholder route detection** — parses the route table in `App.tsx` and
   fails on a route path matching `demo|test|mock|placeholder|coming-soon|
   experimental|preview`, and on any element that is a known placeholder
   component. Legitimate exceptions are declared in the script with a reason.
2. **Fixture leakage detection** — walks the import graph from `main.tsx` and
   fails if any module under `dev/fixtures/`, `tests/`, or `__tests__/` is
   reachable. Also reports the reachable/orphan split so the orphan count can
   only shrink.
3. **Inert primary action detection** — parses every `<button>` opening tag in
   the reachable `.tsx` set and fails on one with no interaction prop.

These complement, rather than replace, `openrisk/no-mock-data`: that rule catches
fabrication by *name*, these catch it by *structure*.

---

## E2E Validation

### The harness had to be repaired first

No E2E test in this repository could run. `global-setup` died on
`admin login returned no access_token` before a single spec started, so the
suite had been dark since two earlier changes landed:

* **MFA is mandated for every account** (W0-03). `/auth/login` answers 200 with a
  short-lived `mfa_token` and nothing else, and both the seed and the login
  helper treated a missing `token_pair` as a failure.
* **The session moved to HttpOnly cookies.** The minted `storageState` put
  `auth_token` in localStorage, which authenticates nothing — and worse than
  nothing: `auth_user` was present, so the route guard let the browser through
  while every API call 401'd. Any authenticated spec would have been asserting
  against a logged-out app.

Both are fixed: the seed and the helper complete whichever second factor was
demanded (TOTP computed from Node's own crypto, no dependency), the seed records
the secret it enrolled so a re-run can answer a challenge instead of failing with
a bare "invalid code", and the `storageState` now carries the real
`or_access` / `or_refresh` / `or_csrf` cookies.

**Three tests in `dead-controls.spec.ts` fail, and were already stale.** They
target `universe-node-count`, `universe-filter` and `delete-org` — testids for
components earlier waves replaced (Asset Universe became `TopologyView`, the
danger zone became `DangerZonePanel`). None of those surfaces belongs to this
wave, so they are reported rather than rewritten. The staleness was simply
invisible while nothing could run.

### The W0-05 scenarios

`tests/e2e/deceptive-ui.spec.ts` covers the six scenarios the brief requires,
each asserting an observable effect rather than the presence of an element:

| Scenario | Assertion |
| --- | --- |
| Real capability | Settings › Notifications: toggle → `PATCH /notifications/preferences` observed → reload → the server's value is what renders |
| Unavailable capability | Custom fields: the state is present, the inert CTA is gone, the next action navigates |
| Experimental capability | `/simulations` and `/leaderboard`: badged, and assert **no** gauge / standing / run history is rendered |
| API failure | Register with `/risks` routed to 500: an error state with a retry, and **no** rows |
| Empty state | A tenant with no risks: `first-use` empty state with a CTA, not a fabricated matrix |
| Permission denied | A `viewer` on an admin route: `AccessDenied` naming the permission, distinct from empty |

Plus the two integrity scenarios this wave's findings demand: the integrations
tab must not render a connected provider without a configuration behind it, and
signing out then in as a second user must not paint the first user's data.

---

## Security Validation

| Check | Result |
| --- | --- |
| Cross-tenant leakage via client cache | **Found (D9)** and fixed; regression-tested |
| Fixture leakage into a production route | None reachable; guarded by two mechanical checks |
| Secrets in a response | None — webhook URLs and credentials are `json:"-"`, surfaced only as `has_*` booleans |
| Secrets rendered by the new integrations panel | None — the panel renders `has_slack` / `slack_enabled`, never a URL or a key |
| Unauthorised action execution | Writes remain permission-guarded client-side and enforced server-side |
| IDOR / BOLA | Out of this wave's scope; covered by the tenant-isolation sweep |

---

## Performance Validation

Removing fixtures risks replacing one literal with a burst of requests. It did
not here:

* The integrations panel issues **three** queries (`/automation/channels`,
  `/vulnerabilities/integrations`, `/vulnerabilities/ticketing`), all already
  fetched elsewhere in the app and therefore served from the shared cache when
  the user arrives from Automation or Vulnerabilities.
* The notifications panel issues **one** query and one mutation.
* The War Room task board issues **one** query, scoped to the incident already
  loaded.
* The backend preference gate adds **one indexed read** per notification send
  (`notification_preferences` is uniquely indexed on `(user_id, tenant_id)`), on
  a path that already writes a row.

No new N+1, no waterfall, no duplicated aggregate.

---

## Surface Disposition Matrix

| # | Surface | Route | Previous State | Final State | Data Source | Primary Action | Tenant Safe | Automated Test | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| D1 | Integrations grid | `/settings?tab=integrations` | DECEPTIVE — 6 literals, 3 shown connected, toggles inert | **REAL** | `/automation/channels`, `/vulnerabilities/integrations`, `/vulnerabilities/ticketing` | Routes to the real config screen per channel | Yes — all three are tenant-scoped | `deceptive-ui.spec.ts` | Live §Integrations |
| D2 | Notification preferences | `/settings?tab=notif` | DECEPTIVE — localStorage, never enforced | **REAL** | `GET/PATCH /notifications/preferences` | Toggle persists and is enforced at send | Yes — keyed `(user_id, tenant_id)` | `deceptive-ui.spec.ts`, Go unit tests | Live §Notifications |
| D3 | Policy toggles | `/settings` | DECEPTIVE — claimed server-side control that has no knob | **REAL** (read-only statement of enforced policy) | Static description of enforced behaviour | None (informational) | n/a | `deceptive-ui.spec.ts` | Live §General |
| D4 | "New custom field" CTA | `/settings?tab=fields` | DECEPTIVE — inert, `toast('coming soon')` | **UNAVAILABLE** | `GET /custom-fields` (list is real) | Docs link + return path | Yes | `deceptive-ui.spec.ts` | Live §Fields |
| D5 | Scheduled-reports peek | `/reports` | DECEPTIVE — 3 invented schedules, blurred | **UNAVAILABLE** | none | Honest capability description | n/a | `deceptive-ui.spec.ts` | Live §Reports |
| D6 | War Room chat composer | `/incidents/:id/war-room` | DECEPTIVE — local state, delivered nowhere | **removed** | n/a | Replaced by the action board | n/a | `deceptive-ui.spec.ts` | Live §War Room |
| D7 | War Room task board | `/incidents/:id/war-room` | UNAVAILABLE while a real API existed | **REAL** | `GET/POST/PUT /incidents/:id/actions` | Create and complete an action | Yes — `ownsIncident` | `deceptive-ui.spec.ts` | Live §War Room |
| D8 | Risk timeline page | `/risks/:riskId/timeline` | DECEPTIVE — crashed on data, failure rendered as "0 changes" | **REAL** (redirect to the real Timeline tab) | `GET /risks/:id/timeline` via the shared client | Opens the drawer's Timeline tab | Yes | `deceptive-ui.spec.ts` | Live §Timeline |
| D9 | Client cache across identities | all routes | DECEPTIVE — previous tenant's data shown to the next user | **REAL** | n/a | n/a | **Fixed** | `deceptive-ui.spec.ts`, unit test | Live §Isolation |
| D10 | Risk drawer CTI tab | `/risks` | DECEPTIVE — bare "coming soon", no next action | **REAL** for CVE-sourced risks, **UNAVAILABLE** otherwise | `GET /cti/vulnerabilities/:cve` | Opens the CVE in Threat Intel | Yes | `deceptive-ui.spec.ts` | Live §CTI tab |
| D11 | Monte-Carlo `seed` | `/analytics/financial` | build guard failing (false positive) | **REAL** | `GET /analytics/financial` | n/a | Yes | lint | `npm run lint` |
| D12 | Settings dead components | `/settings` | dead `Toggle` / `ToggleRow` / `Field` (unsaved input) | **removed** | n/a | n/a | n/a | audit script | diff |
| D13 | Audit-trail URL | `/governance/audit-trail` | DECEPTIVE — opened the Approvals tab instead | **REAL** | `GET /governance/audit-events` | Tab reflected in the URL | Yes | `deceptive-ui.spec.ts` | Live §Governance |
| D14 | Audit trail for a non-admin | `/governance/audit-trail` | DECEPTIVE — denial rendered as "nothing to approve" | **REAL** (`AccessDenied`) | n/a | Names the permission | Yes | `deceptive-ui.spec.ts` | Live §Governance |
| D15 | Preferences JSON contract | `/settings?tab=notif` | DECEPTIVE — GET spoke Go field names, PATCH spoke snake_case; three credential fields serialisable | **REAL** | `GET/PATCH /notifications/preferences` | Round-trips | Yes — `(user_id, tenant_id)` | Go unit test | Live §Notifications |

**`DECEPTIVE` remaining at the end of the wave: 0.**

---

## Acceptance Matrix

| Requirement | Status | Evidence |
| --- | --- | --- |
| No fake KPI in production | **PASS** | Every dashboard KPI traced to its query (§Dashboard Audit). The one `no-mock-data` error was a false positive on a Monte-Carlo RNG seed, suppressed with its reason. |
| No fake integrations in production | **PASS** | Six literals with three "connected" replaced by three real APIs; E2E asserts nothing reads Active with nothing configured, and that Splunk is gone. |
| No inert primary actions | **PASS** | Automated parse of every `<button>` in 275 reachable modules: zero without a handler. The toast-only *New field* CTA removed. |
| No deceptive placeholder routes | **PASS** | No route path matches the placeholder patterns; every legacy path asserted to redirect to a real destination. |
| Fixture-backed production surfaces removed | **PASS** | `openrisk/no-mock-data` at 0; import-graph check finds no test or `dev/fixtures` module reachable from `main.tsx`. |
| Experimental surfaces explicitly labelled | **PASS** | `/simulations` and `/leaderboard` badged, and E2E asserts the invented gauge, standing and history are absent. |
| Unavailable surfaces honest and useful | **PASS** | Custom fields, scheduled reports and War Room messaging each state the situation and offer a next action. |
| Tenant isolation maintained | **PASS** | Cross-tenant reads and writes → 404 live; the client cache, stores and user-scoped storage cleared at both ends of a session change, with unit and E2E coverage. |
| Loading states | **PASS** | Skeletons on every reachable list and detail; the preferences panel renders a skeleton rather than a guessed switch position. |
| Empty states | **PASS** | `first-use` vs `no-results` distinguished; a tenant registered during the run shows zero and no invented people. |
| Error states | **PASS** | 500 on `/risks` and on `/notifications/preferences` both render errors with a retry, and no rows / no switch. |
| Permission-denied states | **PASS** | API refuses with 404; UI renders `AccessDenied` naming the permission, asserted distinct from the empty state. |
| Audit states | **PASS** | `/governance/audit-trail` opens the audit trail; a non-admin is refused out loud instead of shown another tab's empty state. |
| Unit tests | **PASS** | 183 passing (6 session-scope, 15 deceptive-ui guards, 6 Go preference tests, plus the existing suite). 1 pre-existing failure, reproduced on a clean stash. |
| Integration/API tests | **PASS** | Go suite: 61 packages, 0 failures, including the preference gate and the JSON contract. |
| E2E tests | **PASS (W0-05) · PARTIAL (suite)** | `deceptive-ui.spec.ts` 11/11. The suite as a whole was **BLOCKED** before this wave and now runs; three `dead-controls` tests fail against testids earlier waves removed, and `a11y.spec.ts` surfaces real pre-existing WCAG violations. |
| Accessibility | **PARTIAL** | New states reuse audited primitives and expose `aria-pressed` / `role="status"` / labels. No dedicated axe pass on the new states; `a11y.spec.ts` shows pre-existing violations elsewhere. |
| Security | **PASS** | Credential fields cut from the preferences payload (asserted with them populated); no secret rendered by the integrations panel; cross-identity cache leak closed. |
| Performance | **PASS** | Request counts per changed surface recorded; one indexed read added to the notification path. |
| Live proof | **PASS** | [`W0-05_DECEPTIVE_UI_LIVE_PROOF.md`](./W0-05_DECEPTIVE_UI_LIVE_PROOF.md) |

---

## Live Proof Record

See [`W0-05_DECEPTIVE_UI_LIVE_PROOF.md`](./W0-05_DECEPTIVE_UI_LIVE_PROOF.md) —
environment, tenants, commands, observed responses and screenshots of record.

---

## Known Limitations

Recorded in the live-proof document alongside the evidence that bounds them.
The three that most affect how this wave should be read:

1. Three `dead-controls.spec.ts` tests fail against testids earlier waves
   removed. Reported, not rewritten.
2. `a11y.spec.ts` now runs and surfaces real, pre-existing WCAG violations
   (missing progressbar names, colour contrast) across many routes. Newly
   *visible*, not newly introduced.
3. The e-mail half of the preference gate is proven by unit test and by reading
   the four call sites, not by watching an e-mail fail to arrive — no SMTP
   server is configured here.

---

## Recommended Follow-up Work

1. **Delete the 110 orphaned modules.** They are the reservoir this wave's
   findings were drawn from. A separate, mechanical change with its own risk
   profile.
2. **Ship the missing capabilities honestly labelled today**: scheduled reports,
   the simulation engine, tenant-wide leaderboard ranking, the custom-field
   editor, War Room presence and chat.
3. **Per-event in-app notification preferences.** The model has per-event flags
   for email, Slack and webhook, but only a global switch for in-app; the
   Settings screen therefore offers per-event control for email and a single
   switch for in-app. Adding the in-app columns would let the two match.
4. **Tenant-scope the query keys** as belt-and-braces alongside the cache clear.
