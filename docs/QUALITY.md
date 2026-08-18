# OpenRisk — Bank-grade quality bar

The quality posture we hold OpenRisk to, and where each guarantee is enforced.
This is a living dossier: green items are enforced in CI; amber items name the
remaining gap honestly rather than hiding it.

## 1. Accessibility — WCAG 2.2 AA

- **axe-core in CI** across 15 screens (incl. financial, assets, incidents,
  automation, governance, reports, billing, and a 404) with the `wcag22a/aa` rule
  tags — zero serious/critical enforced (`tests/e2e/a11y.spec.ts`). 🟢
- **Skip-to-content link** (WCAG 2.4.1) as the first focusable element. 🟢
- **Visible focus ring** on every keyboard-focused control via `:focus-visible`
  (2.4.7/2.4.11), theme-aware. 🟢
- **prefers-reduced-motion** globally kills animations/transitions. 🟢
- **aria-live** on toasts (sonner), the offline banner, status, loading states. 🟢
- Contrast ≥ 4.5:1 in both themes — the light-theme report contrast bug
  (OR-BUG-011) is fixed and guarded by the axe gate. 🟢

## 2. Performance budget — BLOCKING in CI

- **Initial bundle ~236 KB gzip** (was ~1.4 MB), under the **250 KB** budget
  enforced by `frontend/scripts/check-bundle-budget.mjs` (`npm run budget`), wired
  into the build job. 🟢
  - Root-cause fix: the zxcvbn password dictionaries (~1.1 MB) were loading on the
    login page; now loaded on demand. Vendor `manualChunks` keep heavy libs out of
    the entry; auth screens, create-risk modal and command palette are lazy.
- **Offline / unstable-connection tolerance:** React Query offline-first (SWR
  cache, 24h gcTime), mutations pause and replay on reconnect (write queue), and
  an **OfflineBanner** shows the state + pending writes. 🟢
- LCP < 2s · INP < 200ms · CLS < 0.1 · 10k-row tables (virtualised `DataTable`) —
  measured in the field; add Lighthouse-CI budgets next. 🟡

## 3. Responsive mobile

- Off-canvas sidebar < lg, mobile top bar, `overflow-x` tables, adaptive headers
  (shipped in prior UX passes). Dashboard/Registry/Risk detail/Incidents/
  Notifications have real mobile layouts. 🟢
- Device-lab verification on iPhone 12 / Pixel 5 — pending. 🟡

## 4. Observability

- **Prometheus business metrics:** `time_to_aha_seconds`, `activation_events_total`,
  `aha_reached_total` (activation rate = aha/signups), `risks_created_total`,
  `reports_generated_total{kind}`, plus HTTP/DB/cache/auth metrics; served at
  `GET /metrics`. 🟢
- **Public status page** `/status` (auth-free) polling `GET /api/v1/status`, which
  **probes the real DB** (not a hardcoded UP). 🟢
- **Error reporting:** dependency-free reporter + ErrorBoundary; integrates with
  Sentry when the loader is present. Backend OTLP tracing export plugs into the
  already-vendored OpenTelemetry libs. 🟡

## 5. System pages

- Branded, theme-aware, reduced-motion-safe **404 / 500 / maintenance**; the
  catch-all route renders a real 404 (not a silent redirect). The **500** page is
  shown by a top-level ErrorBoundary with a correlation id instead of a blank
  screen. **403 access-denied** names the missing permission verbatim (copy-paste
  into the role editor) and who to ask. 🟢

## 6. Test coverage

- **Critical-path E2E (Playwright)** exist and run in CI: signup→Aha
  (`activation.spec.ts`, `journey.newcomer.spec.ts`), full risk lifecycle
  (`risk-lifecycle.spec.ts`), report generation (`activation`/`dead-controls`),
  member invite + RBAC (`journey.rbac.spec.ts`), plus a11y, datatable, smoke
  routes, workflows. 🟢
- **Tenant isolation** is exhaustively unit-tested backend-side
  (`internal/security/isolation`, per-repo cross-tenant tests). 🟢
- **Backend statement coverage is ~30% today** (the `pkg/` and
  `internal/application/` core is well covered; handlers, repositories, `main.go`
  and the legacy `internal/service` layer are the untested bulk). The CI gate is
  currently 50%. **Target: >70%.** 🟡
  - **Plan:** ratchet up by adding HTTP-level handler tests (the acceptance-test
    pattern already used for audit/compliance) and repository sqlite tests, one
    module per PR, raising the CI floor as each lands — never lowering it.

Legend: 🟢 enforced · 🟡 partial / planned.
