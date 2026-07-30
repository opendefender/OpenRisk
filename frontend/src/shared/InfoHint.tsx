// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Contextual help (UX-5 / directive §2). A small "?" affordance that explains a
// concept in place, on hover or tap — progressive disclosure instead of a upfront
// product tour. Put it next to the labels users might not immediately grasp
// (security score, ALE, KEV…), so help is available exactly where the question
// arises, and invisible otherwise.

import { useState } from 'react';
import { HelpCircle } from 'lucide-react';

export function InfoHint({ text, className = '' }: { text: string; className?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <span
      className={`relative inline-flex items-center ${className}`}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); setOpen((o) => !o); }}
        className="inline-flex text-ink-muted hover:text-ink transition-colors cursor-help"
        aria-label={text}
      >
        <HelpCircle size={13} />
      </button>
      {open && (
        <>
          {/* tap-away catcher on touch */}
          <span className="fixed inset-0 z-[59]" onClick={(e) => { e.stopPropagation(); setOpen(false); }} aria-hidden="true" />
          <span
            role="tooltip"
            className="absolute z-[60] bottom-full left-1/2 -translate-x-1/2 mb-1.5 w-max max-w-[240px] px-2.5 py-1.5 rounded-[8px] text-[11.5px] leading-snug text-left normal-case tracking-normal font-normal shadow-card-lg"
            style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', color: 'var(--text-secondary)' }}
          >
            {text}
          </span>
        </>
      )}
    </span>
  );
}
