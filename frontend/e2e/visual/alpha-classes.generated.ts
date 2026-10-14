// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * GENERATED — do not edit by hand.
 *   npm run generate:alpha-classes
 *
 * Every Tailwind opacity-modifier class the product uses on a design token.
 * Two jobs at once: it is the list e2e/visual/alpha-modifiers.spec.ts asserts
 * over, and — because theme.css scans this directory via `@source` — naming
 * each class here is what makes Tailwind emit it BARE, rather than only behind
 * the `focus:` / `hover:` variant the product happens to use it with.
 *
 * Regenerate when a call site is added or removed; the spec fails if this list
 * and the source tree disagree.
 */

export const ALPHA_CLASSES = [
  'bg-danger/10',
  'bg-primary/10',
  'ring-primary/40',
  'ring-primary/50',
  'bg-accent/10',
  'bg-success/10',
  'bg-surface-1/5',
  'bg-warning/10',
  'border-danger/20',
  'text-ink-muted/80',
  'bg-accent/8',
  'bg-success/20',
  'bg-surface-0/40',
  'bg-surface-1/10',
  'bg-surface-1/30',
  'bg-surface-3/10',
  'border-border-strong/20',
  'border-border-strong/5',
  'border-danger/50',
  'border-primary/60',
  'border-success/30',
  'border-success/50',
  'border-warning/20',
  'border-warning/30',
  'ring-accent/30',
  'shadow-primary/20',
] as const;
