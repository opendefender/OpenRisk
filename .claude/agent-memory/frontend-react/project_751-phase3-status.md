---
name: 751-phase3-status
description: Status of issue #751 phase 3 ("Dashboard & data") on branch feat/751-phase3-dashboard — SlotReel, WidgetState cross-fade, Empty text reveal; what shipped, what was deliberately deferred, and the D-060/spec-vs-code interpretations made along the way.
metadata:
  type: project
---

Branch `feat/751-phase3-dashboard`, cut from `master` at `e1aafd5` — same
commit phase 2 (`feat/751-phase2-feedback`, see [[project_751-phase2-feedback]])
was cut from, so the two stay independently reviewable. Binding spec:
art-director + ux-designer, arbitrated, posted to the issue; **D-060 = A**
(owner): an in-house slot-reel counter, `@number-flow/react` stays banned but
the lint message now points at the in-house component.

**Shipped, four atomic signed commits:**
1. `shared/ds/SlotReel.tsx` (new) — digit-column reel. Every column's
   position is a pure function of `value` computed on every render (no
   JS-counted intermediate number ever exists), so the browser's own CSS
   transition does the animating; React state (`phase: 'settled'|'rolling'`)
   only drives the `data-rolling`/`data-settled` attributes and the edge
   mask, via React's "adjust state while rendering" pattern — see
   [[feedback_set-state-in-effect-lint]] for why a `useEffect`+`useRef`
   version of the same logic fails `eslint-plugin-react-hooks` v7's
   `set-state-in-effect` rule as a hard error. New `--stagger-step: 40ms`
   token in `primitives.css`.
2. Adopted at all four remaining `useCountUp` call sites — `DashboardPage`'s
   and `features/dashboard/shared.tsx`'s `KpiCard`, `shared/ScoreGauge.tsx`,
   `shared/ui.tsx`'s `RadialGauge` — and both `useCountUp` copies deleted.
   Both gauges' progress arc changed from a `d` path recomputed every frame
   from the counted value to a **static** full-range arc (identical to the
   track) revealed via `pathLength=1` + `strokeDasharray`/`strokeDashoffset`,
   transitioning on `--dur-panel` — no JS animation loop for the arc at all
   now, matching the RingGauge pattern that already existed in `shared/ui.tsx`.
3. `WidgetState.tsx` skeleton→content cross-fade: a 100ms grace window before
   the skeleton is allowed to show (kills the flash for cached/near-instant
   data), and a `contentShown` latch so a background refetch can never bring
   the skeleton back. The "genuinely showing skeleton" and "exiting"
   (fade-skeleton-out/fade-content-in) render paths deliberately produce the
   **identical wrapper JSX shape** — a CSS transition only plays across two
   renders of the same node, never a node's first paint, so if the two
   phases had different tree shapes the "fade" would just be an instant swap.
4. `shared/ds/Empty.tsx` title/description reveal only (dropped on the
   dashboard itself — art-director call, titles are static chrome there).
   New `or-rise-xs` keyframe (index.css), 4px rise, description a
   `--stagger-step` behind the title, each `key`ed on its own text so a
   copy change while the same `Empty` instance stays mounted replays the
   reveal instead of freezing mid- or post-animation.

**Verified:** `npx tsc -b` clean; `npx eslint <all touched files>
--max-warnings=0` clean except pre-existing debt (see below, proven via
`git stash`); full `npx vitest run`: 797/797 green; `npx vite build` clean;
`npm run budget`: 200.0 KB vs a freshly-confirmed `master` baseline of
199.9 KB (also failing the 180 KB gate, pre-existing) — +0.1 KB, inside the
+1 KB allowance.

**Pre-existing lint debt touched-but-not-fixed (confirmed via `git stash`,
same convention as [[feedback_lint-pre-existing-debt]]):**
- `DashboardPage.tsx` — 3 `react-hooks/exhaustive-deps` warnings, same lines
  before and after my diff.
- `features/dashboard/shared.tsx` and `shared/ui.tsx` — `react-refresh/only-
  export-components` errors (mixing hooks/functions with components in one
  file). `shared/ui.tsx` ALSO had `useCountUp` itself already failing
  `set-state-in-effect` on master before I touched it — removing `useCountUp`
  fixed that one specific violation as a side effect; the remaining
  fast-refresh errors are untouched, unrelated debt.

