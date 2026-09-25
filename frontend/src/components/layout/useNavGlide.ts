// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The sidebar's proximity highlight (#751). One hover backdrop for the whole
// nav, which glides to whichever entry the pointer or the keyboard is on,
// instead of each entry lighting up and going dark on its own.
//
// Built in-house: RareUI's Proximity Sidebar was the reference, but its licence
// (Commons Clause) cannot ship in this repository (D-059). And deliberately
// less than it: nothing scales, nothing magnifies, no label moves. People read
// this list all day, so the effect changes how the highlight travels and
// nothing else. Colour is the same --bg-hover the entries used on their own.
//
// Off on touch screens (there is no hover to follow) and under
// prefers-reduced-motion, where the backdrop is not rendered at all and every
// entry keeps its own static hover background, exactly as before.
//
// Positions are written straight to the backdrop's style. A sweep of the
// pointer down the nav fires dozens of events, and none of them re-renders.

import { useEffect, useState, type FocusEvent, type PointerEvent, type RefObject } from 'react';

const QUERIES = ['(prefers-reduced-motion: reduce)', '(pointer: coarse)'];

/** True when the glide should run: a fine pointer, and motion allowed. */
function useGlideAllowed(): boolean {
  const read = () =>
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    QUERIES.every((q) => !window.matchMedia(q).matches);
  const [allowed, setAllowed] = useState(read);

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const lists = QUERIES.map((q) => window.matchMedia(q));
    // Followed, not read once: the preference can change mid-session.
    const onChange = () => setAllowed(lists.every((mq) => !mq.matches));
    lists.forEach((mq) => mq.addEventListener('change', onChange));
    return () => lists.forEach((mq) => mq.removeEventListener('change', onChange));
  }, []);

  return allowed;
}

/** The nav entries the backdrop follows. */
const ENTRY = 'a[data-nav-entry]';

/**
 * `backdropRef` is the caller's, attached to the backdrop element it renders
 * when `enabled`. Kept outside the returned object so that object holds no ref
 * and can be read during render.
 */
export function useNavGlide(backdropRef: RefObject<HTMLDivElement | null>) {
  const enabled = useGlideAllowed();

  const hide = () => {
    const backdrop = backdropRef.current;
    if (backdrop) backdrop.dataset.visible = 'false';
  };

  const moveTo = (target: EventTarget | null) => {
    const backdrop = backdropRef.current;
    const entry = target instanceof Element ? target.closest<HTMLElement>(ENTRY) : null;
    if (!backdrop || !entry) return;
    // The current page already has its accent background. The hover shade
    // under it would only muddy it, so the backdrop steps aside there.
    if (entry.getAttribute('aria-current') === 'page') {
      hide();
      return;
    }

    const place = () => {
      backdrop.style.transform = `translateY(${entry.offsetTop}px)`;
      backdrop.style.left = `${entry.offsetLeft}px`;
      backdrop.style.width = `${entry.offsetWidth}px`;
      backdrop.style.height = `${entry.offsetHeight}px`;
    };

    if (backdrop.dataset.visible === 'true') {
      // Already showing: glide from where it is.
      place();
      return;
    }
    // Arriving from outside the nav: appear in place, then fade in. The
    // transform transition is only armed while visible, and the reflow between
    // the two writes stops the browser from folding them into one slide from
    // wherever the backdrop was last left.
    place();
    void backdrop.offsetHeight;
    backdrop.dataset.visible = 'true';
  };

  const navProps = enabled
    ? {
        onPointerOver: (e: PointerEvent<HTMLElement>) => moveTo(e.target),
        onPointerLeave: hide,
        onFocus: (e: FocusEvent<HTMLElement>) => {
          // Keyboard focus moves it like the pointer does; a click's focus
          // does not, or the backdrop would jump to wherever was clicked last.
          if (e.target instanceof Element && e.target.matches(':focus-visible')) {
            moveTo(e.target);
          }
        },
        onBlur: (e: FocusEvent<HTMLElement>) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node | null)) hide();
        },
      }
    : {};

  return { enabled, navProps };
}
