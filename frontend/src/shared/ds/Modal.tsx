// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Modal — a dialog that takes over until it is answered.
 *
 * ANATOMY   scrim
 *           └ panel  ── header (title, optional subtitle, close)
 *                     ── body (the only part that scrolls)
 *                     ── footer (actions, pinned)
 *
 * The header/body/footer split is not cosmetic. A dialog laid out as one
 * scrolling column puts its submit button below the fold on a laptop, which is
 * the single most reported UI bug in this product's history. Here the panel is
 * capped at --modal-max-h, the body scrolls, and the footer cannot leave.
 *
 * SIZES     sm 380 | md 520 (default) | lg 720 | xl 960
 *
 * MOTION    Scrim fades; panel fades and rises 8px, on --motion-enter in and
 *           --motion-exit out. The panel does NOT scale from 0.9 — a dialog
 *           that zooms reads as a notification. Under prefers-reduced-motion
 *           it appears, in place (index.css's global kill switch zeroes every
 *           transition, so the state flip below lands instantly).
 *
 *           `open` going false does not unmount the panel: it stays mounted
 *           at `data-state="closed"` for its exit transition and is only
 *           removed once `useExitTimer` reports it gone (#751 phase 5 — the
 *           previous `if (!open) return null` cut every close instantly,
 *           which is why a create/edit modal that closed on mutation success
 *           never had an exit to play). Driving the CSS off `data-state`
 *           rather than a mount/unmount keyframe means a reopen mid-exit
 *           reverses the same transition instead of restarting one.
 *
 * A11Y      role="dialog" aria-modal, labelled by its title and described by
 *           its subtitle; focus trapped, restored on close; Escape closes;
 *           the page behind is frozen. All of that comes from
 *           useDismissableLayer, keyed on `open` (not the exit timer) so the
 *           trap releases and focus returns at the START of the close, not
 *           after the panel finishes fading out.
 *
 * Rendered in a portal to document.body so no ancestor's overflow, transform
 * or stacking context can clip it — the reason "the dropdown is cut off inside
 * the drawer" happens.
 */

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from './cn';
import { useDismissableLayer } from './useDismissableLayer';
import { useExitTimer } from './useExitTimer';
import { MODAL_EXIT_MS } from './overlayMotion';
import { Button } from './Button';

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl';

const SIZE: Record<ModalSize, string> = {
  sm: 'max-w-(--modal-w-sm)',
  md: 'max-w-(--modal-w-md)',
  lg: 'max-w-(--modal-w-lg)',
  xl: 'max-w-(--modal-w-xl)',
};

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  subtitle?: ReactNode;
  size?: ModalSize;
  /** Pinned action row. Omit for a dialog whose body carries its own actions. */
  footer?: ReactNode;
  /** Shown to the left of the title — an icon or status marker. */
  leading?: ReactNode;
  /** False while a submission is in flight: the user cannot dismiss a dialog
   *  whose action is already running and half-applied. */
  dismissable?: boolean;
  /**
   * `alertdialog` for a dialog that asks a question with exactly two answers.
   * The role makes a screen reader announce the whole dialog on open rather than
   * waiting to be explored, which is right for "this deletes 14 controls" and
   * wrong for a form. Added for AlertDialog (#443 PR 2); OPTIONAL and defaulting
   * to `dialog`, so every existing call site keeps its exact behaviour and
   * Résolution 1's freeze on these APIs holds.
   */
  role?: 'dialog' | 'alertdialog';
  closeLabel?: string;
  className?: string;
  children: ReactNode;
}

export function Modal({
  open,
  onClose,
  title,
  subtitle,
  size = 'md',
  footer,
  leading,
  dismissable = true,
  role = 'dialog',
  closeLabel = 'Close',
  className,
  children,
}: ModalProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const subtitleId = subtitle ? `${titleId}-sub` : undefined;

  useDismissableLayer(panelRef, { open, onClose, closeOnEscape: dismissable });

  // Stays mounted through its own exit transition instead of cutting the
  // instant `open` goes false.
  const mounted = useExitTimer(open, MODAL_EXIT_MS);
  // `data-state`, not `open` directly, drives the CSS: closing is applied
  // synchronously (same render `open` goes false, matching
  // useDismissableLayer's own immediate focus release), while opening is
  // deferred a frame so the browser paints the closed state at least once —
  // without that the enter transition has nothing to animate from.
  const [state, setState] = useState<'open' | 'closed'>('closed');
  if (!open && state === 'open') setState('closed');
  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => setState('open'));
    return () => cancelAnimationFrame(frame);
  }, [open]);

  if (!mounted) return null;

  return createPortal(
    <div
      className="or-scrim fixed inset-0 z-modal flex items-center justify-center p-4"
      data-state={state}
      style={{
        background: 'var(--surface-overlay)',
        backdropFilter: 'blur(var(--overlay-blur))',
        WebkitBackdropFilter: 'blur(var(--overlay-blur))',
      }}
      /* Clicking the scrim dismisses; clicking the panel must not. The check is
         on the target rather than a stopPropagation on the panel so that a drag
         that STARTS inside the panel and ends on the scrim (selecting text past
         the edge) does not close the dialog. */
      onMouseDown={(event) => {
        if (dismissable && event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role={role}
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={subtitleId}
        tabIndex={-1}
        data-state={state}
        className={cn(
          'or-modal-panel flex w-full flex-col overflow-hidden bg-surface-2 shadow-overlay outline-none',
          'rounded-(--modal-radius) border border-default',
          'max-h-(--modal-max-h)',
          SIZE[size],
          className,
        )}
      >
        <header className="flex items-start gap-3 border-b border-subtle px-(--modal-padding) py-4">
          {leading}
          <div className="min-w-0 flex-1">
            <h2 id={titleId} className="truncate text-md font-semibold text-fg-primary">
              {title}
            </h2>
            {subtitle && (
              <p id={subtitleId} className="mt-0.5 text-sm text-fg-secondary">
                {subtitle}
              </p>
            )}
          </div>
          {dismissable && (
            <Button variant="ghost" size="sm" icon={X} aria-label={closeLabel} onClick={onClose} />
          )}
        </header>

        {/* The only scrolling region. */}
        <div className="min-h-0 flex-1 overflow-y-auto px-(--modal-padding) py-4">{children}</div>

        {footer && (
          <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-subtle bg-surface-1 px-(--modal-padding) py-3">
            {footer}
          </footer>
        )}
      </div>
    </div>,
    document.body,
  );
}
