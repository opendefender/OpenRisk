---
name: project-751-phase4-defects
description: #751 phase 4 (notifications/banners) live-pass findings — light-theme contrast failures on --high/--critical 16% tints, and Playwright MCP round-trip latency invalidating naive manual timing checks
metadata:
  type: project
---

Phase 4 of #751 (notifications & alerts: bell badge, panel, sonner toasts,
BannerStack) shipped on branch `feat/751-phase4-notifications`
(commits cbd3d64/c1be1e8/21c6920/adb7d5d). Unit coverage (818/818 vitest) is
strong and directly maps to the spec's 10 required tests — no tautological
class-list assertions found; tests check computed attributes (`data-armed`,
`data-visible`, node identity, `toHaveAccessibleName`) and real sonner DOM.

**Real defect found in the live pass (light theme only):** text-on-16%-tint
contrast fails AA (4.5:1) for two banner surfaces, using the exact token
values from `frontend/src/index.css` (`--high`/`--critical` mixed 16% over
`--surface-0`, light theme):
- DemoBanner (`--high` `#9a5b00` on ~`#e7dcc9`): **3.99:1** — fails.
- OfflineBanner "Offline" state (`--critical` `#c81e14` on ~`#efd2cc`): **4.03:1** — fails.
- OfflineBanner "Unstable connection" state (`--medium`): 5.26:1 — passes.
- Dark theme: `--high` 6.95:1, `--critical` 4.68:1 (passes but tight — watch
  if `--critical` or `--surface-0` shift even slightly), `--medium` 9.19:1 —
  all pass.
- Badge (`--fg-on-solid` on `--accent-solid`): passes both themes (6.49:1
  dark, 7.70:1 light).
- Toast title/description on `--bg-elevated`: passes both themes comfortably
  (title ≥16:1, description ≥8:1 both themes) — sonner's classNames restyle
  did not touch text colour, so it inherits high-contrast tokens correctly.

**Why it matters:** light theme is not a secondary target for this product —
France/Belgium/Maghreb/Canada enterprise buyers audit accessibility, and
`--high`/`--critical` are semantic severity colours reused everywhere, not
just banners. A fix likely needs either a darker `--high`/`--critical` in
light mode or a higher tint percentage for banner-scale text (12.5px is
already the small end of the AA large-text threshold, so this can't lean on
the 3:1 large-text carve-out).

**How to apply:** when reviewing any future banner/alert surface that reads
`--high`/`--critical`/`--medium` at small text sizes on a 16% (or lower) tint
background, recompute the ratio in light theme specifically before signing
off — dark theme passing is not evidence light theme passes, these are
independently authored hex values in `index.css`, not a shared formula.

**Playwright MCP tool round-trip latency invalidates naive manual timing
checks.** Each `browser_click` → separate `browser_evaluate` round trip in
this environment routinely costs multiple real seconds — enough to blow past
a 4000ms toast duration between "click" and "read the DOM in the next tool
call". Symptom: toasts you just fired appear already gone, or a badge
animation state looks stale, with no actual defect. Fix: do the click AND the
timing-sensitive read inside **one** `browser_evaluate` call using an
in-page `performance.now()` timer and a polling `while` loop (or a plain
`setTimeout` await) rather than stepping across separate tool calls. This is
how the 2000ms offline-hold (~1958ms) and the 4000ms success-toast
(~4209ms incl. exit transition) were confirmed for real in this pass — cross
-checked against the vitest fake-timer tests, not used as the sole source of
truth for exact ms.

**Harness pattern that worked:** temporary `frontend/qa751p4.{html,tsx}`
(sibling to `e2e/visual/harness.tsx`, never under `src/`), overriding
`api.defaults.adapter` with a hand-rolled synchronous responder (no MSW in
this repo) keyed by URL, mutable `state` object, buttons dispatching real
`window` events (`offline`/`online`) and calling the real `useToast()` hook.
Deleted immediately after the pass; confirmed via `git status --short` that
nothing under `frontend/src/` was left behind.
