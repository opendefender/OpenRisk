// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The restrained tilt on the dashboard's overall score card (#856, D-063).
//
// It goes on the card's wrapper in DashboardPage, never inside ScoreGauge: the
// arc and the figure move with the card as one surface, so the data is never
// skewed against its own frame. At most 2° per axis behind an 800px perspective.
//
// It only exists for a mouse that can hover, with motion allowed. Under touch, a
// coarse pointer or `prefers-reduced-motion: reduce`, no listener is attached
// and the element never gets an inline transform, so its computed transform is
// `none`. Keyboard focus never tilts it either: only pointer movement does.
//
// Deliberately absent: glare, a moving shadow, and any change to cursor or role.
// The card is not clickable and must not look it — only the button inside
// ScoreGauge is.

import { useEffect, type RefObject } from 'react';

/** Hover-capable fine pointer AND motion allowed. One query, one listener. */
export const TILT_QUERY =
  '(hover: hover) and (pointer: fine) and (prefers-reduced-motion: no-preference)';
export const TILT_MAX_DEG = 2;
export const TILT_PERSPECTIVE_PX = 800;

/**
 * The rotation for a pointer at (x, y) over `rect`, in degrees, each clamped to
 * ±TILT_MAX_DEG. The side under the pointer dips away from the viewer: pointer
 * at the right edge → rotateY +2°, at the top edge → rotateX +2°.
 */
export function tiltFor(
  x: number,
  y: number,
  rect: Pick<DOMRect, 'left' | 'top' | 'width' | 'height'>,
): { rotateX: number; rotateY: number } {
  if (rect.width <= 0 || rect.height <= 0) return { rotateX: 0, rotateY: 0 };
  const clamp = (n: number) => Math.max(-1, Math.min(1, n));
  // -1 at the left/top edge, +1 at the right/bottom edge.
  const nx = clamp(((x - rect.left) / rect.width) * 2 - 1);
  const ny = clamp(((y - rect.top) / rect.height) * 2 - 1);
  // `+ 0` folds -0 into 0, so a centred pointer prints "0deg", not "-0deg".
  return { rotateX: -ny * TILT_MAX_DEG + 0, rotateY: nx * TILT_MAX_DEG + 0 };
}

export function tiltTransform({ rotateX, rotateY }: { rotateX: number; rotateY: number }): string {
  return `perspective(${TILT_PERSPECTIVE_PX}px) rotateX(${rotateX.toFixed(2)}deg) rotateY(${rotateY.toFixed(2)}deg)`;
}

export function useScoreCardTilt(ref: RefObject<HTMLElement | null>): void {
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof window.matchMedia !== 'function') return;
    const mq = window.matchMedia(TILT_QUERY);

    let frame = 0;
    let lastX = 0;
    let lastY = 0;

    const apply = () => {
      frame = 0;
      el.style.transform = tiltTransform(tiltFor(lastX, lastY, el.getBoundingClientRect()));
    };
    const onMove = (e: PointerEvent) => {
      // A fine-pointer device can still have a touchscreen; only a mouse tilts.
      if (e.pointerType !== 'mouse') return;
      lastX = e.clientX;
      lastY = e.clientY;
      // One update per animation frame, however fast the events arrive.
      if (!frame) frame = requestAnimationFrame(apply);
    };
    const flatten = () => {
      if (frame) cancelAnimationFrame(frame);
      frame = 0;
      // Cleared, not rotate(0): at rest the card has no transform at all, so
      // the text is rasterised flat and the computed style reads `none`.
      el.style.transform = '';
    };

    const attach = () => {
      // Short enough to follow the pointer, long enough to smooth a jump.
      el.style.transition = 'transform var(--dur-base) var(--ease-out)';
      el.addEventListener('pointermove', onMove);
      el.addEventListener('pointerleave', flatten);
    };
    const detach = () => {
      el.removeEventListener('pointermove', onMove);
      el.removeEventListener('pointerleave', flatten);
      flatten();
      el.style.transition = '';
    };

    const sync = () => (mq.matches ? attach() : detach());
    sync();
    // Turning reduced motion on mid-session, or unplugging the mouse, takes
    // effect at once and leaves the card flat.
    mq.addEventListener('change', sync);
    return () => {
      mq.removeEventListener('change', sync);
      detach();
    };
  }, [ref]);
}
