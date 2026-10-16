---
name: technique-exit-windows-and-live-timing
description: Durable lessons from the #751 reviews — short exit windows hide defects in jsdom, a flash before a synchronous unmount never paints, light-theme contrast is independent, and live timing must be measured in-page
metadata:
  type: feedback
---

Kept from the #751 phase reviews (2026-09-28/29). The defects themselves were
fixed on master; these are the ways of finding them again.

- **A flash before a synchronous unmount never paints.** Setting a transient
  state (success check) and closing its parent in the same continuation is
  batched into one commit; the child never renders the new state. When a
  spec says "shows X then closes", check that X's owner survives the commit.
- **Exit windows (~120 ms) hide two kinds of bug.** Old and new UI coexist
  while an exit animation runs. Tests that `waitFor` the unmount, or fire
  `animationEnd` at once, never see that window. Add a test that acts again
  before the exit ends (type, Tab, Escape). jsdom does not enforce
  `pointer-events: none`; only a real browser can prove a click is blocked.
- **Light theme is checked on its own.** Severity colours on a 16 % tint are
  independently authored per theme; dark passing proves nothing for light.
  Recompute the contrast ratio in both themes for small text.
- **Live timing goes in one in-page evaluate.** Each MCP browser round trip
  can cost seconds, enough to miss a 4 s toast. Click and measure in the same
  `browser_evaluate`, with `performance.now()`; cross-check with fake-timer tests.
- **A throwaway harness lives outside `src/`** (next to `e2e/visual/harness.tsx`)
  and is deleted after the pass; `git status --short` proves it.
- **The shared Playwright profile can be locked by a sibling agent.** If the
  profile's `Default/` mtimes keep moving, the lock is live: never kill the
  browser, wait, and if it never frees, report the live pass as not done.

See [[technique-worktree-revert-check]] for proving a test fails without the fix.
