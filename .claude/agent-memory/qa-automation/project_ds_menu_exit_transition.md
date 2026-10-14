---
name: project-ds-menu-exit-transition
description: shared/ds/Menu.tsx delayed-unmount exit transition (#751) — known focus-trap gap during the 120ms exit
metadata:
  type: project
---

`frontend/src/shared/ds/Menu.tsx` (commit e2f016e, issue #751, PR #819) added
a 120ms exit transition via floating-ui's `useTransitionStatus` — the menu
stays mounted with `data-status="close"` and `pointer-events-none` for
`EXIT_MS = 120` after close, instead of unmounting immediately.

**Regression found in review (not yet filed as an issue as of 2026-09-28):**
`FloatingFocusManager modal` keeps focus trapped on the exiting menu
container itself for the full 120ms after Escape/dismiss — Tab does nothing,
focus does not visibly move, and the focused node is `opacity: 0` /
`pointer-events: none` during that window. Focus only reaches the trigger
button once the component actually unmounts (~120-150ms later). Confirmed
with a jsdom probe: right after `Escape`, `document.activeElement` is the
`role="menu"` div itself (not the trigger, not lost to body), and two
subsequent `Tab` presses do not move it. This is a real (if minor, ~120ms)
violation of UX doctrine rule 8/9 (focus returns to trigger on Escape; focus
never invisible/trapped past its usable life). Every test in
`floating.test.tsx` that checks "focus returns to trigger" wraps it in
`waitFor(() => menu not in document)` first, which masks the delay — none of
them assert on the pre-unmount window.

Also unverifiable in jsdom: whether `pointer-events: none` actually blocks a
real click on a menuitem during the 120ms exit — jsdom does not enforce CSS
pointer-events on dispatched events, so a scripted `user.click()` on a
still-mounted, closing item succeeds in jsdom regardless. Needs a real
browser (Playwright) to prove; `shared/ds/Menu` has zero consumers in `src/`
as of this review, so there is no E2E page to hang that check on.

**Why:** relevant context for anyone re-touching Menu.tsx's motion code or
deciding whether to file the focus-trap-during-exit issue.
**How to apply:** if asked to review or extend Menu.tsx's transition, check
first whether this was filed/fixed; if not, it's still open. If a consumer of
`ds/Menu` is added, add a Playwright test for the pointer-events-during-exit
question since jsdom can't answer it.
