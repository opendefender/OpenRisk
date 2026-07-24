# Changelog

All notable changes to OpenRisk will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
Git tags use the `vMAJOR.MINOR.PATCH[-rc.N]` convention; see [docs/VERSIONING.md](docs/VERSIONING.md).

## [Unreleased]

### Added
- **Guided onboarding to the Aha moment.** After signup (email + password only),
  the dashboard runs a 4-step action-driven checklist — create your first risk (the
  Aha), import a framework, invite a teammate, personalize your workspace — with a
  progress bar. Steps auto-tick from real data; the current step is emphasized so no
  one faces an empty dashboard; completion celebrates (reduced-motion-safe confetti).
  Post-Aha personalization (theme + accent) is an inline ghost edit. New
  `features/onboarding`; replaces the single dismissible banner. (UX-01/07/13/17/32.)
- **Page-by-page UI/IA rollout plan** (`docs/UI_IA_PAGE_ROLLOUT.md`): applies
  UI_ELEVATION + IA_NAVIGATION + the founder's UX guidelines screen by screen, with
  status and priority, and a sequenced backlog of shared primitives (undo-delete,
  impact radiography, contextual hints, categorized notifications, role dashboards).
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

### Changed
- **Navigation restructured into 5 GRC intentions** (founder-ratified UX proposal).
  The sidebar is regrouped by the user's intention, in the natural order of the work —
  **Piloter → Identifier → Évaluer → Traiter → Prouver** — plus a utility group
  (7 groups/~20 items → 5 intentions + utility, UX-16/UX-07). Infrastructure and Asset
  Universe are un-flagged as "soon" (both shipped); genuine placeholders (Leaderboard,
  Simulations) are withheld from the sidebar; fake count badges removed from
  Risks/Mitigations. `docs/IA_NAVIGATION_PROPOSAL.md` + `docs/UI_ELEVATION_PROPOSAL.md`
  ratified (accent default azure, density Confort, 4K master-detail); mockups polished.

### Fixed
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

### Added
- **Member invite** (`POST /rbac/members`, admin): provision a real tenant member
  (user + organization_member) with an org role and an optional business-role preset;
  returns a one-time temporary password. Frontend invite modal on `/settings/roles`.

### Changed
- **UI elevation lot** (`feat/ia-nav-ui-elevation`): design tokens (motion, type
  scale, spacing, radii, elevation), a persisted density system (Confort default) with
  a header control, reusable `DataTable`/`EmptyState` primitives, and confetti
  micro-victories (UX-32). See `docs/UI_ELEVATION_PROPOSAL.md` §10.

### Planned
- Board Report mensuel (IA, human-in-the-loop, FCFA) — the second half of M4
- Multi-tenant support
- Mobile app (React Native)
- Slack/Teams notifications
- Jira integration

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

[Unreleased]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.1...HEAD
[1.1.0-rc.1]: https://github.com/opendefender/OpenRisk/compare/v1.0.8...v1.1.0-rc.1
[1.0.4]: https://github.com/opendefender/OpenRisk/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/opendefender/OpenRisk/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/opendefender/OpenRisk/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/opendefender/OpenRisk/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/opendefender/OpenRisk/releases/tag/v1.0.0
