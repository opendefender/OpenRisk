// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Annualised exposure card of the redesigned dashboard (#901): the portfolio
// ALE in millions of the tenant's display currency, split by criticality band,
// with the residual after the plan and the simulated worst plausible year
// (P95). Every figure comes from GET /analytics/financial.

import { Link } from 'react-router';
import { useI18n } from '../../../hooks/useI18n';
import { useFeature } from '../../billing/useEntitlements';
import { useFinancialSummary } from '../../financial/useFinancial';
import { currencyLabel, millions, toDisplay } from '../../financial/money';
import { BlockEmpty, BlockError, BlockSkeleton, Eyebrow, HeaderLink, Panel } from './Panel';

const BANDS = [
  { key: 'critical', color: 'var(--risk-critical)' },
  { key: 'high', color: 'var(--risk-high)' },
  { key: 'medium', color: 'var(--risk-moderate)' },
  { key: 'low', color: 'var(--risk-low)' },
] as const;

const PLAN_NAME: Record<string, string> = {
  free: 'Free',
  pro: 'Pro',
  business: 'Business',
  enterprise: 'Enterprise',
};

export function ExposureCard({ className = '' }: { className?: string }) {
  const { t, locale } = useI18n();
  const feature = useFeature('financial_quantification');
  const summary = useFinancialSummary();
  const locked = !feature.loading && !feature.enabled;
  const data = locked ? undefined : summary.data;
  const currency = currencyLabel(data?.currency);
  const disp = (xaf: number) => toDisplay(xaf, data?.fx_rate_xaf);

  return (
    <Panel testId="dash-exposure" className={`p-5 flex flex-col ${className}`}>
      <div className="flex justify-between items-center">
        <Eyebrow>{t('dashboard.exposure.label')}</Eyebrow>
        <HeaderLink to="/analytics/financial">{t('dashboard.exposure.link')}</HeaderLink>
      </div>

      {locked ? (
        <div className="mt-4">
          <BlockEmpty>
            {t('dashboard.exposure.locked', { plan: PLAN_NAME[feature.requiredPlan] ?? 'Business' })}
          </BlockEmpty>
          <Link
            to="/settings?tab=billing"
            className="text-[12.5px] font-semibold text-accent-strong hover:underline"
          >
            {t('dashboard.exposure.lockedCta')}
          </Link>
        </div>
      ) : summary.isLoading || feature.loading ? (
        <div className="mt-4">
          <BlockSkeleton lines={4} />
        </div>
      ) : summary.isError || !data ? (
        <BlockError onRetry={() => void summary.refetch()} />
      ) : data.quantified_risks === 0 ? (
        <BlockEmpty>{t('dashboard.exposure.empty')}</BlockEmpty>
      ) : (
        <>
          <div className="flex items-baseline gap-2 mt-2.5">
            <span
              className="mono text-[44px] font-semibold leading-none tracking-[-0.03em] text-ink"
              data-testid="dash-ale"
            >
              {millions(disp(data.total_ale.xaf), locale)}
            </span>
            <span className="text-[14px] font-medium text-ink-muted">
              {t('dashboard.exposure.unit', { currency })}
            </span>
          </div>

          <ExposureBands
            buckets={BANDS.map((b) => {
              const bucket = data.by_criticality.find((c) => c.criticality === b.key);
              return { ...b, value: disp(bucket?.ale.xaf ?? 0) };
            })}
          />

          <div className="mt-auto pt-[18px] grid gap-2 text-[12.5px]">
            <div className="flex justify-between border-t border-border-subtle pt-2.5">
              <span className="text-ink-soft">{t('dashboard.exposure.residual')}</span>
              <span className="mono font-semibold text-ink">
                {t('dashboard.exposure.millions', {
                  value: millions(disp(data.total_ale_after.xaf), locale),
                })}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-ink-soft">{t('dashboard.exposure.p95')}</span>
              <span className="mono font-semibold text-ink">
                {data.portfolio_loss.p95
                  ? t('dashboard.exposure.millions', {
                      value: millions(data.portfolio_loss.p95.value, locale),
                    })
                  : '—'}
              </span>
            </div>
          </div>
        </>
      )}
    </Panel>
  );
}

function ExposureBands({
  buckets,
}: {
  buckets: { key: string; color: string; value: number }[];
}) {
  const { t, locale } = useI18n();
  return (
    <>
      <div className="flex h-2 rounded-[8px] overflow-hidden mt-[18px] gap-[2px]" aria-hidden="true">
        {buckets.map((b) => (
          <span
            key={b.key}
            title={`${t(`dashboard.band.${b.key}`)} : ${millions(b.value, locale)} M`}
            style={{ flex: Math.max(b.value, 1), background: b.color }}
          />
        ))}
      </div>
      <div className="flex gap-3.5 flex-wrap mt-2">
        {buckets.map((b) => (
          <span key={b.key} className="flex items-center gap-1.5 text-[11.5px] text-ink-soft">
            <span className="w-2 h-2 rounded-[2px]" style={{ background: b.color }} />
            {t(`dashboard.band.${b.key}`)}{' '}
            <span className="mono text-ink-muted">{millions(b.value, locale)}</span>
          </span>
        ))}
      </div>
    </>
  );
}
