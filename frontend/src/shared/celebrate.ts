// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Micro-victory feedback (docs/UX_CHARTER UX-32, UI_ELEVATION §4/§9c). A sober,
// ~900ms confetti burst fired once per milestone (first risk, first control, first
// report). Pure DOM, no dependency; respects prefers-reduced-motion (the CSS layer
// hides .or-confetti-layer when reduced motion is requested).

export type Milestone = 'first_risk' | 'first_control' | 'first_report' | 'first_evidence';

const SEEN_KEY = 'openrisk_milestones';
const COLORS = ['var(--accent)', 'var(--accent-2)', 'var(--low)', 'var(--high)', 'var(--info)'];

function seen(): Record<string, boolean> {
  try {
    return JSON.parse(localStorage.getItem(SEEN_KEY) || '{}');
  } catch {
    return {};
  }
}

/** Fire a sober confetti burst. Safe to call anywhere (no-op server-side). */
export function confetti(count = 80): void {
  if (typeof document === 'undefined') return;
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;
  const layer = document.createElement('div');
  layer.className = 'or-confetti-layer';
  for (let i = 0; i < count; i++) {
    const p = document.createElement('i');
    const left = Math.random() * 100;
    p.style.left = `${left}%`;
    p.style.background = COLORS[i % COLORS.length];
    p.style.setProperty('--cx', `${(Math.random() - 0.5) * 240}px`);
    p.style.animationDelay = `${Math.random() * 120}ms`;
    p.style.animationDuration = `${800 + Math.random() * 500}ms`;
    layer.appendChild(p);
  }
  document.body.appendChild(layer);
  window.setTimeout(() => layer.remove(), 1600);
}

/**
 * Celebrate a milestone the first time it happens. Returns true if it fired.
 * Frequent/reversible actions should NOT use this — they get the discreet "✓"
 * affordance instead (UX-08).
 */
export function celebrate(milestone: Milestone): boolean {
  const s = seen();
  if (s[milestone]) return false;
  s[milestone] = true;
  try {
    localStorage.setItem(SEEN_KEY, JSON.stringify(s));
  } catch {
    /* ignore quota */
  }
  confetti();
  return true;
}
