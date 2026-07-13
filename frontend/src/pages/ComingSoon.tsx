// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Graceful placeholder for design-language screens whose backend isn't built yet
// (Leaderboard, Infrastructure, Simulations, Asset Universe). Keeps the shell from
// dead-ending — an explicit empty state with icon + title + subtitle, per §8.

import { useLocation, useNavigate } from 'react-router-dom';
import { ArrowLeft, Sparkles } from 'lucide-react';
import { ALL_NAV_ITEMS } from '../shared/navModel';
import { useUIStrings } from '../shared/uiStrings';

export default function ComingSoon() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const L = useUIStrings();

  const item = ALL_NAV_ITEMS.find((it) => it.path === pathname);
  const label = item ? L[item.labelKey] : L.soon;
  const Icon = item?.icon ?? Sparkles;

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="min-h-full flex flex-col items-center justify-center px-6 py-20 text-center" style={{ animation: 'or-fadeup .4s ease' }}>
        <div
          className="w-16 h-16 rounded-2xl flex items-center justify-center mb-6"
          style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}
        >
          <Icon size={30} strokeWidth={1.6} />
        </div>
        <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-accent mb-2">{L.soon}</div>
        <h1 className="disp text-[24px] font-bold tracking-tight text-ink mb-2">{label}</h1>
        <p className="text-[14px] text-ink-soft max-w-sm mb-8">{L.soonSub}</p>
        <button
          onClick={() => navigate('/')}
          className="h-[38px] px-4 rounded-[10px] inline-flex items-center gap-2 text-[13px] font-semibold text-ink hover:bg-hover transition-colors"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
        >
          <ArrowLeft size={16} /> {L.n_dashboard}
        </button>
      </div>
    </div>
  );
}
