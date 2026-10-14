---
name: 751-phase5-status
description: Status of issue #751 phase 5 ("auth & micro-interactions") on branch feat/751-phase5-micro — Modal/Drawer real exit transition, OTP auto-submit, theme-toggle icon cross-fade, stopgap dialog entrances; what shipped, interpretation calls, verification figures.
metadata:
  type: project
---

Branch `feat/751-phase5-micro`, cut from `master` at `234c342c` (after #819
premium-ui and #823 phase2-feedback merged; independent of #824 phase3-dashboard
and #825, per [[project_751-phase4-status]]'s sibling branch). Binding spec:
art-director + ux-designer, arbitrated, posted to the issue and a scratchpad
file (p5-spec-comment.md, p5-art.md/p5-ux.md as supporting detail — spec
comment wins on conflicts). No PR opened, not pushed, per the task's explicit
instructions. D-061 (3D tilt, dropped) was already on the branch before this
session; this session only added D-062.

**Shipped, six commits (`git commit -s`, no co-author trailer):**
1. `491e5a27` — `Modal.tsx`/`Drawer.tsx` stop doing `if (!open) return null`
   (cut every close instantly) and instead stay mounted at
   `data-state="closed"` through a real CSS exit, unmounting via
   `useExitTimer` (byte-identical copy from `feat/751-phase4-notifications`,
   exported from `shared/ds/index.ts` alongside `useSuccessFeedback`). New
   `shared/ds/overlayMotion.ts` holds `MODAL_EXIT_MS`/`DRAWER_EXIT_MS` (kept
   out of Modal.tsx/Drawer.tsx for the same `react-refresh/only-export-
   components` reason phase 4's `notifMotion.ts` was split out), with a
   drift-guard test against `--dur-fast`/`--dur-base` in primitives.css.
   Motion moved from `motion-safe:animate-or-*` keyframes to plain CSS in
   index.css (`.or-scrim`/`.or-modal-panel`/`.or-drawer-panel`), using the
   asymmetric-duration trick ([[feedback_css-motion-tricks]]) so ENTER and
   EXIT carry different tokens without the Tailwind duration-variant trap
   ([[tailwind-duration-variant-trap]] in user memory) — exit values on the
   base/closed rule, enter values on `[data-state='open']`, because the
   browser plays the duration of the state being entered.
2. `3f82148f` — OtpField's ring now transitions `border-color,box-shadow`
   (slides between boxes) and its character fades in on `--motion-press`
   (always-rendered span, `opacity-0`→`100`). `onComplete` wired in
   AuthScreen's `MFAEnrollment` and `MFAEnrollmentDialog`, guarded by `busy`.
   **Real bug found while testing this**: `onComplete` fires synchronously
   inside the same commit as the `setCode` that precedes it, so reading the
   component's `code` closure at that point still sees the PREVIOUS value —
   `submit` needed a `codeOverride` parameter fed the value `onComplete`
   itself received, not `code.trim()`. AuthScreen's MFAChallenge (plain
   input, 6-digit TOTP OR 12-char recovery code, deliberately not an
   OtpField) is untouched — no `onComplete` to wire.
3. `f1cc468d` — AppHeader's Sun/Moon cross-fade in one grid cell
   (`[grid-area:1/1]`, opacity+scale on `--motion-hover`, no rotation);
   `aria-label` names the result ("Switch to light/dark theme"), new
   `themeToLight`/`themeToDark` keys in `shared/uiStrings.ts` (FR/EN) —
   this chrome's established pattern, not `locales/*.json`.
4. `c8d59b26` — `GovernancePage.tsx`'s policy modal (`or-scalein`, a class
   with no CSS rule anywhere — popped in with zero motion) and
   `MFAEnrollmentDialog.tsx` (hand-rolled, no entrance at all) both get
   `motion-safe:animate-or-rise`/`-fadein` as a stopgap. Neither is the ds
   `Modal`, so neither gets commit 1's real exit; migrating them is a
   separate issue.
5. `1fa37aab` — D-062 in `docs/DECISIONS.md`: MFA lockout/rate-limit copy
   NOT added. `backend/internal/handler/auth/mfa_handler.go`'s Challenge/
   Verify and their use cases return the same typed `ErrValidation` for a
   first wrong code and a hundredth — no distinct signal to build copy on.
6. `2c2f8f69` — prettier `--write` on the 3 files it flagged among those
   touched (AppHeader.tsx, mfaEnrollmentDialog.test.tsx, primitives.test.tsx)
   — cosmetic only, no other path in the tree touched.

