// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The framework cap (D-076, #783): every framework is on every hosted plan, and
// the plan sets how many are active at once. When the tenant is at its cap this
// says so where a framework would be created, with the numbers and the way out,
// before the server refuses the create with 402 limit_reached.

import { useNavigate } from 'react-router';
import { Lock } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useLimit } from '../billing/useEntitlements';
import { frameworkCapMessage } from './frameworkCap';

export function FrameworkCapNotice() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const { limit, used, reached } = useLimit('frameworks');
  if (!reached) return null;

  return (
    <div
      role="status"
      data-testid="framework-cap"
      className="flex items-start gap-2.5 rounded-[11px] px-3.5 py-3 text-[12.5px] leading-relaxed"
      style={{ background: 'var(--accent-soft)', border: '1px solid var(--border)' }}
    >
      <Lock size={15} className="mt-0.5 shrink-0 text-accent-strong" aria-hidden="true" />
      <div className="flex-1 min-w-0 text-ink">
        {frameworkCapMessage(lang === 'fr', limit, used)}{' '}
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
