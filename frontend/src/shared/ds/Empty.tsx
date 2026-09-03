// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Empty — the single empty state for the whole app.
 *
 * A fresh tenant is mostly empty states, so they are not an edge case to be
 * handled with a shrug — they are the product's first impression. Three
 * divergent EmptyState components used to exist (shared/, components/shared/,
 * and one inlined in shared/ui.tsx), each with different prop names and a
 * different idea of whether an action was required. They are now this one, and
 * keeping it that way is the point: a fourth is a regression, not a feature.
 *
 * THE VARIANT IS THE WHOLE POINT.
 *
 *   first-use      Nothing here YET. This is a tutorial: say in one sentence
 *                  what the page is for, and offer the action that fills it.
 *                  Never a sad "no data" — the user has not failed at anything.
 *   no-results     There IS data, the filter excluded it. Offer a way back out.
 *   error          We could not load. Offer a retry.
 *   no-permission  It exists, this user may not see it. Say who to ask.
 *
 * The distinction matters because the four cases have opposite remedies, and
 * collapsing them into one grey "Aucune donnée" is what makes a fresh install
 * feel broken rather than new.
 *
 * WHY IT LIVES HERE (D-021, 2026-09-02). It was `src/shared/EmptyState.tsx`
 * under AGPL-3.0. `shared/ds/` is Apache-2.0 (D-014, D-016) and may not depend
 * on or re-export AGPL code — the dependency runs one way only. That left three
 * options: relicense and move, build a second generic `Empty` beside this one,
 * or accept a design system with no empty state. Only this one leaves the
 * product with ONE empty state AND a complete primitive layer.
 *
 * `src/shared/EmptyState.tsx` is now a re-export shim under its original names,
 * so all 24 importers are untouched — #443 Résolution 1 point 4 forbids
 * rewriting call sites.
 */

import type { ReactNode } from 'react';
import { Inbox, SearchX, AlertTriangle, Lock, ExternalLink, type LucideIcon } from 'lucide-react';

export type EmptyVariant = 'first-use' | 'no-results' | 'error' | 'no-permission';

/**
 * A translucent wash of `color` over whatever is behind it.
 *
 * Inlined rather than imported from `shared/riskColors`, which is the AGPL core:
 * an Apache-2.0 file may not depend on it, and the licence-boundary script reads
 * headers rather than imports, so that violation would have passed CI silently.
 * There is nothing to share here anyway — this is one generic `color-mix` with
 * no risk semantics in it, unlike the rest of `riskColors`, which maps
 * criticality and framework names and is product knowledge that stays AGPL.
 */
function tint(color: string, pct: number): string {
  return `color-mix(in srgb, ${color} ${pct}%, transparent)`;
}

interface VariantStyle {
  icon: LucideIcon;
  /** Token driving the icon tile's tint. */
  tone: string;
  /** first-use and error earn a tinted tile; the quieter states stay neutral. */
  tinted: boolean;
}

const VARIANTS: Record<EmptyVariant, VariantStyle> = {
  'first-use': { icon: Inbox, tone: 'var(--accent)', tinted: true },
  'no-results': { icon: SearchX, tone: 'var(--fg-muted)', tinted: false },
  error: { icon: AlertTriangle, tone: 'var(--critical)', tinted: true },
  'no-permission': { icon: Lock, tone: 'var(--fg-muted)', tinted: false },
};

export interface EmptyProps {
  variant?: EmptyVariant;
  /** Overrides the variant's default glyph. */
  icon?: LucideIcon;
  title: string;
  /** For first-use, this is the tutorial line: what is this page for? */
  description?: string;
  /** The action that resolves the state — creating the first record, clearing
   *  the filter, retrying. Strongly encouraged on first-use and no-results. */
  primaryAction?: ReactNode;
  secondaryAction?: ReactNode;
  /** Documentation deep-link, rendered as a quiet trailing link. */
  learnMoreHref?: string;
  learnMoreLabel?: string;
  className?: string;
}

export function Empty({
  variant = 'first-use',
  icon,
  title,
  description,
  primaryAction,
  secondaryAction,
  learnMoreHref,
  learnMoreLabel,
  className = '',
}: EmptyProps) {
  const v = VARIANTS[variant];
  const Icon = icon ?? v.icon;
  const hasActions = Boolean(primaryAction || secondaryAction);

  return (
    <div
      // Hooks for the empty-states E2E sweep, which asserts that a fresh tenant
      // renders one of these on every screen instead of fabricated numbers.
      data-testid="empty-state"
      data-variant={variant}
      role="status"
      className={`flex flex-col items-center justify-center text-center py-16 px-6 ${className}`}
      style={{ animation: 'or-fadein .3s ease' }}
    >
      <div
        className="w-16 h-16 rounded-2xl flex items-center justify-center mb-5"
        style={{
          background: v.tinted ? tint(v.tone, 12) : 'var(--bg-hover)',
          color: v.tone,
        }}
      >
        <Icon size={28} strokeWidth={1.7} />
      </div>

      <div className="text-md font-semibold text-ink mb-1.5">{title}</div>

      {description && (
        <div className="text-sm text-ink-soft max-w-sm leading-relaxed">{description}</div>
      )}

      {hasActions && (
        <div className="flex items-center justify-center gap-2.5 flex-wrap mt-5">
          {primaryAction}
          {secondaryAction}
        </div>
      )}

      {learnMoreHref && (
        <a
          href={learnMoreHref}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 text-xs text-ink-muted hover:text-ink transition-colors mt-4"
        >
          {learnMoreLabel ?? 'En savoir plus'}
          <ExternalLink size={13} strokeWidth={1.8} />
        </a>
      )}
    </div>
  );
}
