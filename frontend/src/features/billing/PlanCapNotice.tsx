// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// A plan cap met where something would be created (D-078: Free does almost
// everything, in small quantities). Under the cap it says nothing, or, with
// `showUsage`, a quiet "1/2" counter so the tenant sees the cap coming. At the
// cap it says what the plan includes, the numbers and the way out, before the
// server refuses the create with 402 limit_reached.

import { useNavigate } from 'react-router';
import { Lock } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useLimit } from './useEntitlements';
import { planCapMessage, planCapUsage, type PlanCapKey } from './planCap';

export function PlanCapNotice({
  limitKey,
  showUsage = false,
}: {
  limitKey: PlanCapKey;
  showUsage?: boolean;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const { limit, used, reached } = useLimit(limitKey);

  if (!reached) {
    if (!showUsage || limit < 0) return null;
    return (
      <p data-testid={`plan-usage-${limitKey}`} className="text-[12px] text-ink-muted">
        {planCapUsage(lang === 'fr', limitKey, limit, used)}
      </p>
    );
  }

  return (
    <div
      role="status"
      data-testid={`plan-cap-${limitKey}`}
      className="flex items-start gap-2.5 rounded-[11px] px-3.5 py-3 text-[12.5px] leading-relaxed"
      style={{ background: 'var(--accent-soft)', border: '1px solid var(--border)' }}
    >
      <Lock size={15} className="mt-0.5 shrink-0 text-accent-strong" aria-hidden="true" />
      <div className="flex-1 min-w-0 text-ink">
        {planCapMessage(lang === 'fr', limitKey, limit, used)}{' '}
        <button
          type="button"
          onClick={() => navigate('/settings?tab=billing')}
          className="font-semibold underline underline-offset-2 text-accent-strong"
        >
          {tr('Voir les plans', 'See plans')}
        </button>
      </div>
    </div>
  );
}
