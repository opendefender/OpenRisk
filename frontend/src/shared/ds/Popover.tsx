// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Popover — a non-modal layer anchored to the control that opened it.
 *
 * THE THREE FLOATING LAYERS, and why they are not interchangeable:
 *
 *   Tooltip   describes the trigger. Hover or focus opens it, it holds no
 *             focusable content, and it disappears the moment you look away.
 *   Popover   holds CONTENT related to the trigger — a filter form, a colour
 *             picker, a small detail card. It is opened by a click, it can be
 *             tabbed into, and the page behind it stays live.
 *   Modal     blocks. Nothing behind it is reachable until it is answered.
 *
 * Putting a form in a Tooltip loses it to a mouse leaving; putting a filter in a
 * Modal stops the user seeing the list they are filtering. The middle case is
 * this component, and it is the one the codebase kept hand-rolling.
 *
 * NON-MODAL IS THE POINT   Focus is NOT trapped. A user can Tab out of an open
 * popover into the page, and that is correct — the content behind is still
 * usable, which is the whole reason not to have used a Modal. Escape closes and
 * returns focus to the trigger; an outside click closes.
 *
 * POSITIONING   `@floating-ui/react`, already a dependency and already used by
 * Tooltip. `flip` and `shift` are what stop a popover on a row near the bottom
 * of a long table from rendering off-screen, which is the failure every
 * hand-rolled version in this codebase had.
 */

import { cloneElement, useState, type ReactElement, type ReactNode } from 'react';
import {
  FloatingFocusManager,
  FloatingPortal,
  autoUpdate,
  flip,
  offset,
  shift,
  useClick,
  useDismiss,
  useFloating,
  useInteractions,
  useRole,
} from '@floating-ui/react';
import { cn } from './cn';

export type PopoverPlacement = 'top' | 'bottom' | 'left' | 'right';

export interface PopoverProps {
  /** The control that opens it. Receives the ref and the open handlers. */
  trigger: ReactElement<Record<string, unknown>>;
  children: ReactNode;
  placement?: PopoverPlacement;
  /** Controlled mode. Omit to let the popover own its state. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Names the layer for assistive tech when it has no visible heading. */
  label?: string;
  className?: string;
}

export function Popover({
  trigger,
  children,
  placement = 'bottom',
  open: controlledOpen,
  onOpenChange,
  label,
  className,
}: PopoverProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : uncontrolledOpen;

  const setOpen = (next: boolean) => {
    if (!isControlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };

  const {
    refs: { setReference, setFloating },
    floatingStyles,
    context,
  } = useFloating({
    open,
    onOpenChange: setOpen,
    placement,
    /* Recomputed while open, so a popover anchored to a row stays anchored when
       the table scrolls underneath it. */
    whileElementsMounted: autoUpdate,
    middleware: [offset(8), flip({ padding: 8 }), shift({ padding: 8 })],
  });

  const { getReferenceProps, getFloatingProps } = useInteractions([
    useClick(context),
    useDismiss(context, { outsidePress: true, escapeKey: true }),
    useRole(context, { role: 'dialog' }),
  ]);

  return (
    <>
      {cloneElement(trigger, {
        ref: setReference,
        ...getReferenceProps(trigger.props),
      })}

      {open && (
        <FloatingPortal>
          {/* modal={false}: focus moves INTO the layer on open and returns to
              the trigger on close, but it is not trapped — the page behind
              stays reachable, which is what makes this a popover and not a
              dialog. */}
          <FloatingFocusManager context={context} modal={false} returnFocus>
            <div
              ref={setFloating}
              style={floatingStyles}
              /* Only aria-label. An aria-labelledby pointing at an id nothing
                 renders is worse than no name at all: it makes the layer
                 announce as unnamed while looking wired up. */
              aria-label={label}
              className={cn(
                'z-popover min-w-48 max-w-sm rounded-md border border-default bg-surface-2',
                'p-3 shadow-overlay outline-none',
                'motion-safe:animate-or-fadein',
                className,
              )}
              {...getFloatingProps()}
            >
              {children}
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </>
  );
}
