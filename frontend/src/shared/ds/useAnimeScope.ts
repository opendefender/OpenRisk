// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * `useAnimeScope` — the only sanctioned entry point to anime.js (#445).
 *
 * anime.js is the SECOND animation runtime in this bundle; `framer-motion` is the
 * first and remains the default for everything else. D-028 admits it for one job
 * CSS cannot do — drawing SVG geometry on entry — and confines it to this hook so
 * the import surface is one file rather than every chart.
 *
 * WHAT THIS HOOK GUARANTEES, AND WHY EACH ONE EXISTS
 * --------------------------------------------------
 * 1. **`scope.revert()` on unmount.** React StrictMode mounts every component
 *    twice in development. Without revert the first mount's observers and tweens
 *    survive it, so the second mount animates targets the first is still driving —
 *    which looks like a flicker and is really a leak.
 *
 * 2. **`prefers-reduced-motion` short-circuits the whole hook**, rather than
 *    shortening the duration. The scope is never created and `setup` is never
 *    called, so no anime.js code runs at all for a user who asked for stillness.
 *
 * 3. **`willAnimate` is returned, and callers MUST honour it.** The design guide is
 *    explicit that *"reduced motion is not 'less motion'"*: a draw that never plays
 *    has to render its FINAL state, not an empty canvas. The contract is therefore
 *    inverted from the obvious one — a component renders its finished self by
 *    default, and only when `willAnimate` is true does it render the hidden initial
 *    state for the animation to move away from. Read it during render, not in an
 *    effect, or the chart paints once before hiding itself and the user sees a
 *    flash of the very content the animation is about to reveal.
 *
 * 4. **Draw once, on first view.** An `IntersectionObserver` starts the scope the
 *    first time the root is visible and then disconnects, so the animation cannot
 *    replay on re-render, on a filter change, or on scrolling back. The guide:
 *    *"Once. No pulse, no loop, no ambient drift."*
 *
 *    This is a plain `IntersectionObserver` on purpose. anime.js's `Scroll` module
 *    is 4.30 KB and #445 forbids it outright — *"the console never animates on
 *    scroll"* — and nothing here reacts to scroll POSITION; it reacts once to first
 *    visibility, which is a different thing.
 *
 * `animate`, `stagger` and `createDrawable` are imported by the CALLER, named, from
 * `animejs`. This hook imports only `createScope`, so a screen that never animates
 * pulls in nothing but the scope module.
 */

import { useEffect, useRef, useState, type DependencyList, type RefObject } from 'react';
import { createScope, type Scope } from 'animejs';

export interface UseAnimeScopeOptions {
  /**
   * Wait for the root to be visible before running `setup`. Default `true`.
   * Pass `false` only for something already on screen at mount.
   */
  onFirstView?: boolean;
  /** Rebuild the scope when these change. Default `[]` — build once. */
  deps?: DependencyList;
}

export interface UseAnimeScopeResult<T extends HTMLElement | SVGElement> {
  /** Attach to the element that owns the animated subtree — usually the `<svg>`. */
  ref: RefObject<T | null>;
  /**
   * `false` when the user prefers reduced motion. Render the FINAL state when this
   * is false; render the initial state only when it is true. See guarantee 3.
   */
  willAnimate: boolean;
}

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

export function useAnimeScope<T extends HTMLElement | SVGElement = SVGSVGElement>(
  setup: (scope: Scope) => void,
  options: UseAnimeScopeOptions = {},
): UseAnimeScopeResult<T> {
  const { onFirstView = true, deps = [] } = options;
  const ref = useRef<T | null>(null);

  /*
   * Read once, at first render, and keep it. A lazy initialiser rather than a
   * useEffect because guarantee 3 needs the answer DURING render — the caller
   * decides what to paint based on it, and an effect answers too late.
   */
  const [willAnimate] = useState(() => !prefersReducedMotion());

  /* Keep `setup` current without making it a dependency: an inline arrow would
     otherwise rebuild the scope on every render, i.e. animate on every render. */
  const setupRef = useRef(setup);
  setupRef.current = setup;

  useEffect(() => {
    if (!willAnimate) return;
    const root = ref.current;
    if (!root) return;

    let scope: Scope | null = null;
    const run = () => {
      if (scope) return;
      scope = createScope({ root: ref as RefObject<HTMLElement | SVGElement> });
      scope.add(() => {
        setupRef.current(scope as Scope);
      });
    };

    if (!onFirstView) {
      run();
      return () => scope?.revert();
    }

    if (typeof IntersectionObserver === 'undefined') {
      /* jsdom and very old browsers: draw immediately rather than never. */
      run();
      return () => scope?.revert();
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((e) => e.isIntersecting)) return;
        observer.disconnect(); // once
        run();
      },
      { threshold: 0.15 },
    );
    observer.observe(root);

    return () => {
      observer.disconnect();
      scope?.revert();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [willAnimate, onFirstView, ...deps]);

  return { ref, willAnimate };
}
