// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Contextual first-encounter help (UX-14): a tooltip that appears the FIRST time
// the user hovers an element and then never repeats (remembered in localStorage).
// No sequential product tour — help arrives at the moment of need. For always-on
// definitions of jargon, use the <Term> glossary instead.

import { useState, type ReactNode } from 'react';

const SEEN_KEY = 'openrisk_hints_seen';

function seen(id: string): boolean {
  try {
    return (JSON.parse(localStorage.getItem(SEEN_KEY) || '{}') as Record<string, boolean>)[id] === true;
  } catch {
    return false;
  }
}
function markSeen(id: string): void {
  try {
    const m = JSON.parse(localStorage.getItem(SEEN_KEY) || '{}') as Record<string, boolean>;
    m[id] = true;
    localStorage.setItem(SEEN_KEY, JSON.stringify(m));
  } catch {
    /* ignore quota */
  }
}

interface HintProps {
  /** Stable id so the hint shows only once, ever. */
  id: string;
  text: string;
  side?: 'top' | 'bottom';
  children: ReactNode;
}

export function Hint({ id, text, side = 'top', children }: HintProps) {
  const [show, setShow] = useState(false);
  const alreadySeen = seen(id);

  const onEnter = () => {
    if (!alreadySeen) setShow(true);
  };
  const onLeave = () => {
    if (show) {
      setShow(false);
      markSeen(id);
    }
  };

  return (
    <span className="relative inline-flex" onMouseEnter={onEnter} onMouseLeave={onLeave} onFocus={onEnter} onBlur={onLeave}>
      {children}
      {show && (
        <span
          role="tooltip"
          className="absolute z-50 left-1/2 -translate-x-1/2 whitespace-normal text-[12px] leading-snug rounded-[9px] px-2.5 py-1.5 pointer-events-none"
          style={{
            [side === 'top' ? 'bottom' : 'top']: 'calc(100% + 8px)',
            width: 'max-content',
            maxWidth: 240,
            background: 'var(--bg-elevated)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border-strong)',
            boxShadow: 'var(--shadow-lg)',
            animation: 'or-fadein .15s ease',
          }}
        >
          {text}
        </span>
      )}
    </span>
  );
}
