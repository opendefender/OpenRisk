// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Drawer — a side panel for the detail of a row you are still looking at.
 *
 * Modal or drawer? A modal ASKS something and blocks until answered. A drawer
 * SHOWS something that belongs to the list behind it: an asset, a risk, an
 * incident, an audit record. That is why the drawer keeps the list visible and,
 * on a wide screen, gives up covering it entirely (see below).
 *
 * ANATOMY   scrim
 *           └ panel  ── header (title, subtitle, actions, close)
 *                     ── body (scrolls)
 *                     ── footer (optional, pinned)
 *
 * SIDES     right (default) | left. There is no top/bottom: this product has
 *           no case for one, and an unused variant is a variant that will be
 *           misused.
 *
 * SIZES     sm 380 | md 480 (default) | lg 620 | xl 780
 *
 * MOTION    Slides in from its edge on --motion-panel (--dur-panel), and back
 *           out on the slower --dur-base/--ease-in — deliberately not the
 *           same pair reversed: a panel that has just travelled the width of
 *           the viewport reads as yanked off screen on the fast timing a
 *           small dialog exits on. Slide is the right verb here precisely
 *           because it says where the thing came from and where it will go
 *           back to — the continuity a modal's fade cannot express.
 *
 *           Same mount/exit contract as Modal (#751 phase 5): `open` going
 *           false does not unmount the panel, it stays at
 *           `data-state="closed"` until `useExitTimer` reports it gone, so a
 *           reopen mid-exit reverses the transition instead of restarting one.
 *
 * WIDE SCREENS  At >=1920px a drawer marked `docked` stops overlaying and sits
 *           beside the list instead (the .or-md-* rules in index.css). On a
 *           monitor with the room, covering half the table to show one row is
 *           a waste of the room.
 *
 * A11Y      Identical contract to Modal — trap, restore, Escape, scroll lock —
 *           because it is the same hook, keyed on `open` (not the exit timer)
 *           so it releases at the start of the close. A drawer is a dialog to
 *           assistive technology, so it says so.
 */

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from './cn';
import { useDismissableLayer } from './useDismissableLayer';
import { useExitTimer } from './useExitTimer';
import { DRAWER_EXIT_MS } from './overlayMotion';
import { Button } from './Button';

export type DrawerSide = 'left' | 'right';
export type DrawerSize = 'sm' | 'md' | 'lg' | 'xl';

const SIZE: Record<DrawerSize, string> = {
  sm: 'w-[min(var(--drawer-w-sm),100vw)]',
  md: 'w-[min(var(--drawer-w-md),100vw)]',
  lg: 'w-[min(var(--drawer-w-lg),100vw)]',
  xl: 'w-[min(var(--drawer-w-xl),100vw)]',
};

export interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  subtitle?: ReactNode;
  side?: DrawerSide;
  size?: DrawerSize;
  /** Header controls — status pill, overflow menu, primary action. */
  actions?: ReactNode;
  footer?: ReactNode;
  closeLabel?: string;
  className?: string;
  children: ReactNode;
}

export function Drawer({
  open,
  onClose,
  title,
  subtitle,
  side = 'right',
  size = 'md',
  actions,
  footer,
  closeLabel = 'Close',
  className,
  children,
}: DrawerProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const subtitleId = subtitle ? `${titleId}-sub` : undefined;

  useDismissableLayer(panelRef, { open, onClose });

  // Same contract as Modal: stays mounted through its exit, `data-state`
  // drives the CSS, closing is synchronous and opening is deferred a frame —
  // see Modal.tsx for the full reasoning, identical here.
  const mounted = useExitTimer(open, DRAWER_EXIT_MS);
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
      className={cn(
        'or-scrim fixed inset-0 z-drawer flex',
        side === 'right' ? 'justify-end' : 'justify-start',
      )}
      data-state={state}
      style={{
        background: 'var(--surface-overlay)',
        backdropFilter: 'blur(var(--overlay-blur))',
        WebkitBackdropFilter: 'blur(var(--overlay-blur))',
      }}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={subtitleId}
        tabIndex={-1}
        data-state={state}
        data-side={side}
        className={cn(
          'or-drawer-panel flex h-full flex-col bg-surface-1 shadow-overlay outline-none',
          side === 'right' ? 'border-l' : 'border-r',
          'border-default',
          SIZE[size],
          className,
        )}
      >
        <header className="flex items-start gap-3 border-b border-subtle px-5 py-4">
          <div className="min-w-0 flex-1">
            <h2 id={titleId} className="truncate text-md font-semibold text-fg-primary">
              {title}
            </h2>
            {subtitle && (
              <p id={subtitleId} className="mt-0.5 truncate text-sm text-fg-secondary">
                {subtitle}
              </p>
            )}
          </div>
          {actions}
          <Button variant="ghost" size="sm" icon={X} aria-label={closeLabel} onClick={onClose} />
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">{children}</div>

        {footer && (
          <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-subtle bg-surface-sunken px-5 py-3">
            {footer}
          </footer>
        )}
      </div>
    </div>,
    document.body,
  );
}
