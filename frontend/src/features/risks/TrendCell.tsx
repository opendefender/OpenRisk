// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The register's trend cell: the direction of the score against its baseline
// 30 days ago. Rising is the bad direction (danger), falling the good one. The
// icon never carries it alone: every state has a text label for screen readers,
// and "no history" is a dash that says so.

import { Minus, TrendingDown, TrendingUp } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import type { RiskTrend } from './riskRegisterService';

export function TrendCell({ trend }: { trend: RiskTrend | null }) {
  const { t } = useI18n();
  switch (trend) {
    case 'up':
      return (
        <span className="text-danger-text inline-flex" title={t('riskRegister.trend.up')}>
          <TrendingUp size={15} aria-hidden="true" />
          <span className="sr-only">{t('riskRegister.trend.up')}</span>
        </span>
      );
    case 'down':
      return (
        <span className="text-success-text inline-flex" title={t('riskRegister.trend.down')}>
          <TrendingDown size={15} aria-hidden="true" />
          <span className="sr-only">{t('riskRegister.trend.down')}</span>
        </span>
      );
    case 'flat':
      return (
        <span className="text-ink-muted inline-flex" title={t('riskRegister.trend.flat')}>
          <Minus size={15} aria-hidden="true" />
          <span className="sr-only">{t('riskRegister.trend.flat')}</span>
        </span>
      );
    case null:
    default:
      return (
        <span className="text-ink-muted text-[12px]" title={t('riskRegister.trend.none')}>
          <span aria-hidden="true">—</span>
          <span className="sr-only">{t('riskRegister.trend.none')}</span>
        </span>
      );
  }
}
