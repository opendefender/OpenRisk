# Changelog

All notable changes to OpenRisk will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
Git tags use the `vMAJOR.MINOR.PATCH[-rc.N]` convention; see [docs/VERSIONING.md](docs/VERSIONING.md).

## [Unreleased]

### Planned
- Board Report mensuel (IA, human-in-the-loop, FCFA) — the second half of M4
- Multi-tenant support
- Mobile app (React Native)
- Slack/Teams notifications
- Jira integration

## [1.1.0-rc.5] - 2026-09-21

> Supersedes `v1.1.0-rc.4`, which was tagged but never published: the release
> workflow failed to build the frontend, because a conflict resolution in the
> organization-switcher merge (#734) had dropped an import from the sidebar
> (#746). The tag `v1.1.0-rc.4` is left in place with no release attached — a
> pushed tag is never moved or deleted (see [docs/VERSIONING.md](docs/VERSIONING.md) §4).
> Everything listed under `[1.1.0-rc.4]` below ships in this release, including
> the fix for [GHSA-cjh4-89ww-2jpr](https://github.com/opendefender/OpenRisk/security/advisories/GHSA-cjh4-89ww-2jpr).
> The full list of changes since `v1.1.0-rc.3` is in
> [CHANGELOG.md](https://github.com/opendefender/OpenRisk/blob/v1.1.0-rc.5/CHANGELOG.md).

### Security
- **SSRF through integration URLs — GHSA-cjh4-89ww-2jpr, high (#743).** Scanner
  live-pull, Jira / ServiceNow ticketing and `POST /integrations/:id/test` could be
  pointed at loopback, private networks or cloud metadata, and could send stored
  credentials to another host. Outbound requests to tenant-configured URLs now only
  reach public https addresses, and credentials must be re-entered when `base_url`
  changes host.
  **Action for operators:** integrations whose `base_url` is `http://` or a private
  address stop working until fixed. Tools on your own network can be opened with
  `OUTBOUND_ALLOWED_CIDRS` (see `docs/SELF_HOSTING.md`); these requests no longer use
  `HTTP(S)_PROXY`.

### Added
- Change your password while signed in, from the settings screen. Your other devices
  are signed out and this one stays signed in; accounts that sign in through an
  identity provider are sent to it (#726, #720).

### Fixed
- The frontend builds again: the sidebar's "My profile" icon import was restored, and
  so were the organization's branded name and logo in the sidebar, which the #734 merge
  had replaced with the login name and initials (#747, #746).
- Every table production code uses is checked against the tables startup builds; the
  marketplace routes, whose tables are never built and which answered 500, are no
  longer mounted (#711, #707).
- The release workflow builds the frontend on Node 20, which Vite 7 requires (#748).

## [1.1.0-rc.4] - 2026-09-21

> Ships the fix for [GHSA-cjh4-89ww-2jpr](https://github.com/opendefender/OpenRisk/security/advisories/GHSA-cjh4-89ww-2jpr)
> to self-hosted operators, together with everything merged since `v1.1.0-rc.3`.
> Still a release candidate: the release trains that follow Wave 0 remain open.

### Security
- **SSRF through integration URLs — GHSA-cjh4-89ww-2jpr, high (#743, #573).** Scanner
  live-pull, Jira / ServiceNow ticketing and `POST /integrations/:id/test` fetched the
  tenant-configured `base_url` with no restriction, and echoed the remote response back
  through `last_pull_error`, ticket errors and the test endpoint. Changing only
  `base_url` also sent the stored API credentials to the new host. Outbound requests
  now go through `pkg/netguard`: https only; loopback, private, link-local (cloud
  metadata), CGNAT, reserved and IPv4-embedding IPv6 ranges refused when the URL is
  saved and again at connect time; redirects stay on the original host. Stored
  credentials must be re-entered when `base_url` moves to another host, and remote
  bodies are no longer echoed.
  **Action for operators:** integrations whose `base_url` is `http://` or a private
  address stop working until fixed. Tools on your own network can be opened with
  `OUTBOUND_ALLOWED_CIDRS` (see `docs/SELF_HOSTING.md`); these requests no longer use
  `HTTP(S)_PROXY`.
- `audit_logs` scoped to the acting organisation — a P0 cross-tenant read (#534).
- Audit of the 92 protected routes that had no permission guard; 17 were wrong (#577).
- Isolation assessment of the 97 pending collection routes (#533).
- Live permission-denial test matrix over all 52 guards, with a route ratchet (#530).
- Session revocation enforced on live streams (#526); no session for a disabled or
  deleted account (#525); MFA sign-in carries the current business role (#523).

### Added
- **Third-party risk (TPRM v1):** vendor register and the vendor → asset → risk chain on
  Business and Enterprise (#676), questionnaire templates, assessments and a public
  token link for vendors (#677, #682), J-7 / J-3 / J-1 reminders (#679). ADR 0004 (#675).
- **Onboarding tunnel:** posture reveal and recognition (#628), residual score from
  control coverage and a starter catalogue (#625), versioned Aha metric (#622), theme and
  language switch inside the tunnel (#667).
- **Organisation and profile:** editable organisation profile and regional settings
  (#721), organisation logo and accent for every member (#723), self-service profile,
  preferences and avatar (#722), switching between organisations (#734).
- **Action Center:** aggregation API (#431) and the `/action-center` page (#434, #436).
- Server-side saved table views that a tenant can share (#584); governed bulk
  operations for risks, vulnerabilities and assets (#585, #586, #603).
- Polymorphic entity contract, universal drawer and global timeline (#414).
- Locale registry, CLDR pluralisation and locale-aware formatting (#604).
- Tenant-safe response caching replacing the passthrough (#531).
- One-command self-host: first admin and generated entitlement table (#610).
- Design system: canonical token contract and theme foundation (#407, #423), form
  primitives, Spinner, AlertDialog, Popover, Menu, Command, OtpField, Empty, TagInput,
  RiskMatrix (#465–#477, #497, #642), in-house chart layer replacing Recharts (#498),
  scoped SVG entry motion (#515).
- Deferrable MFA enrolment before the Aha moment (#334).

### Changed
- Frontend migrated to Tailwind v4 (#449); Prettier adopted (#506); initial bundle
  reduced from 245.8 KB to 179.7 KB (#466).
- Licence references aligned on AGPL-3.0-only with a CLA and DCO check (#448, #516);
  `frontend/design-system/` and `frontend/src/shared/ds/` relicensed to Apache-2.0 (#456).
- Navigation regrouped into the five ratified intentions (#705).
- Lint, test, build, performance-budget and E2E gates made blocking in CI (#500, #469,
  #661).
- The Helm chart exposes `OUTBOUND_ALLOWED_CIDRS` (#743).

### Fixed
- Auth: session refresh when the access cookie has expired (#692); registration is
  atomic and duplicates nothing (#694); pasted one-time codes accepted (#478).
- Onboarding: the tunnel completes from its last step (#693), keeps one state per
  organisation (#736), stops its load-failure remount loop (#697), and five defects found
  by walking it (#633, #635, #644, #717).
- Transactions: mitigation creation (#518), risk bulk actions (#585), mitigation
  sub-actions built at startup and scoped to the tenant (#708).
- RBAC and UI truthfulness: the sidebar shows the real plan and role (#698, #699),
  read-only members no longer get create controls the server refuses (#740), every
  business role can hold the realtime stream (#742), `events:read` in the permission
  catalogue (#618), notifications open their subject (#733), `/threat-intel` redirect
  (#738).
- Infrastructure: `docker-up` starts a stack you can sign up to (#648); the backend cache
  reaches Redis in Docker (#729); no `record not found` error logs from a healthy backend
  (#617).
- Accessibility: unnamed selects and unreachable scroll regions (#646), tint contrast
  pairs (#460).
- Pending invitations counted against the caller's clock (#418).

### Deprecated
- Three legacy SSE endpoints now carry RFC 8594 `Deprecation`, `Sunset` and `Link` headers and are
  **removed in 1.2.0** (sunset **2026-12-02**) — see
  [docs/API_REFERENCE.md](docs/API_REFERENCE.md#deprecated-endpoints) and #527:
  - `GET /mitigations/events` → `GET /realtime/events?aggregates=mitigation`. This one matters:
    it accepts the access token as a **query parameter**, so the credential reaches access logs,
    proxy logs and browser history. The replacement uses the HttpOnly session cookie.
  - `GET /scanner/events` → `GET /scanner/jobs` (**polling**; the realtime hub has no `scan.*`
    events).
  - `GET /reports/{reportId}/progress` → `GET /reports/{reportId}` (**polling**; no `report.*`
    events).

  `GET /scanner/agent/stream` — the Agent's own job channel — is **not** deprecated.

### Removed
- `frontend/src/hooks/useSSE.ts`, a generic SSE hook with no consumers, superseded by
  `src/lib/realtime.ts`.

## [1.1.0-rc.3] - 2026-08-25

> Supersedes `v1.1.0-rc.2`, which was tagged but never published: the release
> workflow builds and tests from the tag, and three cache tests failed on a
> runner with no Redis alongside them. Those tests now skip when Redis is absent
> instead of reporting a missing dependency as a defect. The tag `v1.1.0-rc.2`
> is left in place — a pushed tag is never moved or deleted (see
> [docs/VERSIONING.md](docs/VERSIONING.md) §4) — and simply has no release
> attached to it.

### Milestone

**Wave 0 — "Audit & proof" is complete** (8/8 issues, W0-01 → W0-07). The wave did
not add product surface for its own sake: it established what is actually true
about the system and closed the gaps the audit found. Waves 1–8 remain open, so
this is a release candidate, not a GA.

### Added
- **W0-07 — Real-time event hub and stream contracts.** One authenticated,
  tenant-scoped SSE backbone (`GET /api/v1/realtime/events`) replacing three
  hand-rolled streams and two WebSocket clients that pointed at endpoints the
  backend never served. A versioned event envelope (id, version, tenant,
  aggregate, per-tenant sequence, correlation and causation ids) over a closed
  catalog of 25 canonical domain events across risks, assets, vulnerabilities,
  incidents, controls, assessments and mitigations. Events are appended to a
  durable per-tenant ordered log (`realtime_events`, migration `0059`) before
  delivery, so a reconnect replays from a cursor instead of refetching
  everything; a cursor older than the retained window gets an explicit resync
  rather than a silent partial replay. Delivery is **at-least-once** with
  server-minted ids and idempotent consumers — exactly-once is not claimed.
  Backpressure is real: bounded per-subscriber buffers, per-instance and
  per-tenant connection limits, and a coalescing resync instruction instead of
  blocking. Stream authorization is two-layered (`events:read` to hold a stream,
  then the read permission of each event's own aggregate to receive it). New
  permission `events:read`, 18 Prometheus series with no unbounded labels, and 8
  alert rules built around the hub's real failure mode — silence. The frontend
  gets one shared transport per tab with jittered backoff, a liveness watchdog,
  duplicate suppression, sequence-gap detection and tenant-switch teardown.
  See [docs/W0-07_REALTIME_EVENT_HUB.md](docs/W0-07_REALTIME_EVENT_HUB.md).
- **W0-04 — Real organization member management.** Inviting a colleague is now an
  invitation: a hashed, expiring, revocable token, instead of `POST /rbac/members`
  creating the account immediately and handing the administrator a temporary
  password to relay by hand. Membership gains an explicit lifecycle (status plus
  the timestamps that say when it changed), backfilled from the boolean that used
  to be the only signal (migration `0058`).
  See [docs/W0-04_ORGANIZATION_MEMBER_MANAGEMENT.md](docs/W0-04_ORGANIZATION_MEMBER_MANAGEMENT.md).
- **Financial risk quantification** (FAIR + Monte Carlo).
- **Open-core entitlements and billing**: a plan matrix enforced in the backend,
  with telemetry consent and a cancelable organization-erasure grace period.
- **Attack-surface module**, and **compliance evidence crosswalks + reporting**.

- **Guided onboarding to the Aha moment.** After signup (email + password only),
  the dashboard runs a 4-step action-driven checklist — create your first risk (the
  Aha), import a framework, invite a teammate, personalize your workspace — with a
  progress bar. Steps auto-tick from real data; the current step is emphasized so no
  one faces an empty dashboard; completion celebrates (reduced-motion-safe confetti).
  Post-Aha personalization (theme + accent) is an inline ghost edit. New
  `features/onboarding`; replaces the single dismissible banner. (UX-01/07/13/17/32.)
- The founder's latest UX guidelines are folded into the living rollout plan
  (`docs/UI_IA_ROLLOUT_PLAN.md`, section 1bis): a sequenced backlog of shared
  primitives (undoable delete, impact radiography, contextual hints, ProgressState,
  categorized notifications, keyboard shortcuts, time-travel, premium peek), each
  mapped to screens and priority — to apply UI_ELEVATION + IA_NAVIGATION screen by
  screen without regressing the healthy surface.
- **End-to-end test harness + UX audit (Gate 0).** A runnable Playwright workspace at
  the repo root (`package.json`, `playwright.config.ts`, `tests/e2e/`) that drives the
  real app: a deterministic seed (`scripts/seed-e2e.mjs` + `dev/fixtures/e2e-dataset.json`),
  API-minted `storageState` auth (no UI login except `auth.login.spec.ts`), and suites
  `smoke.routes` (every route in `App.tsx`/`navModel.ts`), `journey.newcomer`,
  `journey.settings`, `journey.rbac`, `a11y` (axe-core WCAG 2.1 AA) and a real risk
  `workflows` journey. **Local run: 95 passed / 0 failed / 19 fixme** (chromium +
  Mobile Chrome); 42 routes render (0 broken), time-to-value measured at 3.7 s. Broken
  flows are written and quarantined `test.fixme` with a bug id. E2E workflow rebuilt on
  v4/v5 actions (Node 20, PR-blocking chromium + Mobile Chrome, nightly firefox/webkit,
  route×status job summary); `ci.yml` gains a blocking `frontend-typecheck` job. Stable
  `data-testid`s added (attribute-only): `login-*`, `nav-*`, `app-main`, `settings-tab-*`.
  Deliverables: [docs/UX_CHARTER.md](docs/UX_CHARTER.md), [docs/UX_AUDIT_2026-07.md](docs/UX_AUDIT_2026-07.md),
  [docs/IA_NAVIGATION_PROPOSAL.md](docs/IA_NAVIGATION_PROPOSAL.md),
  [docs/UI_ELEVATION_PROPOSAL.md](docs/UI_ELEVATION_PROPOSAL.md) (+ 3 mockups),
  [docs/DOCS_INVENTORY.md](docs/DOCS_INVENTORY.md).
- **Single-source versioning.** The root `VERSION` file is the sole source of truth,
  propagated to the Go binary (via `-ldflags` → reported by `GET /api/v1/health`), the
  Helm chart (`version`/`appVersion`) and the frontend (`package.json`) through
  `make sync-version` / `make check-version`. Tag convention `vX.Y.Z[-rc.N]` (SemVer 2.0.0)
  with a tag-triggered release workflow that fails when the tag and `VERSION` diverge.
  See [docs/VERSIONING.md](docs/VERSIONING.md).
- **M4 — Official compliance report (PDF, 1-click).** New `GET /compliance/frameworks/{id}/report?locale=fr|en`
  streams a print-ready PDF for a framework: cover identity (organization, framework, date, requester),
  executive summary (compliance %, per-status breakdown, progress bar) and a paginated controls table
  (reference, name, colored status, evidence count, source citation). All data strictly tenant-scoped.
  Pure renderer in `backend/pkg/report` (fully unit-tested, no DB/HTTP), `GenerateComplianceReportUseCase`
  in the application layer, `CountEvidencesByFramework` repo method (single grouped query), and a
  "PDF report" button on the Compliance page (FR/EN). Serves the COBAC/BCEAO/ISO one-click statement goal.

- **Member invite** (`POST /rbac/members`, admin): provision a real tenant member
  (user + organization_member) with an org role and an optional business-role preset;
  returns a one-time temporary password. Frontend invite modal on `/settings/roles`.

### Changed
- **W0-06 — The home Security Command Center reads what it displays.** Home is a
  dispatcher over six persona dashboards plus an executive view, which is why the
  earlier cleanups had not finished the job. Every persona now reads real tenant
  data, `/stats` gains a period and one score instead of two, the inventory is
  counted in SQL rather than in the browser, and a rejected period is refused
  instead of being answered with a dashboard.
  See [docs/W0-06_SECURITY_COMMAND_CENTER.md](docs/W0-06_SECURITY_COMMAND_CENTER.md).
- **W0-02 — Release-line integrity.** `master` is established as the single
  trustworthy release line with zero branch divergence; the P1 operational risks
  found (hardcoded `latest` image tags, unlocked AutoMigrate on boot) are recorded
  in [docs/W0-02_RELEASE_LINE_INTEGRITY.md](docs/W0-02_RELEASE_LINE_INTEGRITY.md).

- **Navigation restructured into 5 GRC intentions** (founder-ratified UX proposal).
  The sidebar is regrouped by the user's intention, in the natural order of the work —
  **Piloter → Identifier → Évaluer → Traiter → Prouver** — plus a utility group
  (7 groups/~20 items → 5 intentions + utility, UX-16/UX-07). Infrastructure and Asset
  Universe are un-flagged as "soon" (both shipped); genuine placeholders (Leaderboard,
  Simulations) are withheld from the sidebar; fake count badges removed from
  Risks/Mitigations. `docs/IA_NAVIGATION_PROPOSAL.md` + `docs/UI_ELEVATION_PROPOSAL.md`
  ratified (accent default azure, density Confort, 4K master-detail); mockups polished.

- **UI elevation lot** (`feat/ia-nav-ui-elevation`): design tokens (motion, type
  scale, spacing, radii, elevation), a persisted density system (Confort default) with
  a header control, reusable `DataTable`/`EmptyState` primitives, and confetti
  micro-victories (UX-32). See `docs/UI_ELEVATION_PROPOSAL.md` §10.

### Fixed
- **The cache tests no longer fail a CI job that has no Redis.** Three of the
  four built a client against `localhost:6379` and then asserted on the result
  unconditionally, so an absent server was reported as a defect in the cache
  service. All four now ping first and skip with the reason — which is what the
  first test in the file had always intended. This is what blocked `v1.1.0-rc.2`
  from publishing.
- **UX bug registry — all 12 audit findings** (`fix/ux-audit-bug-registry`, atomic
  commits): **OR-BUG-001/009** registration is real (3 fields → account + org
  membership → auto-login → land; fake MFA façade removed); **OR-BUG-002** first-run
  onboarding card + personalized greeting; **OR-BUG-003** invite a member into the
  tenant (`InviteMemberUseCase` + `POST /rbac/members` + invite modal; live-proven:
  invite → member logs in with tenant + business role → RBAC governs them);
  **OR-BUG-004** Settings shows real session data (org/email/time zone) + honest
  placeholders for un-built areas, with persisted "Saved ✓" preference toggles;
  **OR-BUG-005** sidebar shows the real org name + real cyber score (was hardcoded
  72/"Banque Atlantique"); **OR-BUG-006** scan preview no longer retry-spams the
  console; **OR-BUG-008** auth rate limit relaxed 5/15min → 15/5min; **OR-BUG-010**
  plain-language glossary tooltips for CVE/KEV/EPSS/CVSS; **OR-BUG-011/012** WCAG 2.1
  AA — axe-core reports 0 serious/critical on the 6 key screens (labels + contrast
  fixed, a11y gate un-quarantined).

### Removed
- **W0-05 — Deceptive UI.** Fixtures, placeholders and dead-end actions removed
  across the app, with the audit and the disposition of every finding recorded in
  [docs/W0-05_DECEPTIVE_UI_AUDIT.md](docs/W0-05_DECEPTIVE_UI_AUDIT.md). Two dead
  WebSocket clients and the unrouted page that consumed them go with W0-07.

### Security
- **W0-03 — Authentication security baseline.** The identity surface — login,
  registration, MFA, OAuth2, SAML2, access tokens, refresh-token rotation,
  revocation, logout, sessions, recovery and tenant switching — is verified and
  hardened, with its actual state recorded rather than assumed. Refresh-token
  family rotation (migration `0057`), MFA backup codes moved to a CSPRNG, and
  HttpOnly cookie sessions with double-submit CSRF.
  See [docs/W0-03_AUTHENTICATION_SECURITY_BASELINE.md](docs/W0-03_AUTHENTICATION_SECURITY_BASELINE.md).
- **Tenant-isolation sweep**: every route audited against the tenant predicate,
  with regression probes for the leaks it found.
- The audit trail is made append-only (migration `0055`).

### Known limitations

- The real-time hub is an event log, **not a transactional outbox**: the row is
  appended after the business transaction commits, so a crash in that window
  loses the event. Survivable because the stream is never the source of truth and
  every client has a resync path.
- Stream authorization happens at connect, so a session revoked mid-stream keeps
  its stream until it ends (2 h ceiling) or the client reconnects.
- No load test was run against the real-time hub, so no capacity figure is
  claimed.

## [1.1.0-rc.1] - 2026-07-23

> Release-candidate hardening pass (branch `release/hardening-rc1`, stacked on
> `release/1.0-rc1`). Focus: reliability, security, multi-tenant isolation,
> performance and a green test suite — no new features.
>
> ⚠️ Versioning note: GitHub already carries `1.0.0`–`1.0.8` tags/releases that
> predate most of the current product. They do **not** reflect today's feature
> set. This RC proposes restarting the line at `1.1.0-rc.1`; the stale releases
> should be curated/relabelled (see `docs/RC_HARDENING_REPORT.md`).

### Security
- **Cross-tenant leak fixed** in bulk operations: `/bulk-operations` ran
  delete/update/export/assign filtered only by a user-supplied query with no
  `tenant_id` scope (a bulk delete could hit every tenant). Now tenant-scoped
  end-to-end with isolation tests. (Earlier RC also fixed the analytics/dashboard
  aggregation leak.)
- **Rate limiter is now Redis-backed** so brute-force protection on
  `/auth/login|register|refresh` holds across a horizontally-scaled deployment
  (was per-instance in-memory); degrades gracefully to in-memory if Redis is down.
- Legacy HS256 `/auth/legacy/*` login surface removed; default `admin123` seed
  refused in production (earlier RC).

### Performance
- **Route-based code splitting** (React.lazy): initial JS bundle 1.56 MB → ~651 kB.
- **Composite DB indexes** (migration 0039) on hot `(tenant_id, …)` access paths
  across risks/vulnerabilities/assets/compliance_controls/incidents/mitigations/audit_events.

### Fixed
- **Edit-Risk "Save" was broken in production**: the form declared `tags` as an
  array while the input produced a string, so Zod rejected every save. Fixed.
- Frontend test suite: 7 failing tests + 2 unloadable files → **43 passed / 0 failed**.
- `TestRiskCRUDFlow` green (sqlite DDL re-synced) — backend **36 pkg OK / 0 FAIL**.

### Accessibility
- Shared `Input` now associates `<label>`/`<input>` (screen readers + testability).

### RBAC
- Real per-route `PermissionRoute` guard wired from the shared nav permission map.

### Removed (dead code / duplicates)
- `ai_risk_predictor_service.go` (never wired), tenant-blind `risk_repo.go`,
  duplicate `components/CreateRiskModal.tsx`, dead `shared/fixtures.ts`, and two
  legacy Jest test files.

## [1.0.4] - 2025-01-02

### Added
- Analytics dashboard with real-time risk metrics
- Gamification system with badges and progress tracking
- Custom fields framework (5 field types supported)
- Bulk operations for risks and mitigations
- Advanced search and filtering capabilities
- Risk timeline view (audit trail)

### Improved
- Dashboard load time reduced by 40%
- Mobile responsive design across all pages
- API response times optimized
- Documentation structure reorganized

### Fixed
- API token expiration edge cases
- Search filter bugs with special characters
- Session handling on token refresh
- Mobile menu navigation issues

## [1.0.3] - 2024-12-15

### Added
- OAuth2/SAML2 SSO support (Google, GitHub, Azure AD)
- Role-Based Access Control (RBAC)
- API token management (create, revoke, rotate)
- Comprehensive audit logging

### Improved
- Authentication flow security
- Permission matrix granularity
- Database query optimization

### Fixed
- JWT token refresh bugs
- Permission check edge cases

## [1.0.2] - 2024-12-01

### Added
- Mitigation sub-actions (checklist items)
- Asset relationship management
- Risk scoring engine improvements

### Fixed
- Soft-delete cascade issues
- Asset linking bugs

## [1.0.1] - 2024-11-15

### Added
- Basic CRUD for risks, mitigations, assets
- Initial dashboard
- Documentation structure

## [1.0.0] - 2024-11-01

### Added
- Initial release
- Core risk management features
- React frontend + Go backend
- Docker Compose setup
- Basic authentication

---

[Unreleased]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.5...HEAD
[1.1.0-rc.5]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.4...v1.1.0-rc.5
[1.1.0-rc.4]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.3...v1.1.0-rc.4
[1.1.0-rc.3]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.1...v1.1.0-rc.3
[1.1.0-rc.1]: https://github.com/opendefender/OpenRisk/compare/v1.0.8...v1.1.0-rc.1
[1.0.4]: https://github.com/opendefender/OpenRisk/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/opendefender/OpenRisk/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/opendefender/OpenRisk/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/opendefender/OpenRisk/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/opendefender/OpenRisk/releases/tag/v1.0.0
