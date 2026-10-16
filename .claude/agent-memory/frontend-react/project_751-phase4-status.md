---
name: 751-phase4-status
description: Status of issue #751 phase 4 ("Notifications & alerts") on branch feat/751-phase4-notifications — bell badge, notif panel dismissable layer, toast restyle, banner stack/collapse; what shipped, sonner internals verified (not assumed), and the interpretation calls made.
metadata:
  type: project
---

Branch `feat/751-phase4-notifications`, cut from `master` at `e1aafd5`
(same commit phases 2/3 were cut from — see [[project_751-phase2-feedback]],
[[project_751-phase3-status]]). Binding spec: art-director + ux-designer,
arbitrated, posted to the issue and a scratchpad file (p4-spec-comment.md,
with p4-art.md/p4-ux.md as supporting detail — the spec comment wins on
conflicts). No PR opened, not pushed, per the task's explicit instructions.

**Shipped, four atomic signed commits (`git commit -s`, no co-author
trailer):**
1. `cbd3d64` — Bell badge shows the real unread count (was a dot), capped
   `9+` visually but the bell's `aria-label` always carries the true number.
   `useUnreadCount` now returns `{ count, isFetched }`. Badge is always
   mounted, gated by `data-armed`/`data-visible` (CSS asymmetric-duration
   trick, `.notif-badge` in index.css) — a same-count poll changes neither
   attribute so the transition structurally cannot retrigger. "Armed" flips
   true one macrotask after the query's first fetch resolves (new local
   `useArmedBadge` hook in AppHeader.tsx), so the very first paint is always
   unanimated even when the query resolves from warm cache.
