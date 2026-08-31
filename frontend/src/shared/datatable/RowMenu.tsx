// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The row "⋯" menu — the bug this file exists to kill.
//
// The old menus were `position: absolute` inside the row's `<td>`, so on the
// last rows of a table the menu was clipped by the card's `overflow: hidden`
// and its items were unreachable without scrolling the page first. A menu whose
// items you cannot see is a dead control.
//
// The fix is structural, not cosmetic: the menu renders in a portal at the end
// of <body> (no ancestor can clip it) and is positioned by @floating-ui with
// flip + shift + size, so it opens upwards when there is no room below and is
// never taller than the viewport. Guaranteed by e2e:
// tests/e2e/datatable.spec.ts › "row menu on the last row of 200 is fully visible".

import { useCallback, useRef, useState } from 'react';
import { MoreHorizontal } from 'lucide-react';
import {
  FloatingFocusManager,
  FloatingPortal,
  autoUpdate,
  flip,
  offset,
  shift,
  size,
  useClick,
  useDismiss,
  useFloating,
  useInteractions,
  useListNavigation,
  useRole,
} from '@floating-ui/react';
import type { RowAction } from './types';

interface RowMenuProps<T> {
  row: T;
  actions: RowAction<T>[];
  label: string;
}

export function RowMenu<T>({ row, actions, label }: RowMenuProps<T>) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const listRef = useRef<Array<HTMLElement | null>>([]);

  const visible = actions.filter((a) => !a.hidden?.(row));

  const { refs: anchor, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement: 'bottom-end',
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(6),
      // Flip above the trigger when the row is near the bottom of the viewport.
      flip({ padding: 8, fallbackPlacements: ['top-end', 'bottom-start', 'top-start'] }),
      // Slide back inside the viewport rather than overflowing sideways.
      shift({ padding: 8 }),
      // Never taller than the space available; scroll inside the menu instead.
      size({
        padding: 8,
        apply({ availableHeight, elements }) {
          Object.assign(elements.floating.style, {
            maxHeight: `${Math.max(120, availableHeight)}px`,
            overflowY: 'auto',
          });
        },
      }),
    ],
  });

  const click = useClick(context);
  const dismiss = useDismiss(context, { outsidePress: true, escapeKey: true });
  const role = useRole(context, { role: 'menu' });
  const listNav = useListNavigation(context, {
    listRef,
    activeIndex,
    onNavigate: setActiveIndex,
    loop: true,
    focusItemOnOpen: true,
  });
  const { getReferenceProps, getFloatingProps, getItemProps } = useInteractions([click, dismiss, role, listNav]);



  // floating-ui hands back *callback ref setters*, not ref objects. Wrapping
  // them keeps the JSX free of member access, which the react-hooks/refs rule
  // (rightly) forbids for real refs.
  const setAnchor = useCallback((node: HTMLElement | null) => anchor.setReference(node), [anchor]);
  const setPopover = useCallback((node: HTMLElement | null) => anchor.setFloating(node), [anchor]);

  if (visible.length === 0) return null;

  return (
    <>
      <button
        ref={setAnchor}
        {...getReferenceProps({
          onClick: (e) => e.stopPropagation(),
          onKeyDown: (e) => e.stopPropagation(),
        })}
        type="button"
        aria-label={label}
        data-testid="row-menu-trigger"
        data-open={open || undefined}
        className="w-7 h-7 rounded-[7px] inline-flex items-center justify-center text-ink-muted hover:bg-hover transition-colors"
        style={open ? { background: 'var(--bg-hover)', color: 'var(--fg-primary)' } : undefined}
      >
        <MoreHorizontal size={17} />
      </button>

      {open && (
        <FloatingPortal>
          <FloatingFocusManager context={context} modal={false} initialFocus={-1} returnFocus>
            <div
              ref={setPopover}
              style={{
                ...floatingStyles,
                zIndex: 120,
                minWidth: 186,
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border)',
                borderRadius: 11,
                boxShadow: 'var(--shadow-lg)',
                padding: 4,
                animation: 'or-scalein .12s cubic-bezier(.2,.8,.2,1)',
              }}
              data-testid="row-menu"
              {...getFloatingProps({ onClick: (e) => e.stopPropagation() })}
            >
              {visible.map((action, i) => {
                const Icon = action.icon;
                const disabled = action.disabled?.(row) ?? false;
                return (
                  <div key={action.key}>
                    {action.separatorBefore && i > 0 && (
                      <div style={{ height: 1, background: 'var(--border)', margin: '4px 0' }} />
                    )}
                    <button
                      type="button"
                      role="menuitem"
                      ref={(node) => {
                        listRef.current[i] = node;
                      }}
                      tabIndex={activeIndex === i ? 0 : -1}
                      disabled={disabled}
                      {...getItemProps({
                        onClick: () => {
                          if (disabled) return;
                          setOpen(false);
                          action.onSelect(row);
                        },
                      })}
                      className="w-full flex items-center gap-2.5 px-3 py-2 rounded-[8px] text-[13px] font-medium text-left transition-colors hover:bg-hover disabled:opacity-50 disabled:pointer-events-none"
                      style={{ color: action.danger ? 'var(--critical)' : 'var(--fg-primary)' }}
                    >
                      {Icon && <Icon size={15} />}
                      {action.label}
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
