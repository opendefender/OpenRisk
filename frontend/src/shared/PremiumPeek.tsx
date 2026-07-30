// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Premium teaser (UX-18/19): a paid feature is shown BLURRED with its benefit and
// a soft CTA — never a hard "you reached your limit" wall. Create desire before
// asking for the payment effort. Meant to be triggered at the three conversion
// moments (after the Aha, at a limit, after a significant win), not sprinkled.

import { type ReactNode } from 'react';
import { Lock, Sparkles } from 'lucide-react';

interface PremiumPeekProps {
  title: string;
  /** What the user gains — framed as time/value saved, not a restriction. */
  benefit: string;
  ctaLabel?: string;
  onUpgrade?: () => void;
  /** The real feature UI, rendered blurred behind the teaser. */
  children: ReactNode;
}

export function PremiumPeek({ title, benefit, ctaLabel = 'Débloquer', onUpgrade, children }: PremiumPeekProps) {
  return (
    <div className="relative rounded-[14px] overflow-hidden" style={{ border: '1px solid var(--border)' }}>
      <div aria-hidden="true" style={{ filter: 'blur(6px)', pointerEvents: 'none', userSelect: 'none', opacity: 0.6 }}>
        {children}
      </div>
      <div
        className="absolute inset-0 flex flex-col items-center justify-center text-center gap-2 p-6"
        style={{ background: 'color-mix(in srgb,var(--bg-app) 55%,transparent)', backdropFilter: 'blur(2px)' }}
      >
        <div className="w-10 h-10 rounded-[12px] flex items-center justify-center" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}>
          <Lock size={18} />
        </div>
        <div className="text-[15px] font-bold text-ink">{title}</div>
        <div className="text-[13px] text-ink-soft max-w-[42ch]">{benefit}</div>
        <button
          onClick={onUpgrade}
          className="mt-1.5 h-[38px] px-4 rounded-[10px] text-[13px] font-semibold text-white inline-flex items-center gap-2"
          style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 3px 12px var(--accent-glow)' }}
        >
          <Sparkles size={15} /> {ctaLabel}
        </button>
      </div>
    </div>
  );
}