2. `c1be1e8` — Notif panel wired to `useDismissableLayer` (`closeOnEscape:
   true, lockScroll: false`); Escape closes and returns focus to the bell,
   Tab traps inside, initial focus lands on the first focusable control.
   New `shared/ds/useExitTimer.ts` (keeps a layer mounted through its CSS
   exit animation, unmounting on a plain timer, instant under reduced
   motion) — reused again in commit 4. Panel's hard-coded inline
   `or-scalein .16s` replaced with `or-notif-enter`/`or-notif-exit`
   keyframes (animation-name swap on `data-open`, not a transition, because
   it has to play on the panel's very first paint too).
3. `21c6920` — `ThemedToaster` drops `richColors` for `toastOptions.classNames`
   mapped to tokens (`--bg-elevated`/`--border`/`--elev-3`), a per-type lucide
   icon, `bottom-right` at 16px offset, `visibleToasts={3}`, `expand`, `gap=8`.
   `useToast` durations: success/info 4000ms, warning 6000ms, error 8000ms;
   any toast with an action is `Infinity` (WCAG 2.2.1). CSS override
   (`[data-sonner-toast][data-styled='true'].or-toast`, unlayered, higher
   specificity than sonner's own injected stylesheet — Tailwind utilities in
   `@layer utilities` would lose to sonner's unlayered CSS regardless of
   specificity/order, cascade layers always lose to unlayered rules) touches
   only surface/typography and the transition duration/easing, never
   `transform`/`opacity` values themselves (those encode sonner's multi-toast
   stack offset via `--y`; overwriting them would break the stack, not just
   the motion).
4. `adb7d5d` — New `shared/BannerStack.tsx` (thin ordered wrapper, no
   priority/collapse logic — none was asked for at ≤2 banners). Wraps Demo
   (permanent, top) + Offline (closest to header) in App.tsx. OfflineBanner
   always mounted now, collapses via a new shared `.or-collapse` CSS class
   (`grid-template-rows: 0fr→1fr` + opacity — needed because there is no
   fixed pixel height to transition any other way, and needs `min-height: 0`
   on the child or a grid item refuses to shrink below its content size),
   held open ≥2000ms from first shown regardless of how fast the connection
   recovers, content frozen the instant it starts closing so the collapse
   animates the last real message. MFAEnrollmentBanner's dismiss reuses
   `useExitTimer` for the same collapse-then-unmount; the dismissed section
   drops `data-testid`/`role`/`aria-live` the instant `dismissed` flips
   (before the exit timer fires) so the pre-existing session-dismiss test's
   *synchronous* `queryByTestId(...).not.toBeInTheDocument()` assertion keeps
   passing while the pixels still fade for `--dur-fast`.

**Verified, not assumed (ThemedToaster.test.tsx, real sonner 2.0.7, no
mock):** the ux spec's two "verify before shipping" claims about sonner both
turned out FALSE and are reported, not silently patched by forking the
dependency:
- Sonner sets one shared `aria-live="polite"` on its outer `<section>` for
  every toast type; no toast, including error, ever gets `role="alert"` —
  read straight from `node_modules/sonner/dist/index.mjs`, confirmed by a
  real jsdom render.
- Hovering/pointerdown on a toast pauses its dismiss timer (`setExpanded`/
  `setInteracting`); the toaster's `onFocus` handler only remembers where to
  return focus later (its own Escape-close feature) and never touches that
  state — there is no `focusin` listener anywhere in the package. Tabbing
  keyboard focus onto a toast's action button does **not** pause dismissal.
Neither is fixed this phase (restyling sonner, not forking it, per the
spec's own instruction); both are pinned down by a positive-control test
(mouse hover proven to pause, in the same file) so a future sonner bump that
changes either is caught rather than re-assumed.

**Spec-vs-code interpretation calls made (stated, not escalated):**
- Bell aria-label copy: art spec's arbitrated comment gives
  `"Notifications, N unread"` / `"Notifications, N non lues"` (comma form);
  ux spec's own alternate draft used a parenthetical form. Followed the
  arbitrated spec-comment's literal wording, per the task's own
  spec-comment-wins rule.
- New chrome strings (bell aria-label) were NOT added to
  `src/locales/{fr,en}.json` — `shared/uiStrings.ts`'s own header comment
  says it is "kept separate from the nested locales/*.json ... Consume via
  useUIStrings()", and `AppHeader.tsx` already sources every other string
  (`notifTitle`, `notifAll`, density/shortcuts labels) from that file or
  inline `lang === 'fr' ? … : …` ternaries, never from the JSON catalog. New
  strings followed the file's own established, documented pattern instead
  of the task's generic locales-file instruction.
- Toast's literal "8px rise" / "4px drop" enter/exit amplitude was NOT
  replicated — only the transition duration/easing were overridden (see
  commit 3 above). Replicating the exact pixel amplitude would mean
  overwriting sonner's `transform`/`--y`, which also carries its stacking
  math; doing so is exactly the "cost more than it gains" replacement the
  spec said not to do.

**Pre-existing lint debt touched-but-not-fixed (confirmed via `git stash`,
same convention as [[feedback_lint-pre-existing-debt]]):** `App.tsx` — unused
`GlobalShortcuts` import, and a `set-state-in-effect` on the unrelated
guided-tour-redirect effect (~line 362); `MFAEnrollmentBanner.tsx` —
`react-refresh/only-export-components` (the pre-existing `copyFor` named
export sits beside the component in the same file). Both existed at the
exact same lines (shifted only by my own added lines) before any of my
edits.

**Verified before handback:** `npx tsc -b` clean; `npx eslint <every file
touched across all 4 commits> --max-warnings=0` clean except the two
pre-existing items above; full `npx vitest run`: 818/818 green (was 812
green on master pre-branch); `npx vite build` clean; `npm run budget`:
201.1 KB vs a freshly-built `e1aafd5` baseline of 199.9 KB (confirmed via a
disposable `git worktree`, not a stash, after a stash mishap — see
[[feedback_git-stash-untracked-path-footgun]]) — +1.2 KB, the 180 KB gate
itself is pre-existing failing debt, not something this phase moved.

**Next:** motion-designer/qa-automation review, a live pass (banner text
contrast at 12.5px on a 16% tint, badge contrast, both flagged "must
measure" by the art spec and not checked here), then the PR ("Part of
#751") — none of that was done in this session (no push, no PR, per this
session's explicit instructions, same as phases 2/3).
