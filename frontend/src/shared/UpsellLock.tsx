// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Soft upsell (UX-6 / directive §5). Never a hard wall: show a BLURRED preview of
// the paid feature behind a clear benefit + CTA, so the value is visible and the
// upgrade feels like unlocking, not being blocked. `moment` labels which of the
// three good push moments this is (after the Aha, at a limit, after a win).
//
// NOTE: there is no billing/plan backend yet, so the lock is presentational — the
// CTA is wired by the caller (e.g. a toast / a future /billing route). Do not gate
// anything security-relevant on this; it's a growth surface, not authorization.

import type { ReactNode } from 'react';
import { Lock, ArrowRight, Sparkles } from 'lucide-react';

interface UpsellLockProps {
  title: string;
  description: string;
  ctaLabel: string;
  onUpgrade: () => void;
  /** Optional context label, e.g. "Premium" or "Plan limit reached". */
  moment?: string;
  /** The real (or representative) feature UI, shown blurred behind the overlay. */
  children: ReactNode;
}

export function UpsellLock({ title, description, ctaLabel, onUpgrade, moment, children }: UpsellLockProps) {
  return (
    <div className="relative overflow-hidden rounded-[16px]">
      <div className="pointer-events-none select-none" style={{ filter: 'blur(7px)', opacity: 0.45 }} aria-hidden="true">
        {children}
      </div>
      <div
        className="absolute inset-0 flex items-center justify-center p-6 text-center"
        style={{ background: 'radial-gradient(120% 90% at 50% 30%, transparent, var(--bg-primary) 78%)' }}
      >
        <div className="max-w-sm" style={{ animation: 'or-fadeup .4s ease' }}>
          <div className="w-12 h-12 rounded-2xl mx-auto mb-4 flex items-center justify-center text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-2))', boxShadow: '0 4px 16px var(--accent-glow)' }}>
            <Lock size={22} />
          </div>
          {moment && (
            <div className="inline-flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[.06em] text-accent mb-2">
              <Sparkles size={12} /> {moment}
            </div>
          )}
          <h3 className="disp text-[21px] font-bold tracking-tight text-ink mb-2">{title}</h3>
          <p className="text-[13.5px] text-ink-soft leading-relaxed mb-5">{description}</p>
          <button
            onClick={onUpgrade}
            className="h-10 px-5 rounded-[11px] inline-flex items-center gap-2 text-[13.5px] font-semibold text-white transition-[filter] hover:brightness-110"
            style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 4px 14px var(--accent-glow)' }}
          >
            {ctaLabel} <ArrowRight size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}
