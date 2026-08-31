// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Badge — a read-only status marker.
 *
 * VARIANTS  neutral | accent | success | warning | danger | info |
 *           experimental | unavailable
 *
 * The separation that matters
 * --------------------------
 * A badge takes an `intent` (how it should look) and nothing else. It does NOT
 * know what a risk status is, what a scan result is, or what "P1" means. The
 * mapping from a business enum to an intent is written once per domain and is a
 * plain lookup table — see ./badgeIntents.ts.
 *
 * Why: the moment a badge switches on business values, every new enum value in
 * the backend becomes a UI change, and two features inevitably disagree about
 * whether "in_review" is a warning. Keeping the mapping outside also means a
 * reviewer can read the table and check it against the domain, which is not
 * possible when the logic is a chain of ternaries inside a render.
 *
 * A11Y      Colour is never the only carrier — every badge shows its label as
 *           text (WCAG 1.4.1). `dot` adds a shape, not a replacement.
 *           A badge that is the sole meaning of a row gets `role="status"` via
 *           the `live` prop so a change is announced.
 *
 * WHY NOT the one that came before: components/ui/Badge was a shadcn component
 * pasted in without its theme. Its variants referenced bg-secondary,
 * bg-destructive, text-primary-foreground and ring-ring — none of which exist
 * in this project's Tailwind config — so `variant="destructive"` rendered with
 * no background at all and the intent was invisible.
 */

import type { ReactNode } from 'react';
import { cn } from './cn';

export type BadgeIntent =
  | 'neutral'
  | 'accent'
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'
  | 'experimental'
  | 'unavailable';

export type BadgeSize = 'sm' | 'md';

/* Tinted surface + matching text, both from tokens whose pairing is verified
   in CI. Deliberately not filled: a page of filled badges is a page of alarms,
   and the risk register routinely shows twenty at once. */
const INTENT: Record<BadgeIntent, string> = {
  neutral: 'bg-surface-3 text-fg-secondary border-transparent',
  accent: 'bg-accent-soft text-accent border-transparent',
  success: 'bg-success-surface text-success-text border-transparent',
  warning: 'bg-warning-surface text-warning-text border-transparent',
  danger: 'bg-danger-surface text-danger-text border-transparent',
  info: 'bg-info-surface text-info-text border-transparent',
  /* Outlined rather than tinted: "not finished" is a different KIND of
     statement from "this is warning-severity", and an outline reads as
     provisional. */
  experimental: 'bg-transparent text-accent border-accent-line',
  /* Muted and struck-through-adjacent: the one badge that says "no data here",
     which must never be mistaken for a healthy zero. */
  unavailable: 'bg-transparent text-fg-muted border-subtle',
};

const DOT: Record<BadgeIntent, string> = {
  neutral: 'bg-fg-muted',
  accent: 'bg-accent',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  info: 'bg-info',
  experimental: 'bg-accent',
  unavailable: 'bg-fg-muted',
};

const SIZE: Record<BadgeSize, string> = {
  sm: 'h-(--badge-h) px-(--badge-px) text-2xs gap-1',
  md: 'h-6 px-2.5 text-xs gap-1.5',
};

export interface BadgeProps {
  intent?: BadgeIntent;
  size?: BadgeSize;
  /** Leading dot. A shape cue in addition to the label, never instead of it. */
  dot?: boolean;
  /** Announce changes — for a badge that is the live state of the page. */
  live?: boolean;
  className?: string;
  children: ReactNode;
}

export function Badge({
  intent = 'neutral',
  size = 'sm',
  dot = false,
  live = false,
  className,
  children,
}: BadgeProps) {
  return (
    <span
      role={live ? 'status' : undefined}
      className={cn(
        'inline-flex items-center whitespace-nowrap rounded-full border font-semibold',
        SIZE[size],
        INTENT[intent],
        className,
      )}
    >
      {dot && (
        <span aria-hidden="true" className={cn('h-1.5 w-1.5 shrink-0 rounded-full', DOT[intent])} />
      )}
      {children}
    </span>
  );
}
