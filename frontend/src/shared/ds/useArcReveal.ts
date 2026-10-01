// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Two-step reveal for a gauge arc's `strokeDashoffset`.
 *
 * Review coherence fix (#751 phase 3): on a fresh mount the number rolled
 * (via `SlotReel`'s `rollOnMount`) while the arc beside it snapped straight
 * to its final sweep — the two halves of the same number told a different
 * story about whether this data had just arrived.
 *
 * `false` (not yet revealed) for exactly the FIRST render after a `fresh`
 * mount — the caller renders `strokeDashoffset={1}` (fully undrawn) for
 * that render — then `true` from the next tick onward, once the caller
 * switches to the real `1 - pct`. `--dur-panel` (already declared on the
 * path's own `transition`) then draws it in, because a CSS transition only
 * plays between two PAINTED states: this needs the SAME two-commit shape as
 * `SlotReel`'s own `mountRoll`, not the render-time-adjustment trick this
 * codebase uses for a plain value CHANGE elsewhere, which collapses into a
 * single commit and would never paint the undrawn frame at all.
 *
 * Read once, at mount, via a lazy initializer: a cached render (`fresh`
 * false/absent) never enters the pending state, so it paints the real
 * dashoffset immediately — plain, matching `SlotReel`'s "mount with data
 * already present" contract — and nothing here ever re-arms once revealed,
 * so a later same-value refetch (or any later render at all) cannot make
 * the arc re-sweep.
 */

import { useEffect, useState } from 'react';

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

/** @returns true once the caller should render the real `1 - pct` offset. */
export function useArcReveal(fresh: boolean | undefined): boolean {
  const [pending, setPending] = useState(() => !!fresh && !prefersReducedMotion());

  useEffect(() => {
    if (!pending) return;
    // A macrotask, not a bare effect-body setState — the browser paints the
    // pending (undrawn) frame before this callback ever runs, same as
    // SlotReel's own mount-roll timer. `useEffect` already runs AFTER the
    // browser has painted; the timer's job is only to keep this setState out
    // of the effect BODY, which `eslint-plugin-react-hooks`'s
    // `set-state-in-effect` rule forbids. Moving this to `useLayoutEffect`
    // would break the reveal: a layout effect (and any synchronous setState
    // inside one) runs and re-renders BEFORE the browser paints, so the
    // "undrawn" frame would never reach the screen — the arc would just
    // appear at its final sweep, silently, exactly the bug this hook exists
    // to fix.
    const id = setTimeout(() => setPending(false), 0);
    return () => clearTimeout(id);
  }, [pending]);

  return !pending;
}
