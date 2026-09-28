// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Menu — a list of ACTIONS hanging off a trigger.
 *
 * MENU OR SELECT? The most common mistake this replaces. A `Select` picks a
 * VALUE and the choice persists in the control. A menu fires an ACTION and
 * closes: duplicate, export, archive, revoke. The row-actions "…" button in
 * every table is a menu; the severity dropdown next to it is a select. Using a
 * menu for a value means the current value is invisible, and using a select for
 * an action means the last action taken is displayed as if it were state.
 *
 * KEYBOARD, which is the whole reason not to hand-roll this. The ARIA menu
 * pattern is a roving tab stop, not a list of tab stops: the menu is ONE stop,
 * Up/Down move within it, Home/End jump, typing letters jumps to a matching
 * item, Enter/Space activate, Escape closes and returns focus to the trigger.
 * `useListNavigation` and `useTypeahead` from @floating-ui/react implement that
 * contract; every hand-rolled version in this codebase implemented some of it.
 *
 * DESTRUCTIVE ITEMS are marked `destructive`, which colours them AND sorts them
 * to the end behind a separator. An irreversible action adjacent to a benign one
 * is a mis-click waiting to happen, and the visual difference alone does not
 * help someone navigating by keyboard — the separation does.
 *
 * MOTION (#751) The surface grows from its trigger: scale 0.97 → 1 with a fade
 * on --motion-enter, and leaves on --motion-exit to 0.99, so closing is quicker
 * and smaller than opening. The origin follows the RESOLVED placement, so a menu
 * that flipped above its trigger grows upward. It animates `scale`, not
 * `transform`, because floating-ui positions the layer with `transform` and the
 * two would overwrite each other. Reduced motion drops the transition through the
 * global rule in index.css; the menu still opens and closes.
 */

import { cloneElement, useRef, useState, type ReactElement } from 'react';
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
  useListNavigation,
  useRole,
  useTransitionStatus,
  useTypeahead,
  type Placement,
} from '@floating-ui/react';
import { type LucideIcon } from 'lucide-react';
import { cn } from './cn';

export interface MenuItem {
  label: string;
  onSelect: () => void;
  icon?: LucideIcon;
  /** Rendered in danger colour, after a separator, at the end of the list. */
  destructive?: boolean;
  disabled?: boolean;
}

export interface MenuProps {
  trigger: ReactElement<Record<string, unknown>>;
  items: ReadonlyArray<MenuItem>;
  placement?: 'bottom-start' | 'bottom-end' | 'top-start' | 'top-end';
  /**
   * Overrides the accessible name. By DEFAULT the menu is named by its trigger,
   * which is the conventional pattern and usually right — a "Row actions" button
   * opens a menu announced as "Row actions". Pass this only when the trigger's
   * name is not enough on its own, e.g. an icon-only "…" button in a table where
   * every row has one.
   */
  label?: string;
  className?: string;
}

/* Must match --dur-fast, the duration inside --motion-exit: it is how long the
   layer stays mounted once closed. Shorter clips the exit; longer leaves an
   invisible menu catching nothing for a beat (pointer-events are off). */
const EXIT_MS = 120;

/* The corner of the menu nearest its trigger. */
function originFor(placement: Placement): string {
  const [side, align] = placement.split('-');
  const y = side === 'top' ? 'bottom' : 'top';
  const x = align === 'end' ? 'right' : align === 'start' ? 'left' : 'center';
  return `${y} ${x}`;
}

export function Menu({ trigger, items, placement = 'bottom-end', label, className }: MenuProps) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const listRef = useRef<Array<HTMLElement | null>>([]);

  /* Destructive actions go last, whatever order the caller passed them in. The
     sort is stable, so everything else keeps the caller's ordering. */
  const ordered = [...items].sort(
    (a, b) => Number(Boolean(a.destructive)) - Number(Boolean(b.destructive)),
  );
  const firstDestructive = ordered.findIndex((i) => i.destructive);

  const {
    refs: { setReference, setFloating },
    floatingStyles,
    context,
  } = useFloating({
    open,
    onOpenChange: setOpen,
    placement,
    whileElementsMounted: autoUpdate,
    middleware: [offset(6), flip({ padding: 8 }), shift({ padding: 8 })],
  });

  const { isMounted, status } = useTransitionStatus(context, { duration: EXIT_MS });

  const listNav = useListNavigation(context, {
    listRef,
    activeIndex,
    onNavigate: setActiveIndex,
    loop: true,
  });

  const typeahead = useTypeahead(context, {
    listRef: useRef(ordered.map((i) => i.label)),
    activeIndex,
    onMatch: setActiveIndex,
  });

  const { getReferenceProps, getFloatingProps, getItemProps } = useInteractions([
    useClick(context),
    useDismiss(context, { outsidePress: true, escapeKey: true }),
    useRole(context, { role: 'menu' }),
    listNav,
    typeahead,
  ]);

  function activate(item: MenuItem) {
    if (item.disabled) return;
    setOpen(false);
    item.onSelect();
  }

  return (
    <>
      {cloneElement(trigger, {
        ref: setReference,
        ...getReferenceProps(trigger.props),
      })}

      {isMounted && (
        <FloatingPortal>
          {/* A menu DOES trap focus while open: unlike a popover it holds only
              actions, and tabbing out of a half-open action list into the page
              behind is how a user loses the menu without meaning to. Escape and
              selecting both return focus to the trigger. */}
          <FloatingFocusManager context={context} modal returnFocus>
            <div
              ref={setFloating}
              data-status={status}
              style={{
                ...floatingStyles,
                transformOrigin: originFor(context.placement),
                transition:
                  status === 'close'
                    ? 'opacity var(--motion-exit), scale var(--motion-exit)'
                    : 'opacity var(--motion-enter), scale var(--motion-enter)',
              }}
              className={cn(
                'z-dropdown min-w-44 overflow-hidden rounded-md border border-default',
                'bg-surface-2 py-1 shadow-overlay outline-none',
                'data-[status=initial]:scale-97 data-[status=initial]:opacity-0',
                'data-[status=close]:pointer-events-none data-[status=close]:scale-99 data-[status=close]:opacity-0',
                className,
              )}
              {...getFloatingProps()}
              /* AFTER the spread, and clearing aria-labelledby with it.
                 getFloatingProps() sets aria-labelledby to the trigger, which
                 wins over aria-label — so a `label` placed before the spread was
                 silently inert. Found by the test, which asked for the name it
                 had passed and got the trigger's instead. */
              {...(label ? { 'aria-label': label, 'aria-labelledby': undefined } : {})}
            >
              {ordered.map((item, index) => {
                const Icon = item.icon;
                return (
                  <div key={item.label}>
                    {index === firstDestructive && index > 0 && (
                      <div role="separator" className="my-1 h-px bg-border-subtle" />
                    )}
                    <button
                      type="button"
                      role="menuitem"
                      disabled={item.disabled}
                      ref={(node) => {
                        listRef.current[index] = node;
                      }}
                      tabIndex={activeIndex === index ? 0 : -1}
                      className={cn(
                        'flex w-full items-center gap-2 px-3 text-left text-sm',
                        'min-h-(--control-h-sm)',
                        'disabled:opacity-55 disabled:pointer-events-none',
                        item.destructive ? 'text-danger-text' : 'text-fg-primary',
                        activeIndex === index && 'bg-surface-3',
                      )}
                      {...getItemProps({ onClick: () => activate(item) })}
                    >
                      {Icon && <Icon size={14} aria-hidden="true" />}
                      {item.label}
                    </button>
                  </div>
                );
              })}
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </>
  );
}
