// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Motion policy for the auth screens, in one place so every screen obeys the
// same rules:
//
//   - nothing runs longer than 400 ms;
//   - entry is a cascade at 80 ms per step;
//   - `prefers-reduced-motion: reduce` disables all of it — not "shortens", not
//     "keeps a subtle version". That preference is often set because motion
//     causes real symptoms, so a tasteful compromise is still the wrong answer.
//
// index.css carries a global `prefers-reduced-motion` rule that strips every CSS
// animation and transition. This module is the JavaScript half: timers and
// intervals are invisible to CSS, so anything driven from JS has to consult the
// preference itself.
//
// Kept out of AuthLayout.tsx so that file exports components only (React Fast
// Refresh, enforced by react-refresh/only-export-components).

import { useEffect, useState } from 'react';

/** Reads the OS reduced-motion preference and keeps following it. */
export function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return false;
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  });

  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return;
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    const onChange = (e: MediaQueryListEvent) => setReduced(e.matches);
    // Subscribed rather than read once: someone can turn the preference on while
    // the login screen is open, and it must take effect without a reload.
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, []);

  return reduced;
}

/**
 * Entry cascade: 80 ms per step, 260 ms per element.
 *
 * Returns an empty style object when motion is off, so the element renders in
 * its final state with no animation to strip.
 */
export function cascade(step: number, reduced: boolean): React.CSSProperties {
  if (reduced) return {};
  return {
    animation: 'or-fadeup 260ms ease both',
    animationDelay: `${step * 80}ms`,
  };
}