**Spec-vs-code interpretations made (stated, not escalated — all smallest-
consistent-interpretation calls):**
- "AuditDashboard/AnalystDashboard tiles, which don't animate today, are left
  as they are" (spec's own words) is factually wrong against the code — both
  render `KpiRow`/`KpiCard` from `features/dashboard/shared.tsx`, which DID
  call `useCountUp`. Followed the spec's own authoritative file:line list
  instead (which names `features/dashboard/shared.tsx:120` explicitly), so
  both dashboards get `SlotReel` for free via the shared component — not
  reverted.
- SlotReel's four mount/refetch/change bullets (spec) contained an apparent
  contradiction between "first data arrival: rolls from 0" and "cached
  navigation, mount with data already there: plain, no roll". **SUPERSEDED —
  do not reuse this reasoning.** My original call was that mount is always
  plain and "rolls from 0 on first arrival" is honestly unreachable in this
  codebase; the coordinator corrected this same day (see "Follow-up fix"
  below): the real distinction is fresh vs cached DATA, not mount vs update,
  and it needs a caller-supplied signal (`isFetchedAfterMount`) because mount
  alone can't express it. Left here only so a future session doesn't
  rediscover and re-ship the same wrong call.
- KpiCard's `locale` prop: `DashboardPage`'s KpiCard passes `localeTag(lang)`
  (matches its pre-existing `fmt`); `features/dashboard/shared.tsx`'s KpiCard
  passes no `locale` at all (matches its pre-existing bare `.toLocaleString()`
  with no `lang` prop on that component) — not widened to add a `lang` prop
  there, smallest correct change.
- `RadialGauge`'s `%`/suffix handling: left `suffix` as separate literal text
  outside `SlotReel` (as before), rather than forcing it through
  `Intl.NumberFormat`'s `style: 'percent'`, since the KPI values are already
  0–100 numbers, not 0–1 fractions, and switching semantics would be a data
  contract change, not a motion change.
- Empty's "exit opacity only" is implemented as: a title/description change
  in place gets the identical `or-rise-xs` full reveal via a `key` swap, not
  a separate opacity-only-vs-full-rise distinction — the 24 real call sites
  never update `title`/`description` on an already-mounted `Empty` in
  practice, so a more elaborate two-mode mechanism would have no observable
  effect; stated as a scoped simplification rather than built unexercised.

**Follow-up fix (same day, two more signed commits on the same branch):** the
coordinator caught that as first shipped, SlotReel never rolled on page load
at all — emptying D-060 = A. My own spec-contradiction call above (mount is
always plain) was wrong; the coordinator's correction: the real distinction
is FRESH data vs CACHED data, not mount vs update, and mount alone cannot
express it (every SlotReel mount already has a value in hand, whether that
value was just fetched or served from cache). Fix: a `rollOnMount` prop
(default false, current behaviour preserved), read once via a lazy
`useState` initializer so a later flip can't retroactively start/cancel a
roll. Making it visually roll needed a genuine two-commit sequence — render
every digit at `'0'` on the actual first paint, then a `setTimeout(fn, 0)`
(a macrotask, so the browser paints the '0' frame first; a timer callback,
not a bare effect-body call, for the same `set-state-in-effect` reason as
[[feedback_set-state-in-effect-lint]]) moves it to the real value. Wired via
each caller's own query's `isFetchedAfterMount`: `KpiSpec`/`KpiCard` (both
copies) and `ScoreGauge`/`RadialGauge` gained a `fresh` prop, threaded
through DashboardPage, ViewerDashboard, AnalystDashboard, EstateDashboard,
AuditDashboard, ExecDashboard, ScorePage (both mounts) and
ExecutiveDashboard. New integration test
(`src/features/score/__tests__/scoreRollOnMount.test.tsx`): a real
`ScorePage` + real `QueryClient`, proving fresh-fetch-rolls and
cached-renders-plain, with the fresh case confirmed to fail pre-fix via
`git stash`. Verified again after: `npx tsc -b` clean, `npx vitest run`
805/805, `npm run budget` still 200.0 KB.

**Not done, with reasons:**
- githubactivity heatmap (spec item 4) — explicitly "no code this phase" per
  spec; needs its own issue.
- No PR opened, not pushed — per this session's explicit instructions (same
  as phase 2).

**Next:** motion-designer/qa-automation review, live pass, then the PR
("Part of #751") — none of that was done in this session.
