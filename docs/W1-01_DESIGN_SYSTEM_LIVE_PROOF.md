<!-- Copyright (c) 2026 OpenDefender Contributors
     SPDX-License-Identifier: BUSL-1.1 -->

# W1-01 — Design System Live Proof

What was actually executed, and what it returned. Nothing here is predicted: every
number is copied from a command run on this branch, and everything that could not
be run is marked `NOT AVAILABLE` rather than assumed to pass.

## Environment

| | |
| --- | --- |
| OS | Ubuntu 26.04 LTS · Linux 7.0.0-30-generic |
| Node | v26.7.0 |
| Playwright | 1.61.1 |
| Vite | 7.2.4 · TypeScript 5.9.3 · Tailwind 3.4.18 |
| Frontend dev server | `vite --port 5199 --strictPort`, started fresh for this run |
| Backend | **Not started.** Not required: the visual and axe suites run against a static harness by design, and the one app route reachable without an API (`/login`) was used for the real-app captures. |

> A stale dev server invalidated an earlier round of this proof — see
> *Known Limitations* item 1. The suite now takes `OPENRISK_BASE_URL` so it can be
> pointed at a server you just started, and this run used one.

## Commit SHA

```
branch  feat/w1-01-openrisk-design-system
head    3d34514
base    master (9154233)
commits 82 (14 from this wave; the rest are the branch this was stacked on)
```

## Browser / Viewport Matrix

| Engine | Available | Used |
| --- | --- | --- |
| Chromium 1234 | yes | all suites |
| Firefox | **NOT AVAILABLE** — not installed in this environment (`~/.cache/ms-playwright` holds chromium only) | — |
| WebKit | **NOT AVAILABLE** — same | — |

| Viewport | White | Dark |
| --- | --- | --- |
| 1440×900 desktop | PASS | PASS |
| 1280×800 (snapshot default) | PASS | PASS |
| 834×1112 tablet | PASS | PASS |
| 390×844 narrow | PASS | PASS |

Every cell is the `no horizontal overflow` test across all five gallery pages,
plus the snapshot suite at the default viewport.

## White Mode Validation

```
node scripts/check-contrast.mjs
  LIGHT — 0 failing
```

- All text roles ≥4.5:1 on all five surfaces.
- Semantic `-text` tokens ≥4.5:1 on their `-surface` and on `--surface-2`.
- `--focus-ring-color` ≥3:1 on all five surfaces (this is `--accent-500` in
  light; `--accent-400` measured 2.73:1 on the canvas and was rejected).
- `--border-control` ≥3:1 on surfaces 1–3.
- All 8 chart series ≥3:1 on `--surface-1` using the light ramp.
- `--accent` ≥4.5:1 on all surfaces, for **both** accent variants.
- axe: 0 serious/critical across 5 gallery pages.

Evidence: `docs/ui/w1-01/app-login-desktop-light.png`,
`docs/ui/w1-01/app-login-narrow-light.png`,
`frontend/e2e/visual/design-system.spec.ts-snapshots/ds-*-light-chromium-linux.png`.

## Dark Mode Validation

```
node scripts/check-contrast.mjs
  DARK — 0 failing
```

Same pair set, dark values. axe: 0 serious/critical across 5 gallery pages.

Evidence: `docs/ui/w1-01/app-login-desktop-dark.png`,
`docs/ui/w1-01/app-login-narrow-dark.png`,
`frontend/e2e/visual/design-system.spec.ts-snapshots/ds-*-dark-chromium-linux.png`.

## Component Validation

The matrix the brief asks for. **A cell is only ✅ if a test or a snapshot
actually exercises that state** — not if the code merely supports it.

| Primitive | White | Dark | Hover | Focus | Active | Disabled | Loading | Error | Empty | A11y |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Button | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ✅ | ✅ | ✅ | n/a | ✅ |
| Input / Textarea / Select | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ✅ | ✅ | ✅ | n/a | ✅ |
| Badge | ✅ | ✅ | n/a | n/a | n/a | n/a | n/a | ✅ | ✅ | ✅ |
| Table (DataTable) | ✅ | ✅ | ⚠️ | ⚠️ | ⚠️ | n/a | ✅ | ✅ | ✅ | ⚠️ |
| Modal | ✅ | ✅ | ⚠️ | ✅ | n/a | ✅ | ✅ | n/a | n/a | ✅ |
| Drawer | ✅ | ✅ | ⚠️ | ✅ | n/a | n/a | n/a | n/a | n/a | ✅ |
| Chart palette | ✅ | ✅ | n/a | n/a | n/a | n/a | n/a | n/a | ⚠️ | ✅ |
| Graph | ✅ | ✅ | n/a | n/a | ✅ | n/a | n/a | n/a | ⚠️ | ✅ |
| Empty | ✅ | ✅ | n/a | ✅ | n/a | n/a | n/a | n/a | ✅ | ✅ |
| Loading | ✅ | ✅ | n/a | n/a | n/a | n/a | ✅ | n/a | n/a | ✅ |
| Error | ✅ | ✅ | ⚠️ | ✅ | n/a | n/a | ✅ | ✅ | n/a | ✅ |
| Permission denied | ✅ | ✅ | n/a | ✅ | n/a | n/a | n/a | n/a | ✅ | ✅ |
| Tabs | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | n/a | n/a | n/a | ✅ |
| Tooltip | ⚠️ | ⚠️ | ⚠️ | ⚠️ | n/a | ✅ | n/a | n/a | n/a | ⚠️ |
| Audit entry | ✅ | ✅ | n/a | n/a | n/a | n/a | n/a | n/a | n/a | ✅ |

