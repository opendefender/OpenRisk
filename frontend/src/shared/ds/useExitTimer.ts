// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Keeps a layer mounted for its CSS exit animation, then reports it as gone.
 *
 * `open` flipping to `false` does not unmount anything by itself: the caller
 * keeps rendering the layer while this hook returns `true`, so its CSS exit
 * animation (keyed off the same `open` value, e.g. a `data-open` attribute)
 * gets a frame to play before the node leaves the DOM. The unmount itself
 * runs off a plain timer, never `transitionend` — an event that would simply
 * never fire once `prefers-reduced-motion: reduce` kills every transition and
 * animation globally (index.css:243), stranding the layer mounted forever.
 * Reduced motion instead collapses `exitMs` to 0 here, so the two mechanisms
 * (the CSS kill switch and this timer) agree without either watching the
 * other.
 *
 * Re-opening before the exit timer fires cancels it immediately: `open` and
 * `mounted` are kept in sync at render time whenever `open` is true (React's
 * "adjust state while rendering" pattern — see
 * .claude/agent-memory/frontend-react/feedback_set-state-in-effect-lint.md),
 * so a mount is never in doubt.
 */

import { useEffect, useState } from 'react';

export function useExitTimer(open: boolean, exitMs: number): boolean {
  const [mounted, setMounted] = useState(open);
  if (open && !mounted) setMounted(true);

  useEffect(() => {
    if (open || !mounted) return;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const timer = setTimeout(() => setMounted(false), reduced ? 0 : exitMs);
    return () => clearTimeout(timer);
  }, [open, mounted, exitMs]);

  return mounted;
}
