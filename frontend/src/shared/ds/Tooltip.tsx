// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Tooltip — a short label for a control whose meaning is not fully carried by
 * what you can see (an icon-only button, a truncated cell, an abbreviation).
 *
 * What it is not: a place to put content. If the interface needs a paragraph to
 * be understandable, the interface is the problem — use InfoHint for a
 * deliberate progressive-disclosure popover, or fix the label.
 *
 * A11Y      This is the part tooltips usually get wrong (WCAG 1.4.13):
 *           - opens on hover AND on keyboard focus, so it exists for a keyboard
 *             user at all
 *           - Escape dismisses it while the pointer is still over the trigger
 *           - it does not disappear when the pointer moves onto it, so the
 *             content is selectable and does not vanish out from under a
 *             pointer moving diagonally
 *           - the content is the trigger's aria-describedby, not its name — a
 *             tooltip supplements the accessible name, it does not replace it
 *           - it is not focusable itself and traps nothing
 *
 * Positioned with floating-ui, already a dependency: flip when there is no room
 * below, shift to stay on screen, and an offset that clears the control.
 * Outranks modals in the z scale — a tooltip on a control inside a dialog has
 * to be readable.
 */

import {
  cloneElement,
  useCallback,
  useId,
  useState,
  type ReactElement,
  type ReactNode,
} from 'react';
import {
  FloatingPortal,
  autoUpdate,
  flip,
  offset,
  shift,
  useDismiss,
  useFloating,
  useFocus,
  useHover,
  useInteractions,
  useRole,
  safePolygon,
} from '@floating-ui/react';
import { cn } from './cn';

export type TooltipPlacement = 'top' | 'bottom' | 'left' | 'right';

export interface TooltipProps {
  content: ReactNode;
  placement?: TooltipPlacement;
  /** Milliseconds before showing on hover. Zero for a control the user is
   *  already pointing at deliberately. */
  delay?: number;
  /** Skip rendering entirely — for a label that is only sometimes needed
   *  (a cell that is only sometimes truncated). */
  disabled?: boolean;
  children: ReactElement<Record<string, unknown>>;
  className?: string;
}

export function Tooltip({
  content,
  placement = 'top',
  delay = 250,
  disabled = false,
  children,
  className,
}: TooltipProps) {
  const [open, setOpen] = useState(false);
  const id = useId();

  const { refs, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement,
    whileElementsMounted: autoUpdate,
    middleware: [offset(6), flip({ padding: 8 }), shift({ padding: 8 })],
  });

  /* floating-ui's setReference/setFloating are callback-ref SETTERS, not ref
     reads, but reaching them through `refs.` during render trips the React
     refs lint rule. Wrapping them is the pattern already used by RowMenu. */
  const setReference = useCallback(
    (node: HTMLElement | null) => refs.setReference(node),
    [refs],
  );
  const setFloating = useCallback(
    (node: HTMLElement | null) => refs.setFloating(node),
    [refs],
  );

  const interactions = useInteractions([
    // safePolygon keeps the tooltip open while the pointer travels toward it.
    useHover(context, { delay: { open: delay, close: 80 }, handleClose: safePolygon() }),
    useFocus(context),
    useDismiss(context, { escapeKey: true }),
    useRole(context, { role: 'tooltip' }),
  ]);

  if (disabled) return children;

  return (
    <>
      {cloneElement(children, {
        ref: setReference,
        // describedby, never labelledby: the trigger keeps its own name.
        'aria-describedby': open ? id : undefined,
        ...interactions.getReferenceProps(children.props),
      })}
      {open && (
        <FloatingPortal>
          <div
            ref={setFloating}
            id={id}
            style={floatingStyles}
            {...interactions.getFloatingProps()}
            className={cn(
              'z-tooltip max-w-[260px] rounded-sm border border-default bg-surface-2 px-2 py-1',
              'text-2xs leading-snug text-fg-primary shadow-elev-2',
              'motion-safe:animate-or-fadein',
              className,
            )}
          >
            {content}
          </div>
        </FloatingPortal>
      )}
    </>
  );
}