Legend — ✅ covered by a snapshot, an axe pass or a unit test ·
⚠️ implemented and reviewed, **not** exercised by an automated check ·
n/a state does not exist for this primitive.

The ⚠️ cells are honest gaps, and they cluster in two places:

- **Hover and active** are not screenshotted anywhere. The snapshots are static;
  proving a hover state needs a `page.hover()` per element. Reviewed by reading
  the classes, not proven.
- **Tooltip** is the least-covered primitive: it has unit-level structure but no
  gallery page, because a tooltip only exists while hovered and a static
  screenshot cannot hold one open.
- **DataTable a11y** is inherited from its own suite (25 tests in
  `shared/datatable/__tests__`), not re-verified here.

## Product Screen Validation

| Screen area | Theme parity | Migrated off legacy primitives | Snapshotted |
| --- | :-: | :-: | :-: |
| Login / Register | ✅ captured both themes, both viewports | ✅ | ✅ (manual capture) |
| Assets | token-driven | ✅ | ❌ |
| Compliance | token-driven | ✅ | ❌ |
| Mitigations | token-driven | ✅ | ❌ |
| Risks | token-driven | ✅ | ❌ |
| Users / Roles / Tokens | token-driven | ✅ | ❌ |
| Settings | token-driven | ✅ | ❌ |
| Reports | token-driven | ✅ | ❌ |
| Incidents | token-driven | ✅ | ❌ |
| Vulnerabilities, Governance, Automation, Dashboards | token-driven | via `shared/ui` adapters | ❌ |

"token-driven" means the screen's colour now resolves through the theme tokens,
so a theme flip retints it — it does **not** mean the screen was visually
inspected in both themes. Only Login was, because it is the only route this
environment can reach without a backend.

## Accessibility Validation

```
playwright test e2e/visual/ --reporter=line
  36 passed
```

Of which **14 are axe runs** — 5 gallery pages × 2 themes, plus 2 overlays × 2
themes — at `wcag2a, wcag2aa`, failing on any serious or critical violation.

**Two real defects were found by this suite and fixed:**

1. `color-contrast` — every `text-accent` in the product measured **3.65:1** on
   white. `--accent` was declared once, in `index.css`, and used unchanged in
   both themes. Fixed by giving each accent variant a per-theme definition and
   extending `check-contrast.mjs` to verify each variant in each theme.
2. `aria-valid-attr-value` — `Tabs` set `aria-controls` on every tab pointing at
   panel ids that could not exist (the id was generated privately inside `Tabs`
   while `TabPanel` took its own from the caller, and inactive panels are not
   rendered). Fixed by requiring an explicit shared `id` and only referencing a
   panel from the active tab.

```
vitest run
  Test Files  1 failed | 28 passed (29)
  Tests       1 failed | 284 passed (285)
```

The single failure is `App.integration.test.tsx › should render Login page when
not authenticated`. **Verified pre-existing**: it fails identically on the base
commit (`9154233`) in a clean worktree — `1 failed | 2 passed (3)`.

Of the 285, **33 are new** design-system tests covering accessible names, the
description/error announcement order, `aria-invalid` + `role="alert"`, required
in the accessible name, focus trap and focus restoration, scroll lock/restore,
the tabs keyboard pattern, and the category-vs-severity palette separation.

## Contrast Validation

```
node scripts/check-contrast.mjs
  124 pairs checked, 0 failing.
  All token pairs meet their WCAG target in both themes.
```

Baseline was **68 pairs**. The 56 added by this wave are the non-text checks
(WCAG 1.4.11) that never existed: chart series, focus ring on every surface,
control borders, graph edges, and the accent per theme × per variant.

Four failures were surfaced by adding them and were fixed by changing tokens,
not by lowering the bar:

| Failure | Was | Now |
| --- | --- | --- |
| `--border-strong` as a control edge | 1.70:1 dark / 1.65:1 light | new `--border-control`, 3.0+ both |
| `--graph-edge` / `--graph-node-stroke` | 1.36:1 / 1.65:1 | 3.0+ both |
| `--accent-400` as the light focus ring | 2.73:1 on canvas | light ring is `--accent-500` |
| `--accent` as text on white | 3.65:1 | per-theme variant, 4.5+ |