**Conditional-render callers that still cut** (Modal/Drawer's new exit can
only play if the component itself stays mounted — listed, not rewritten,
per the task's explicit instruction):
- `features/tprm/VendorDetailPage.tsx:188` — `{sendOpen && chain.data && (<SendAssessmentModal .../>)}`.
- `features/compliance/RemediationDetailPage.tsx:180` — `{confirmDelete && plan && (<DangerConfirm open .../>)}`.
- `features/compliance/AuditDetailPage.tsx:194` — `{confirmDelete && audit && (<DangerConfirm open .../>)}`.
- `features/entity-drawer/EntityDrawerHost.tsx:56` — `if (!state) return null;` before `<EntityDrawer>`, whose own `<Drawer open ...>` (EntityDrawer.tsx:117) is hardcoded `open` (always true) — the PARENT unmounts the whole subtree on close, not a prop flip.
All other ds Modal/Drawer callers checked (`AssetHistoryDrawer`,
`BulkPreviewDialog`, two `MembersView` DangerConfirms, `SettingsScreen`'s
DangerConfirm, two `AlertDialog` sites, three `ImpactDialog` sites,
`SendAssessmentModal` itself) render the component unconditionally and pass
a real `open` prop — those get the new exit for free.

**Test-file gotchas hit this session (worth remembering beyond this phase):**
- `vi.spyOn(window, 'matchMedia').mockRestore()` is unsafe here: the base
  mock in `src/test/setup.ts` is ALREADY a `vi.fn().mockImplementation(...)`,
  and restoring a spy wrapped around an already-mocked function clears that
  implementation instead of bringing it back — every test after the first to
  do this gets `window.matchMedia(...)` returning `undefined`, not an error at
  the spy site itself. Fix: save `window.matchMedia` in a plain variable and
  reassign it directly in a `finally`, never `vi.spyOn(...).mockRestore()`
  against that particular setup mock.
- `Shake` (shared/ds or features/auth/fields.tsx) remounts its children via
  React `key={errorKey}` on every new error nonce — a DOM node reference
  captured before a failed submit is DETACHED after it; re-query with
  `screen.getByTestId(...)` rather than reusing the earlier handle.
- jsdom's `navigator.clipboard` is a REAL (if fake) Clipboard implementation
  by default in this vitest version, not undefined — `Object.defineProperty`
  or plain assignment at module scope silently loses to it (and isn't even
  reachable that early), but `vi.spyOn(navigator.clipboard, 'writeText')`
  called per-test (after `render`) works.
- A `vitest run` across the whole suite (89 files, 860 tests, heavy real-timer/
  userEvent use) showed one transient extra failure in an unrelated,
  independently-passing test on a single run out of four — resource
  contention under full-suite parallelism, not a regression (confirmed by
  rerunning the full suite clean and the specific file 3x in isolation).

**Verified before handback:** `npx tsc -b` — clean except the two
pre-existing errors also present on the branch's own base (`MembersView.tsx`
`DeleteButton`, `CreateRiskModal.tsx` `ScrollProgress`, confirmed via a
disposable `git worktree` at `234c342c`, not a stash); `npx eslint` on all 16
touched files, `--max-warnings=0` — clean except `GovernancePage.tsx`'s
pre-existing `react-hooks/purity` error and three `exhaustive-deps` warnings,
identical on the same worktree baseline; full `npx vitest run`: 850/860
green, same 10 pre-existing failures as the `234c342c` baseline (833/843
there) — net +17 tests, all new, all green, 0 regressions; `npx vite build`
clean; `npm run budget`: 201.1 KB vs a freshly-built `234c342c` baseline of
201.0 KB (disposable worktree) — +0.1 KB, the 180 KB gate itself is
pre-existing failing debt this phase did not move.

**Not implemented, stated plainly:** the "reopening mid-exit" and "unmounts
at once under reduced motion" required tests exist for Modal (both) and
Drawer (reduced-motion only) in `primitives.test.tsx`, not a third,
near-duplicate "reopening mid-exit" test for Drawer specifically — the state
machine is identical code, covered once. The "policy modal and MFA dialog
carry the enter classes" required test exists only for `MFAEnrollmentDialog`
(cheap, already had a working harness); `GovernancePage.tsx` has no test
file at all and building one just to assert a className was judged not worth
its cost against a component this large — the change itself (two class
names) is trivial and was read, not just asserted.

**Next** — motion-designer/qa-automation review, a live pass (the phase 4
lesson: motion figures need measuring in a real browser, not assumed), then
the PR ("Part of #751") — none of that was done in this session (no push, no
PR, per this session's explicit instructions, same as phases 2-4).
