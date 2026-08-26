// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

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
 * MOTION    Scrim fades; panel fades and rises 8px, both on --motion-enter.
 *           The panel does NOT scale from 0.9 — a dialog that zooms reads as a
 *           notification. Under prefers-reduced-motion it appears, in place.
 *
 * A11Y      role="dialog" aria-modal, labelled by its title and described by
 *           its subtitle; focus trapped, restored on close; Escape closes;
 *           the page behind is frozen. All of that comes from
 *           useDismissableLayer, so it is identical in every dialog.
 *
 * Rendered in a portal to document.body so no ancestor's overflow, transform
 * or stacking context can clip it — the reason "the dropdown is cut off inside
 * the drawer" happens.
 */

import { useId, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from './cn';
import { useDismissableLayer } from './useDismissableLayer';
import { Button } from './Button';

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl';

const SIZE: Record<ModalSize, string> = {
  sm: 'max-w-[var(--modal-w-sm)]',
  md: 'max-w-[var(--modal-w-md)]',
  lg: 'max-w-[var(--modal-w-lg)]',
  xl: 'max-w-[var(--modal-w-xl)]',
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
  closeLabel = 'Close',
  className,
  children,
}: ModalProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const subtitleId = subtitle ? `${titleId}-sub` : undefined;

  useDismissableLayer(panelRef, { open, onClose, closeOnEscape: dismissable });

  if (!open) return null;

  return createPortal(
    <div
      className="fixed inset-0 z-modal flex items-center justify-center p-4 motion-safe:animate-or-fadein"
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
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={subtitleId}
        tabIndex={-1}
        className={cn(
          'flex w-full flex-col overflow-hidden bg-surface-2 shadow-overlay outline-none',
          'rounded-[var(--modal-radius)] border border-default',
          'motion-safe:animate-or-rise',
          'max-h-[var(--modal-max-h)]',
          SIZE[size],
          className,
        )}
      >
        <header className="flex items-start gap-3 border-b border-subtle px-[var(--modal-padding)] py-4">
          {leading}
          <div className="min-w-0 flex-1">
            <h2 id={titleId} className="truncate text-md font-semibold text-text-primary">
              {title}
            </h2>
            {subtitle && (
              <p id={subtitleId} className="mt-0.5 text-sm text-text-secondary">
                {subtitle}
              </p>
            )}
          </div>
          {dismissable && (
            <Button variant="ghost" size="sm" icon={X} aria-label={closeLabel} onClick={onClose} />
          )}
        </header>

        {/* The only scrolling region. */}
        <div className="min-h-0 flex-1 overflow-y-auto px-[var(--modal-padding)] py-4">
          {children}
        </div>

        {footer && (
          <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-subtle bg-surface-1 px-[var(--modal-padding)] py-3">
            {footer}
          </footer>
        )}
      </div>
    </div>,
    document.body,
  );
}
