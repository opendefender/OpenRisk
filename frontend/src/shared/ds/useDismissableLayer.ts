// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The shared behaviour of every layer that covers the page: modal, drawer,
 * and anything else that takes over.
 *
 * Four things, all of which were previously each overlay's own problem and
 * therefore missing from most of them:
 *
 *   1. Escape closes.
 *   2. Focus moves INTO the layer on open and RETURNS to the trigger on close.
 *      Without the return, dismissing a dialog drops a keyboard user back at
 *      the top of the document — they lose their place in a 200-row table.
 *   3. Tab is trapped inside. Without it, tabbing walks out of the dialog and
 *      into the page behind it, which is still there and still clickable to
 *      anything that is not a mouse (WCAG 2.4.3).
 *   4. The page behind does not scroll, and does not shift sideways when the
 *      scrollbar is removed.
 *
 * Nested layers: each mount pushes onto a module-level stack and only the top
 * layer responds to Escape, so closing a confirmation inside a drawer closes
 * the confirmation — not both.
 */

import { useEffect, useRef, type RefObject } from 'react';

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

/** Open layers, innermost last. Only the last one reacts to Escape. */
const layerStack: symbol[] = [];

/** How many layers currently want the page frozen. */
let scrollLocks = 0;
let restoreBodyStyle: { overflow: string; paddingRight: string } | null = null;

function lockScroll() {
  scrollLocks += 1;
  if (scrollLocks > 1) return;
  const { body } = document;
  restoreBodyStyle = { overflow: body.style.overflow, paddingRight: body.style.paddingRight };
  // Replacing the scrollbar's width with padding keeps the page from jumping
  // sideways as it is hidden — a 15px shift of everything behind the dialog is
  // small, constant, and reads as the app glitching.
  const gap = window.innerWidth - document.documentElement.clientWidth;
  body.style.overflow = 'hidden';
  if (gap > 0) body.style.paddingRight = `${gap}px`;
}

function unlockScroll() {
  scrollLocks = Math.max(0, scrollLocks - 1);
  if (scrollLocks > 0 || !restoreBodyStyle) return;
  document.body.style.overflow = restoreBodyStyle.overflow;
  document.body.style.paddingRight = restoreBodyStyle.paddingRight;
  restoreBodyStyle = null;
}

export interface DismissableLayerOptions {
  open: boolean;
  onClose: () => void;
  /** Set false for a layer that must be dismissed by an explicit choice. */
  closeOnEscape?: boolean;
  /** Freeze the page behind. Drawers and modals do; a popover does not. */
  lockScroll?: boolean;
  /** Element to focus on open. Defaults to the first focusable in the panel. */
  initialFocusRef?: RefObject<HTMLElement | null>;
}

export function useDismissableLayer<T extends HTMLElement>(
  panelRef: RefObject<T | null>,
  { open, onClose, closeOnEscape = true, lockScroll: shouldLock = true, initialFocusRef }: DismissableLayerOptions,
) {
  // Kept in a ref so changing the handler between renders does not tear down
  // and rebuild the listeners (which would drop the layer out of the stack).
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });

  useEffect(() => {
    if (!open) return;

    const token = Symbol('layer');
    layerStack.push(token);

    const previouslyFocused = document.activeElement as HTMLElement | null;
    if (shouldLock) lockScroll();

    // Focus after paint: the panel animates in, and focusing an element that
    // is still mid-transform makes some browsers scroll the container.
    const focusFrame = requestAnimationFrame(() => {
      const target =
        initialFocusRef?.current ??
        panelRef.current?.querySelector<HTMLElement>(FOCUSABLE) ??
        panelRef.current;
      target?.focus({ preventScroll: true });
    });

    const onKeyDown = (event: KeyboardEvent) => {
      // Only the innermost layer responds, so a confirmation opened from a
      // drawer closes itself and leaves the drawer standing.
      if (layerStack[layerStack.length - 1] !== token) return;

      if (event.key === 'Escape' && closeOnEscape) {
        event.stopPropagation();
        onCloseRef.current();
        return;
      }

      if (event.key !== 'Tab') return;

      const panel = panelRef.current;
      if (!panel) return;
      const focusable = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
        (el) => el.offsetParent !== null || el === document.activeElement,
      );
      if (focusable.length === 0) {
        // Nothing to tab to: keep focus on the panel rather than letting it
        // escape to the page behind.
        event.preventDefault();
        panel.focus({ preventScroll: true });
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement as HTMLElement | null;

      if (event.shiftKey && (active === first || !panel.contains(active))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', onKeyDown, true);

    return () => {
      cancelAnimationFrame(focusFrame);
      document.removeEventListener('keydown', onKeyDown, true);
      const index = layerStack.indexOf(token);
      if (index !== -1) layerStack.splice(index, 1);
      if (shouldLock) unlockScroll();
      // Return focus to whatever opened this. Guarded because the trigger can
      // legitimately be gone — a row menu whose row was just deleted.
      if (previouslyFocused?.isConnected) previouslyFocused.focus({ preventScroll: true });
    };
  }, [open, closeOnEscape, shouldLock, initialFocusRef, panelRef]);
}