## Keyboard Validation

Automated:

- `focus is visible and moves through the controls in order` — asserts
  `:focus-visible` is visible and its computed `outline-style` is not `none`.
- `tabs are operable with the arrow keys` — ArrowRight moves selection.
- Unit: focus enters a Modal, Tab is trapped across 8 presses, Escape closes,
  focus **returns to the trigger**; same contract asserted for Drawer.
- Unit: exactly one tab carries `tabindex="0"` (roving tabindex); arrows wrap;
  End jumps to the last tab.

Not automated: a full manual screen-reader pass (NVDA/VoiceOver). **NOT
AVAILABLE** in this environment.

## Reduced Motion Validation

```
playwright test --grep "reduced motion"   (test.use({ reducedMotion: 'reduce' }))
  content is present and visible with animation disabled — PASS
```

The assertion is deliberately about presence, not about absence of animation:
the stylesheet already kills animation globally under
`prefers-reduced-motion: reduce`, and the failure mode that actually ships is an
entrance implemented as "start at `opacity: 0`", which leaves the element
invisible forever once the animation is off. The test asserts the button is
visible and at `opacity: 1`.

## Responsive Validation

```
no horizontal overflow — desktop — light   PASS
no horizontal overflow — desktop — dark    PASS
no horizontal overflow — tablet  — light   PASS
no horizontal overflow — tablet  — dark    PASS
no horizontal overflow — narrow  — light   PASS
no horizontal overflow — narrow  — dark    PASS
touch targets clear the WCAG 2.5.8 minimum PASS
```

