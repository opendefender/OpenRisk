// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * ScrollProgress — a keyline that fills as a long scrolling area is read.
 *
 * For a form whose header and footer are pinned while the body scrolls (create
 * or edit a risk): the scrollbar says where you are in the body, but not at a
 * glance from the header, which is where the eye goes back to. It is the same
 * 2px accent keyline as the active tab and nav item.
 *
 * It follows the scroll directly, with no transition of its own, so it is not
 * motion in the reduced-motion sense: it moves only while the user moves the
 * content. It hides itself when the content does not overflow, since a bar
 * stuck at 100% on a short form says nothing.
 *
 * Decorative: aria-hidden. Scroll position is already exposed by the scroll
 * container itself, and a progressbar role would announce a "progress" that is
 * not progress through the task.
 */

import { useEffect, useRef, type RefObject } from 'react';
import { cn } from './cn';

export interface ScrollProgressProps {
  /** The element that scrolls. */
  target: RefObject<HTMLElement | null>;
  className?: string;
}

export function ScrollProgress({ target, className }: ScrollProgressProps) {
  const barRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = target.current;
    const bar = barRef.current;
    if (!el || !bar) return;

    // Written to the element, not to state: this runs on every scroll event,
    // and a re-render per frame would be the expensive way to move one bar.
    let frame = 0;
    const update = () => {
      frame = 0;
      const range = el.scrollHeight - el.clientHeight;
      if (range <= 1) {
        bar.style.opacity = '0';
        return;
      }
      const ratio = Math.min(1, Math.max(0, el.scrollTop / range));
      bar.style.opacity = '1';
      bar.style.transform = `scaleX(${ratio})`;
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };

    update();
    el.addEventListener('scroll', schedule, { passive: true });
    // The content grows as sections expand or errors appear under fields.
    const observer = new ResizeObserver(schedule);
    observer.observe(el);
    for (const child of Array.from(el.children)) observer.observe(child);
    return () => {
      el.removeEventListener('scroll', schedule);
      observer.disconnect();
      if (frame) cancelAnimationFrame(frame);
    };
  }, [target]);

  return (
    <div aria-hidden="true" className={cn('h-(--keyline-w) w-full', className)}>
      <div
        ref={barRef}
        data-testid="scroll-progress"
        className="h-full w-full origin-left rounded-full bg-accent opacity-0"
        style={{ transform: 'scaleX(0)' }}
      />
    </div>
  );
}
