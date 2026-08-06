// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The single empty state for the whole app.
//
// A fresh tenant is mostly empty states, so they are not an edge case to be
// handled with a shrug — they are the product's first impression. Three
// divergent EmptyState components used to exist (this one, components/shared/,
// and one inlined in shared/ui.tsx), each with different prop names and a
// different idea of whether an action was required. They are now this one.
//
// The variant is the whole point:
//
//   first-use      Nothing here YET. This is a tutorial: say in one sentence
//                  what the page is for, and offer the action that fills it.
//                  Never a sad "no data" — the user has not failed at anything.
//   no-results     There IS data, the filter excluded it. Offer a way back out.
//   error          We could not load. Offer a retry.
//   no-permission  It exists, this user may not see it. Say who to ask.
//
// The distinction matters because the four cases have opposite remedies, and
// collapsing them into one grey "Aucune donnée" is what makes a fresh install
// feel broken rather than new.

import type { ReactNode } from 'react';
import { Inbox, SearchX, AlertTriangle, Lock, ExternalLink, type LucideIcon } from 'lucide-react';
import { softFill } from './riskColors';

export type EmptyStateVariant = 'first-use' | 'no-results' | 'error' | 'no-permission';

interface VariantStyle {
  icon: LucideIcon;
  /** Token driving the icon tile's tint. */
  tone: string;
  /** first-use and error earn a tinted tile; the quieter states stay neutral. */
  tinted: boolean;
}

const VARIANTS: Record<EmptyStateVariant, VariantStyle> = {
  'first-use': { icon: Inbox, tone: 'var(--accent)', tinted: true },
  'no-results': { icon: SearchX, tone: 'var(--text-muted)', tinted: false },
  error: { icon: AlertTriangle, tone: 'var(--critical)', tinted: true },
  'no-permission': { icon: Lock, tone: 'var(--text-muted)', tinted: false },
};

export interface EmptyStateProps {
  variant?: EmptyStateVariant;
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

export function EmptyState({
  variant = 'first-use',
  icon,
  title,
  description,
  primaryAction,
  secondaryAction,
  learnMoreHref,
  learnMoreLabel,
  className = '',
}: EmptyStateProps) {
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
          background: v.tinted ? softFill(v.tone, 12) : 'var(--bg-hover)',
          color: v.tone,
        }}
      >
        <Icon size={28} strokeWidth={1.7} />
      </div>

      <div className="text-[15px] font-semibold text-ink mb-1.5">{title}</div>

      {description && (
        <div className="text-[13px] text-ink-soft max-w-sm leading-relaxed">{description}</div>
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
          className="inline-flex items-center gap-1.5 text-[12.5px] text-ink-muted hover:text-ink transition-colors mt-4"
        >
          {learnMoreLabel ?? 'En savoir plus'}
          <ExternalLink size={13} strokeWidth={1.8} />
        </a>
      )}
    </div>
  );
}
