// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Tabs — switching between views of the same subject.
 *
 * The active tab is marked by the OpenRisk keyline: a 2px accent rule on the
 * bottom edge, not a filled pill. It is the same device as the active nav item
 * and the focused panel, which is what makes "where am I" one idea across the
 * product rather than three.
 *
 * A11Y      Full WAI-ARIA tabs pattern, which is mostly about the keyboard:
 *           - roving tabindex, so Tab enters the tablist once and moves ON to
 *             the panel, rather than walking through eight tabs
 *           - Left/Right move between tabs and wrap; Home/End jump to the ends
 *           - activation follows focus (automatic), which is correct here
 *             because switching a tab is cheap and local
 *           - aria-controls / aria-labelledby tie each tab to its panel
 *
 * Deep-linking is the caller's job: several screens carry the active tab in
 * `?tab=`, and a component that owned that would have to know about the router.
 */

import { useId, useRef, type KeyboardEvent, type ReactNode } from 'react';
import { cn } from './cn';
import { Badge } from './Badge';

export interface TabItem<T extends string = string> {
  id: T;
  label: ReactNode;
  /** A count next to the label — rows behind this tab, findings, members. */
  count?: number;
  icon?: ReactNode;
  disabled?: boolean;
}

export interface TabsProps<T extends string = string> {
  items: readonly TabItem<T>[];
  value: T;
  onChange: (id: T) => void;
  /** Accessible name for the tablist, e.g. "Risk detail sections". */
  label: string;
  className?: string;
}

export function Tabs<T extends string = string>({
  items,
  value,
  onChange,
  label,
  className,
}: TabsProps<T>) {
  const baseId = useId();
  const listRef = useRef<HTMLDivElement>(null);

  const enabled = items.filter((item) => !item.disabled);

  function move(delta: number) {
    if (enabled.length === 0) return;
    const current = enabled.findIndex((item) => item.id === value);
    // Wraps: at the last tab, Right goes back to the first. A dead end at the
    // edge of a tablist is a small thing that makes a keyboard user check
    // whether the app is broken.
    const next = enabled[(current + delta + enabled.length) % enabled.length];
    onChange(next.id);
    focusTab(next.id);
  }

  function focusTab(id: string) {
    requestAnimationFrame(() => {
      listRef.current
        ?.querySelector<HTMLButtonElement>(`[data-tab-id="${CSS.escape(id)}"]`)
        ?.focus();
    });
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    switch (event.key) {
      case 'ArrowRight':
        event.preventDefault();
        move(1);
        break;
      case 'ArrowLeft':
        event.preventDefault();
        move(-1);
        break;
      case 'Home':
        event.preventDefault();
        if (enabled[0]) {
          onChange(enabled[0].id);
          focusTab(enabled[0].id);
        }
        break;
      case 'End': {
        event.preventDefault();
        const last = enabled[enabled.length - 1];
        if (last) {
          onChange(last.id);
          focusTab(last.id);
        }
        break;
      }
    }
  }

  return (
    <div
      ref={listRef}
      role="tablist"
      aria-label={label}
      onKeyDown={onKeyDown}
      className={cn('flex items-center gap-1 overflow-x-auto border-b border-subtle', className)}
    >
      {items.map((item) => {
        const active = item.id === value;
        return (
          <button
            key={item.id}
            role="tab"
            type="button"
            data-tab-id={item.id}
            id={`${baseId}-tab-${item.id}`}
            aria-selected={active}
            aria-controls={`${baseId}-panel-${item.id}`}
            /* Roving tabindex: exactly one tab is in the tab order. */
            tabIndex={active ? 0 : -1}
            disabled={item.disabled}
            onClick={() => onChange(item.id)}
            className={cn(
              'relative -mb-px inline-flex shrink-0 items-center gap-2 whitespace-nowrap px-3 py-2.5',
              'text-sm font-medium transition-colors duration-fast ease-out',
              'disabled:pointer-events-none disabled:opacity-45',
              active
                ? 'text-text-primary'
                : 'text-text-secondary hover:text-text-primary',
            )}
          >
            {item.icon}
            {item.label}
            {typeof item.count === 'number' && (
              <Badge intent={active ? 'accent' : 'neutral'}>{item.count}</Badge>
            )}
            {/* The keyline. A sibling element rather than a border so it can sit
                flush with the tablist rule underneath without the two fighting
                over the same pixel row. */}
            <span
              aria-hidden="true"
              className={cn(
                'absolute inset-x-2 bottom-0 h-[var(--keyline-w)] rounded-full transition-opacity duration-fast ease-out',
                active ? 'bg-accent opacity-100' : 'opacity-0',
              )}
            />
          </button>
        );
      })}
    </div>
  );
}

export interface TabPanelProps {
  /** Must match the TabItem id and the Tabs instance it belongs to. */
  tabsId: string;
  id: string;
  active: boolean;
  children: ReactNode;
  className?: string;
}

/**
 * The panel half of the pattern. Rendered only when active — the alternative
 * (render all, hide with CSS) leaves every panel's queries running and its
 * focusable content reachable by screen readers.
 */
export function TabPanel({ tabsId, id, active, children, className }: TabPanelProps) {
  if (!active) return null;
  return (
    <div
      role="tabpanel"
      id={`${tabsId}-panel-${id}`}
      aria-labelledby={`${tabsId}-tab-${id}`}
      /* Focusable so that Tab out of the tablist lands in the content, which is
         where the user was going. */
      tabIndex={0}
      className={cn('outline-none', className)}
    >
      {children}
    </div>
  );
}