Each overflow test walks all five gallery pages at that viewport and asserts
`scrollWidth - clientWidth <= 0`. The target-size test measures every rendered
`<button>` at 390px against the 24×24 floor (the `link` variant is exempt under
the standard's inline exception); smallest control is 28px.

Real app at 390px: `docs/ui/w1-01/app-login-narrow-{light,dark}.png`, no
overflow, 0 console errors.

## Visual Regression

```
playwright test e2e/visual/ --reporter=line
  36 passed (19.0s)
```

| Suite | Coverage |
| --- | --- |
| `design-system.spec.ts` | 5 gallery pages × 2 themes = 10 snapshots, + 10 axe, + keyboard/motion/responsive/target/theme-switch |
| `overlays.spec.ts` | 2 overlays × 2 themes = 4 snapshots, + 4 axe, + coverage guard + staleness guard |

Stability: **3 consecutive clean runs** after adding the `document.fonts` wait.
Before it, snapshots failed intermittently because the web fonts are fetched at
runtime and a screenshot could catch the fallback face.

The coverage guard fails CI when an overlay exists in the app and is neither
registered nor listed in `NOT_YET_REGISTERED`; the staleness guard fails when an
exemption no longer matches any file. **57 overlays are currently exempt** — the
honest record of what has no snapshot. The list may only shrink.

## Performance

| Measurement | Result | Method |
| --- | --- | --- |
| Theme flip — style recalculation | **p50 1.60ms · p95 2.10ms · max 2.50ms** (n=40, 172-element page) | Set `data-theme`, force sync style resolution via `getComputedStyle` + `offsetHeight`, measure |
| Theme flip — no reload, both directions | PASS | `flipping the theme attribute retints the page, both directions` |
| JS bundle (eager) | **241.7 KB / 250 KB budget** — within, unchanged by this wave | `node scripts/check-bundle-budget.mjs` |
| CSS bundle | **84.13 kB raw / 16.14 kB gzip** (was 74.30 / 14.08) | `vite build` |
| Build time | 6.4s | `vite build` |

The CSS grew ~10 kB raw / ~2 kB gzipped. That is the new utility surface — the
type scale with per-step line heights, named z-layers, the chart palette, motion
tokens. It buys the removal of ~1500 inline decisions' worth of *future* CSS and
is a fixed, one-time cost rather than one that scales with screens.

Animation cost: every transition is on `transform`, `opacity`, `color`,
`background-color`, `border-color` or `filter`. None touches layout, so a
200-row table with an action button per row does not reflow on hover.

## Commands Executed

```bash
# from frontend/
./node_modules/.bin/tsc -b --force
node scripts/check-contrast.mjs
./node_modules/.bin/vitest run
./node_modules/.bin/eslint .
./node_modules/.bin/vite build
node scripts/check-bundle-budget.mjs
node scripts/overlay-inventory.mjs
node scripts/migrate-primitives.mjs          # the one-shot codemod
./node_modules/.bin/vite --port 5199 --strictPort   # fresh server for the suites
OPENRISK_BASE_URL=http://localhost:5199 \
  ./node_modules/.bin/playwright test e2e/visual/
```

## Results

| Check | Result |
| --- | --- |
| Typecheck (`tsc -b --force`) | **PASS** — 0 errors |
| Token contrast (124 pairs, both themes) | **PASS** — 0 failing |
| Unit tests | **PASS with a known pre-existing failure** — 284/285; the 1 failure reproduces on the base commit |
| ESLint | **PASS (no regression)** — 358 problems vs **360** on the base commit; no new rule fires |
| Visual regression (36 tests) | **PASS** — 3 consecutive clean runs |
| Accessibility (axe, wcag2a+wcag2aa, 14 pages) | **PASS** — 0 serious/critical |
| Keyboard | **PASS** |
| Reduced motion | **PASS** |
| Responsive (3 viewports × 2 themes) | **PASS** |
| Target size (WCAG 2.5.8) | **PASS** |
| Theme switching | **PASS** — 1.60ms p50, both directions, no reload |
| Production build | **PASS** — 6.4s |
| Bundle budget | **PASS** — 241.7 KB ≤ 250 KB |
| Backend / API tests | **NOT APPLICABLE** — no backend change in this wave |
| Firefox / WebKit | **NOT AVAILABLE** — engines not installed |
| Manual screen-reader pass | **NOT AVAILABLE** in this environment |
| Live product-screen walkthrough beyond `/login` | **BLOCKED** — needs a running backend |

## Screenshots / Evidence

All generated by the commands above on this branch. None is a mock-up.

**Real application** (`docs/ui/w1-01/`) — the running app at `/login`, the one
route reachable without a backend:

| File | What it shows |
| --- | --- |
| `app-login-desktop-light.png` | 1440×900, White. Flat accent button — no gradient, no glow. 0 console errors. |
| `app-login-desktop-dark.png` | 1440×900, Dark. Same layout, cool-navy canvas. 0 console errors. |
| `app-login-narrow-light.png` | 390×844, White. No overflow. |
| `app-login-narrow-dark.png` | 390×844, Dark. No overflow. |

**Design system** (`frontend/e2e/visual/design-system.spec.ts-snapshots/`) — the
reference images the suite compares against, 10 files:

`ds-{controls,forms,states,charts,feedback}-{light,dark}-chromium-linux.png`

- **controls** — 5 button variants × 6 states, 3 sizes, 3 icon-only sizes, 8 badge intents, 2 badge sizes
- **forms** — 11 field configurations: required, filled, icons+loading, invalid, warning, success, disabled, read-only, select, invalid select, textarea
- **states** — empty, loading (skeleton + spinner), error with technical detail, permission denied, audit entries
- **charts** — the 8 categorical series, the 5 severity encodings, the topology graph with active/dimmed nodes
- **feedback** — the four semantic surface/text pairs and the four-step surface ramp

**Overlays** (`frontend/e2e/visual/overlays.spec.ts-snapshots/`) — 4 files, the
two confirmation dialogs in both themes, regenerated after they were rebuilt on
the `Modal` primitive.

## Known Limitations

1. **An earlier round of this proof was invalid, and that is why the tooling
   changed.** The suite had been running against a dev server left over from a
   previous session, which had loaded the *previous* `tailwind.config.js` at
   startup. Its stylesheet still carried Tailwind's default type scale, so
   `text-2xs` did not exist and a field description rendered larger than its own
   label. Every snapshot from that round was discarded. `baseURL` is now
   overridable and this run used a server started for it. The lesson is recorded
   because it will recur.
2. **Firefox and WebKit were not exercised.** Only Chromium is installed. Nothing
   here is engine-specific in a way that should matter — the system is CSS custom
   properties, flexbox and grid — but that is an argument, not evidence.
3. **Hover and active states are not snapshotted.** Reviewed by reading, not
   proven. See the ⚠️ cells in the component matrix.
4. **Product screens other than `/login` were not visually verified.** They are
   token-driven and typecheck, and their controls are migrated, but no one has
   looked at them in both themes in this environment. The remaining screens are
   the first candidates for harness registration.
5. **The bulk of arbitrary values remain**: 1518 arbitrary font sizes and 455
   arbitrary radii across the feature screens (from 1559 / 471). The lint rule is
   enforced on an allowlist that currently holds `src/shared/ds/**` plus four
   shared files. Growing that list is the unit of follow-on work.
6. **506 raw `<button>` and 181 raw `<input>` elements remain.** They retint with
   the theme but do not get the primitives' behaviour — no loading state, no
   enforced accessible name.
7. **`no-raw-colors` still exempts 76 legacy files** (58 findings), unchanged in
   either direction by this wave.
8. **The one failing unit test is not fixed.** It is pre-existing and unrelated
   to the design system; fixing it was out of scope and would have hidden the
   comparison against the baseline.
