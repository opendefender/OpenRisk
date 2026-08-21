# W0-05 — Deceptive UI · Live Proof Record

Everything below was observed against a running stack. Nothing here is inferred
from reading code; where a claim could only be established by reading, it says
so and appears under [Known Limitations](#known-limitations) instead.

---

## Environment

| | |
| --- | --- |
| Date | 2026-08-21 |
| Backend | Go 1.25.12, built from this branch, `:8080` |
| Frontend | Vite dev server, `:5174` (a clean instance; the pre-existing `:5173` server's API proxy was returning 500) |
| Database | PostgreSQL 16 on `:5434` (`openrisk`) |
| Cache / bus | Redis on `:6379` |
| Branch | `feat/w0-05-remove-deceptive-ui` |
| Commit | `dbe3714` |

The backend was rebuilt and restarted mid-session, after the JSON-tag fix, and
every result below the restart marker was re-observed against the new binary.

## Test Tenants and Users

Created through the **real signup flow** (`POST /auth/register`), then through
the **real mandatory MFA enrolment** (`/auth/mfa/setup` → `/auth/mfa/verify`
with a computed TOTP). No user was inserted by hand and no session was forged.

| Alias | Purpose |
| --- | --- |
| `w005-a-…@openrisk.test` / *W005 a Co* | Owner of the incident, risk and preference data exercised below |
| `w005-b-…@openrisk.test` / *W005 b Co* | The second tenant, used to prove isolation |
| `admin@opendefender.io` / *OpenDefender* | The seeded E2E persona (14 risks, 3 frameworks, 5 vulnerabilities) |
| `w005-empty-…`, `w005-viewer-…` | Registered per-run by the E2E empty-state and permission scenarios |

---

## Real Capabilities Tested

### Notification preferences (D2, D15)

Before the fix, `GET` and `PATCH` on the same path spoke different vocabularies:

```
GET /api/v1/notifications/preferences
{ "ID": "dc7cfc64-…", "DisableAllNotifications": false,
  "SlackWebhookURL": "", "WebhookSecret": "", … }
```

`PATCH` bound `disable_all_notifications`. No client could read a preference and
write it back. After the fix, and after a backend restart:

```
GET  → { "id": "dc7cfc64-…", "disable_all_notifications": false,
         "email_on_critical_risk": true, "slack_enabled": false, … }
         ← no slack_webhook_url, no webhook_secret, no user/tenant relation

PATCH {"disable_all_notifications": true}   → disable_all_notifications = True
PATCH {"disable_all_notifications": false}  → restored to False
```

Driven from the UI by the E2E scenario: clicking the switch produces a real
`PATCH` (**200**), the rendered position is the server's answer rather than the
click, and it survives a reload.

### War Room response tasks (D7)

```
POST /api/v1/incidents                      → 201  (incident 5, critical)
GET  /api/v1/incidents/5/actions            → 200  []        ← empty, not "unavailable"
POST /api/v1/incidents/5/actions            → 201  {"id":1,
        "title":"Isolate db-02 from the payment VLAN",
        "assigned_to":"alice@w005a.test","status":"pending"}
PUT  /api/v1/incidents/5/actions/1          → 200  → list reads [(1,'in_progress')]
PUT  /api/v1/incidents/5/actions/1          → 200  → list reads [(1,'completed')]
```

Each status was **read back from the server**, not asserted from local state.

### Risk drawer CTI tab (D10)

```
GET /api/v1/risks/<manual risk>   → source_cve_id = None      ← the "no CVE" branch
GET /api/v1/cti/vulnerabilities/CVE-2021-44228 → 200
    cisa_known: true, cisa_due_date: 2021-12-24,
    mitre_tactics: [TA0001, TA0002], mitre_techniques: [T1190, T1059…]
```

Both branches of the tab are backed by data the running system actually holds.

### Governance audit trail (D13, D14)

Reproduced before the fix: a restricted session at `/governance/audit-trail`
rendered the **Approvals** tab, showing `"Rien à approuver"` — the page
snapshot is in
`tests/e2e/.artifacts/test-results/deceptive-ui-permission-de-*/error-context.md`.
After the fix the same navigation renders `AccessDenied` naming the permission,
asserted by the E2E permission scenario.

---

## Unavailable Capabilities Tested

| Surface | Observed |
| --- | --- |
| Custom-field editor | "Éditeur de champs unavailable" renders; the toast-only *New field* CTA is absent; an API-docs action is present |
| Scheduled reports | `scheduled-reports-unavailable` badge renders; neither "Every Monday" nor "Board email" appears anywhere |
| War Room messaging | The composer is gone; the panel states that real-time messaging does not exist and points at the task board |

## Experimental Capabilities Tested

| Surface | Observed |
| --- | --- |
| `/simulations` | "Coming soon" badge; **no** 8.4 gauge, **no** "last run" |
| `/leaderboard` | "Coming soon" badge; **no** "#2", **no** invented colleague names |

---

## Fixture Leakage Checks

```
$ npm run audit:surfaces
Reachable modules: 275
Orphaned modules:  110  (unreachable from main.tsx; not user-facing)

PASS  Placeholder routes
PASS  Fixture leakage
PASS  Inert primary actions
```

```
$ npx eslint .
openrisk/no-mock-data: 0        (was 1 — a false positive on the Monte-Carlo seed)
```

## Placeholder Route Checks

No route path matches `demo|mock|placeholder|coming-soon|experimental|dummy|
fixture|sandbox`. Every legacy path is asserted to be a `<Navigate replace>` to
a real destination, by test rather than by inspection.

## Inert Action Checks

Automated parse of every `<button>` opening tag in the 275 reachable `.tsx`
modules, across line breaks, with comments stripped: **zero** without an
interaction prop.

---

## API Failure Cases

| Case | Observed |
| --- | --- |
| `GET /risks` → 500 | Error state with a retry; **0 rows** rendered, despite the tenant holding 14 risks and the cache holding them moments earlier |
| `GET /notifications/preferences` → 500 | "Preferences unavailable" with a retry; **no switch** rendered in a guessed position |
| `GET /cti/vulnerabilities/:cve` fails | Advisory-unavailable state, not "no CVE linked" — which would be a different and wrong claim about a risk that names one |

No reachable module falls back to a fixture on error.

## Empty States

A tenant registered during the run, with onboarding completed through the API,
lands on `/risks` and sees the first-use empty state. The dashboard shows none
of `Fatou`, `Amir`, `INC-2026-014` or `srv-paie-01` — the invented people,
incident and host that earlier releases rendered on every tenant.

## Permission Denied States

```
Tenant B → GET /api/v1/risks/<tenant A's risk>   → 404      (not 403: the id is not even confirmed)
Tenant B → GET /api/v1/incidents/5               → 404
Tenant B → GET /api/v1/incidents/5/actions       → 404
Tenant B → POST /api/v1/incidents/5/actions      → 404
```

The API is the authority and refuses without disclosing. The UI renders
`AccessDenied`, naming the missing permission — a distinct screen from the
empty state, which the E2E scenario asserts by requiring the empty-state copy to
be **absent**.

## Tenant Isolation

The client-side finding (D9) is covered by unit tests that assert the effect:
the query cache is empty after `clearSessionScope`, registered stores are back
to their initial shape, user-scoped storage keys are gone and device preferences
survive. The E2E scenario additionally asserts that no `openrisk.table.*` saved
view — which embeds tenant vocabulary in its filter values — outlives a sign-out.

Those tests found a defect in the shared harness on the way: the `localStorage`
mock declared `length` as a plain property evaluated once at import, so it was
permanently `0` and every `for (i < localStorage.length)` sweep was a silent
no-op. Made a getter.

## Accessibility

The new states are built from the existing primitives (`EmptyState`,
`ErrorState`, `AccessDenied`), which carry headings, roles and focus handling.
The new switch exposes `aria-pressed` and `aria-label` and is `disabled` while
in flight; its save/failure indicator is `role="status"`. The War Room task
board labels its inputs and its status control.

`a11y.spec.ts` runs in the suite and passes on the routes it covers. **No
dedicated axe-core pass was run against the specific new states** — see Known
Limitations.

## Performance

| Surface | Requests |
| --- | --- |
| Settings › Integrations | 3 (`/automation/channels`, `/vulnerabilities/integrations`, `/vulnerabilities/ticketing`), all shared-cache hits when arriving from Automation or Vulnerabilities |
| Settings › Notifications | 1 query + 1 mutation per change |
| War Room task board | 1, scoped to the already-loaded incident |
| Backend preference gate | 1 indexed read per send, on `notification_preferences` (unique on `(user_id, tenant_id)`), on a path that already writes a row |

No new N+1, no waterfall, no duplicated aggregate.

---

## Commands and Results

| Command | Result |
| --- | --- |
| `go build ./...` | **PASS** |
| `go vet ./...` | **PASS** |
| `go test ./...` | **PASS** — 61 packages, 0 failures |
| `npx tsc -b --force` | **PASS** |
| `npx vite build` | **PASS** |
| `npx vitest run` | **183 passed, 1 failed** — the failure is `App.integration.test.tsx › should render Login page when not authenticated`, **pre-existing** (reproduced with this branch's changes stashed) |
| `npm run audit:surfaces` | **PASS** on all three checks |
| `npx eslint .` | `openrisk/no-mock-data` **0** (was 1). 357 other findings, all pre-existing; **no new** unused import or violation in any file this wave touched, verified per-file against the baseline |
| `npx playwright test tests/e2e/deceptive-ui.spec.ts` | **11/11 PASS** |
| `npx playwright test` (full suite) | see below |

### Full E2E suite

Before this wave: **BLOCKED** — `global-setup` failed on every run, so no spec
executed at all.

After the harness repair, run serially as CI runs it (`--workers=1`):
**91 passed · 27 failed · 4 skipped**. `deceptive-ui.spec.ts` is 11/11 within
that run.

The 27 are on surfaces this wave did not touch, and running the suite is what
made them visible. Three causes were identified and two of them fixed:

| Cause | Count | Disposition |
| --- | --- | --- |
| Playwright `baseURL` dropping the `/api/v1` prefix on paths starting with `/` | 4 | **Fixed** — `journey.members` went 5 → 3 |
| `apiLogin` needing the TOTP secret passed explicitly (a regression introduced by the harness repair itself) | 5 | **Fixed** — now defaulted, so no call site can forget |
| Real pre-existing WCAG violations (`a11y.spec.ts`) | 11 | Reported. Verified pre-existing: `/governance` fails identically with this branch stashed |
| Testids removed by earlier waves (`dead-controls`) | 2 | Reported — the testids exist nowhere in the codebase |
| UI login now requiring MFA (`auth.login.spec.ts`) | 2 | Reported |
| Not root-caused | the remainder | Reported. On member-invitation and risk-lifecycle surfaces this wave did not touch |

**Honest statement of what that last row means**: those specs could not run
before this branch, so "pre-existing" cannot be demonstrated by re-running them
on the baseline — only that the surfaces are not ones this wave changed.

### The suite exceeds the auth rate limit

The dominant cause of the remaining failures, and the one worth acting on first.

`/auth/login` and `/auth/register` sit behind a per-IP limiter of **15 requests
per 5 minutes** (`main.go`, deliberately widened from 5/15min so a couple of
mistyped passwords do not lock a real user out). The suite makes far more than
that from one IP: `journey.members` builds an admin API context **per test**,
`risk-lifecycle` does the same, and several specs register a tenant of their own.

So a full run throttles itself, and the resulting `429`s surface as assertion
failures further down each test — a login failure at the top of a test looks
like a broken feature at the bottom of it. Re-running the four affected specs
immediately after a full run reproduces this exactly: **10 failed / 12 passed,
with `429 Rate limit exceeded` in the error of every one.**

This is not a product defect and not a defect in any individual spec; it is the
suite as a whole outrunning a control that is correctly sized for humans. The
fix is to log in once per file (or once per run) and share the context, rather
than per test — a change in specs this wave does not own, so it is recorded
rather than made.

**Consequence for reading any red run:** check for `429` before concluding
anything about the product.

---

## Known Limitations

1. **Two `dead-controls.spec.ts` tests fail.** They target `universe-node-count` /
   `universe-filter` and `delete-org`, testids that exist nowhere in the
   codebase, for components earlier waves replaced. Reported, not rewritten:
   those surfaces are not this wave's, and asserting new behaviour on them
   without studying it would be guessing.
2. **The e-mail half of the preference gate is proven by unit test, not by a
   delivered message.** No SMTP server is configured here, so
   `ShouldNotify(...Email)` is asserted at the use-case boundary and at the four
   call sites by inspection of the diff — not by watching an e-mail fail to
   arrive.
3. **The in-app notification gate has no per-event control.** The stored model
   has `EmailOn*` / `SlackOn*` / `WebhookOn*` columns but only a global switch
   for in-app, so the Settings screen offers per-event control for e-mail and a
   single switch for the bell. Offering per-event in-app rows would mean showing
   switches the server cannot honour.
4. **No dedicated axe-core pass on the new states.** `a11y.spec.ts` passes on the
   routes it covers, and the new states reuse audited primitives, but the
   specific `UNAVAILABLE` / `AccessDenied` / task-board renderings were not put
   through an accessibility scanner.
5. **110 orphaned modules remain.** They are unreachable from `main.tsx` and
   therefore not user-facing, but they hold most of the remaining fabricated
   literals and `catch → return fixture` fallbacks. The count is pinned by test
   so it can only shrink; deleting them is a separate, mechanical change.
6. **Slack, Teams, Jira and ServiceNow were not connected to real endpoints.**
   The integrations tab is proven to report what the three configuration APIs
   say, and proven not to invent a connection — but no live third-party delivery
   was exercised, because no credentials for those services exist here.
7. **The dev server on `:5173` has a broken API proxy** (returns 500). All
   browser-driven proof used a clean instance on `:5174`. That is an environment
   condition, not a product defect, and it was not investigated further.
8. **`docs/GITHUB_ISSUES_ANALYSIS.md` is untracked in the working tree** and was
   not authored by this wave; it is left alone.
