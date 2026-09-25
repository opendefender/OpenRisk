// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

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
 * MOTION    One keyline for the whole tablist, which slides to the active tab
 *           on --dur-slow / --ease-out (#751). Same duration both ways: a tab
 *           change is one reversible motion, not an open and a close. The
 *           first placement does not animate, and reduced motion removes the
 *           slide entirely (global rule in index.css) — the keyline just moves.
 *
 * Deep-linking is the caller's job: several screens carry the active tab in
 * `?tab=`, and a component that owned that would have to know about the router.
 *
 * `id` is required, and TabPanel's `tabsId` must match it. That looks like
 * friction until you notice the alternative: with the id generated privately
 * inside Tabs, `aria-controls` pointed at an element id that no TabPanel could
 * ever have, so the tab/panel association was broken in every consumer. axe
 * catches it as aria-valid-attr-value; a person never would.
 */

import { useLayoutEffect, useRef, type KeyboardEvent, type ReactNode } from 'react';
import { cn } from './cn';
import { Badge } from './Badge';

export interface TabItem<T extends string = string> {
  id: T;
  label: ReactNode;
  /** A count next to the label — rows behind this tab, findings, members. */
  count?: number;
  icon?: ReactNode;
  disabled?: boolean;
  /** data-testid for the tab button, for suites that already target one. */
  testId?: string;
}

export interface TabsProps<T extends string = string> {
  /** Shared with every TabPanel of this group; ties tab to panel. */
  id: string;
  items: readonly TabItem<T>[];
  value: T;
  onChange: (id: T) => void;
  /** Accessible name for the tablist, e.g. "Risk detail sections". */
  label: string;
  className?: string;
}

export function Tabs<T extends string = string>({
  id: baseId,
  items,
  value,
  onChange,
  label,
  className,
}: TabsProps<T>) {
  const listRef = useRef<HTMLDivElement>(null);
  const keylineRef = useRef<HTMLSpanElement>(null);

  /* Place the keyline under the active tab, and keep it there when the list
     resizes (a count badge changes width, the drawer is resized). Written
     straight to the element's style rather than through state: a measurement
     that feeds a re-render is a second paint for nothing. */
  useLayoutEffect(() => {
    const list = listRef.current;
    const keyline = keylineRef.current;
    if (!list || !keyline) return;

    const place = () => {
      const tab = list.querySelector<HTMLElement>(`[data-tab-id="${CSS.escape(value)}"]`);
      if (!tab || tab.offsetWidth === 0) {
        keyline.style.opacity = '0';
        return;
      }
      // inset-x-2: the keyline stops 8px short of each edge of its tab.
      keyline.style.width = `${tab.offsetWidth - 16}px`;
      keyline.style.transform = `translateX(${tab.offsetLeft + 8}px)`;
      keyline.style.opacity = '1';
      // Arm the transition only after the first placement has painted, so the
      // keyline does not fly in from the left edge on mount.
      if (!keyline.dataset.ready) {
        requestAnimationFrame(() => {
          keyline.dataset.ready = 'true';
        });
      }
    };

    place();
    const observer = new ResizeObserver(place);
    observer.observe(list);
    return () => observer.disconnect();
  }, [value, items]);

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
      className={cn(
        'relative flex items-center gap-1 overflow-x-auto border-b border-subtle',
        className,
      )}
    >
      {items.map((item) => {
        const active = item.id === value;
        return (
          <button
            key={item.id}
            role="tab"
            type="button"
            data-tab-id={item.id}
            data-testid={item.testId}
            id={`${baseId}-tab-${item.id}`}
            aria-selected={active}
            /* Only the active tab points at a panel. Inactive panels are not
               rendered at all (see TabPanel), so an aria-controls on every tab
               would reference element ids that do not exist — which is a real
               violation, not a technicality: assistive technology follows that
               reference and finds nothing. */
            aria-controls={active ? `${baseId}-panel-${item.id}` : undefined}
            /* Roving tabindex: exactly one tab is in the tab order. */
            tabIndex={active ? 0 : -1}
            disabled={item.disabled}
            onClick={() => onChange(item.id)}
            className={cn(
              'relative -mb-px inline-flex shrink-0 items-center gap-2 whitespace-nowrap px-3 py-2.5',
              'text-sm font-medium transition-colors duration-fast ease-out',
              'disabled:pointer-events-none disabled:opacity-45',
              active ? 'text-fg-primary' : 'text-fg-secondary hover:text-fg-primary',
            )}
          >
            {item.icon}
            {item.label}
            {typeof item.count === 'number' && (
              <Badge intent={active ? 'accent' : 'neutral'}>{item.count}</Badge>
            )}
          </button>
        );
      })}
      {/* The keyline. A sibling of the tabs rather than a border on each, so it
          can travel between them, and so it sits flush on the tablist rule
          (-bottom-px) without the two fighting over the same pixel row. */}
      <span
        ref={keylineRef}
        aria-hidden="true"
        data-testid="tabs-keyline"
        className={cn(
          'pointer-events-none absolute -bottom-px left-0 h-(--keyline-w) rounded-full bg-accent opacity-0',
          'data-[ready=true]:transition-[transform,width] data-[ready=true]:duration-slow data-[ready=true]:ease-out',
        )}
      />
    </div>
  );
}

export interface TabPanelProps {
  /** Must equal the `id` given to the Tabs of this group. */
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
