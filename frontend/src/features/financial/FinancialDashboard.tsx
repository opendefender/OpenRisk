// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Financial quantification on the October 2026 redesign (#904): the register
// turned into money. Four headline figures (inherent ALE, targeted residual
// ALE, plan cost, 3-year ROSI), the loss-exceedance curve with the board's risk
// appetite, exposure by risk, and the treatment plans ranked by payback.
//
// Kept from the previous page, outside the design's layout: the methodology
// (opened from the subtitle), the display currency (header, admin) and the
// investment simulator (below the design's blocks).

import { useState } from 'react';
import { Link } from 'react-router';
import { ChevronRight } from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { formatNumber } from '../../i18n/format';
import { FeatureGate } from '../../shared/FeatureGate';
import { Select } from '../../shared/ds';
import { PageFrame, PageHeader } from '../../shared/ui';
import { useFeature } from '../billing/useEntitlements';
import { BlockEmpty, BlockError, BlockSkeleton, Panel } from '../dashboard/console/Panel';
import {
  SUPPORTED_CURRENCIES,
  type CurrencyCode,
  type FinancialSummary,
  type Methodology,
} from './financialService';
import { LossExceedanceCard } from './LossExceedanceCard';
import { paybackTone } from './lec';
import { MethodologyModal } from './MethodologyModal';
import { portfolioMethodology } from './methodology';
import { currencyLabel, millions, toDisplay } from './money';
import { SimulatorCard } from './SimulatorCard';
import { useFinancialSummary, useSetCurrency } from './useFinancial';

export function FinancialDashboard() {
  const { t, locale } = useI18n();
  const fin = useFeature('financial_quantification');
  const summary = useFinancialSummary();
  const [methodology, setMethodology] = useState<Methodology | null>(null);
  const data = summary.data;
  const cur = currencyLabel(data?.currency);

  // On a plan without the feature, the blurred preview and the upsell.
  if (!fin.loading && !fin.enabled) {
    return (
      <PageFrame>
        <PageHeader title={t('financial.title')} />
        <FeatureGate feature="financial_quantification">
          <BlockSkeleton lines={6} height={40} />
        </FeatureGate>
      </PageFrame>
    );
  }

  const loading = summary.isLoading || fin.loading;
  const failed = !loading && (summary.isError || !data);
  const empty = !!data && data.total_risks === 0;

  return (
    <PageFrame>
      <PageHeader
        className="!mb-[22px]"
        title={t('financial.title')}
        subtitle={
          data ? (
            <>
              {t('financial.subtitle', { currency: cur })}
              <button
                type="button"
                onClick={() => setMethodology(portfolioMethodology(data, t))}
                className="underline decoration-dotted underline-offset-2 hover:text-ink"
                data-testid="fin-methodology"
              >
                {t('financial.method', {
                  iterations: formatNumber(
                    locale,
                    data.iterations || data.portfolio_loss.iterations,
                  ),
                })}
              </button>
            </>
          ) : undefined
        }
        actions={data ? <CurrencyPicker current={data.currency} /> : undefined}
      />

      {failed ? (
        <Panel className="px-5 py-[18px]">
          <BlockError onRetry={() => void summary.refetch()} />
        </Panel>
      ) : empty ? (
        <Panel className="px-5 py-[18px]">
          <BlockEmpty>{t('financial.emptyRegister')}</BlockEmpty>
        </Panel>
      ) : (
        <>
          <KpiTiles data={data} />
          {data ? (
            <LossExceedanceCard data={data} />
          ) : (
            <Panel className="px-5 py-[18px] mb-4">
              <BlockSkeleton lines={6} height={30} />
            </Panel>
          )}
          <div className="flex flex-wrap gap-4 mb-4">
            <ExposureTable data={data} className="flex-[1.5_1_520px]" />
            <PaybackList data={data} className="flex-[1_1_380px]" />
          </div>
          {data && <SimulatorCard summary={data} onExplain={setMethodology} />}
        </>
      )}

      {methodology && <MethodologyModal m={methodology} onClose={() => setMethodology(null)} />}
    </PageFrame>
  );
}

/* ------------------------------------------------------------ KPI tiles */

function KpiTiles({ data }: { data: FinancialSummary | undefined }) {
  const { t, locale } = useI18n();
  const cur = currencyLabel(data?.currency);
  const m = (xaf: number) => millions(toDisplay(xaf, data?.fx_rate_xaf), locale);
  const rosi = data?.portfolio_rosi_3y_computable
    ? `${data.portfolio_rosi_3y >= 0 ? '+' : '−'}${formatNumber(locale, Math.abs(Math.round(data.portfolio_rosi_3y * 100)))} %`
    : '—';
  const tiles = [
    {
      key: 'inherent',
      label: t('financial.kpi.inherent'),
      value: data ? m(data.total_ale.xaf) : '',
      sub: data ? t('financial.kpi.inherentSub', { currency: cur, count: data.total_risks }) : '',
      color: 'var(--fg-primary)',
    },
    {
      key: 'residual',
      label: t('financial.kpi.residual'),
      value: data ? m(data.total_ale_after.xaf) : '',
      sub: t('financial.kpi.residualSub'),
      color: 'var(--success-text)',
    },
    {
      key: 'plan',
      label: t('financial.kpi.plan'),
      value: data ? m(data.total_remediation.xaf) : '',
      sub: data ? t('financial.kpi.planSub', { currency: cur, count: data.treated_risks }) : '',
      color: 'var(--fg-primary)',
    },
    {
      key: 'rosi',
      label: t('financial.kpi.rosi'),
      value: rosi,
      sub: data?.portfolio_rosi_3y_computable
        ? t('financial.kpi.rosiSub')
        : t('financial.kpi.rosiNone'),
      color: 'var(--fg-primary)',
    },
  ];
  return (
    <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-4 mb-4">
      {tiles.map((k) => (
        <Panel key={k.key} testId={`fin-kpi-${k.key}`} className="px-[18px] py-4">
          <div className="text-[10.5px] tracking-[0.06em] uppercase font-semibold text-ink-muted">
            {k.label}
          </div>
          {data ? (
            <>
              <div
                className="mono text-[30px] font-semibold tracking-[-0.02em] mt-2 leading-tight"
                style={{ color: k.color }}
                data-testid={`fin-kpi-${k.key}-value`}
              >
                {k.value}
              </div>
              <div className="text-[12px] text-ink-muted mt-1">{k.sub}</div>
            </>
          ) : (
            <div className="or-skeleton h-[36px] w-28 rounded-[8px] mt-2" aria-busy="true" />
          )}
        </Panel>
      ))}
    </div>
  );
}

/* ----------------------------------------------------- exposure by risk */

function ExposureTable({
  data,
  className,
}: {
  data: FinancialSummary | undefined;
  className: string;
}) {
  const { t, locale } = useI18n();
  const rows = (data?.top_risks ?? []).filter((r) => r.ale.xaf > 0);
  const m = (xaf: number) => millions(toDisplay(xaf, data?.fx_rate_xaf), locale);
  const th = 'h-[34px] font-semibold border-b border-border-subtle';
  return (
    <Panel testId="fin-exposure" className={`overflow-hidden ${className}`}>
      <div className="px-5 pt-4 pb-2.5">
        <h2 className="m-0 text-[13.5px] font-semibold text-ink">
          {t('financial.exposure.title')}
        </h2>
      </div>
      {!data ? (
        <div className="px-5 pb-4">
          <BlockSkeleton lines={8} height={22} />
        </div>
      ) : rows.length === 0 ? (
        <div className="px-5 pb-3">
          <BlockEmpty>{t('financial.exposure.empty')}</BlockEmpty>
        </div>
      ) : (
        <table className="w-full border-collapse text-[13px] table-fixed">
          <thead>
            <tr className="text-left text-ink-muted text-[10.5px] tracking-[0.06em] uppercase">
              <th className={`${th} px-5`}>{t('financial.exposure.risk')}</th>
              <th className={`${th} px-2 text-right w-[84px]`}>
                {t('financial.exposure.inherent')}
              </th>
              <th className={`${th} px-2 text-right w-[88px]`}>
                {t('financial.exposure.residual')}
              </th>
              <th className={`${th} pl-3 pr-5 w-[20%]`}>{t('financial.exposure.reduction')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const after = r.ale_after?.xaf ?? r.ale.xaf;
              const share = r.ale.xaf > 0 ? (r.ale.xaf - after) / r.ale.xaf : 0;
              const critical = r.criticality.toLowerCase() === 'critical';
              return (
                <tr
                  key={r.id}
                  className="h-10 border-b border-border-subtle"
                  data-testid="fin-exposure-row"
                >
                  <td className="px-5 min-w-0">
                    <Link
                      to={`/risks?focus=${r.id}`}
                      className="flex items-baseline gap-2 min-w-0 text-ink hover:underline"
                      title={r.title}
                    >
                      <span className="mono text-[11.5px] text-ink-muted shrink-0">
                        #{r.id.slice(0, 8)}
                      </span>
                      <span className="truncate">{r.title}</span>
                    </Link>
                  </td>
                  <td
                    className="px-2 text-right mono font-semibold"
                    style={{ color: critical ? 'var(--danger-text)' : 'var(--fg-primary)' }}
                  >
                    {m(r.ale.xaf)}
                  </td>
                  <td className="px-2 text-right mono text-ink-soft">{m(after)}</td>
                  <td className="pl-3 pr-5">
                    <span className="flex items-center gap-2">
                      <span
                        className="flex-1 h-1.5 rounded-[6px] overflow-hidden"
                        style={{ background: 'var(--chart-track)' }}
                      >
                        <span
                          className="block h-full"
                          style={{ width: `${share * 100}%`, background: 'var(--chart-3)' }}
                        />
                      </span>
                      <span className="mono text-[11.5px] text-ink-muted w-9 text-right">
                        {share > 0 ? `−${Math.round(share * 100)}%` : '—'}
                      </span>
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </Panel>
  );
}

/* ------------------------------------------------------ payback ranking */

function PaybackList({
  data,
  className,
}: {
  data: FinancialSummary | undefined;
  className: string;
}) {
  const { t, locale } = useI18n();
  const m = (xaf: number) => millions(toDisplay(xaf, data?.fx_rate_xaf), locale);
  const rows = data?.treatments ?? [];
  return (
    <Panel testId="fin-payback" className={`overflow-hidden ${className}`}>
      <div className="px-5 pt-4 pb-2.5">
        <h2 className="m-0 text-[13.5px] font-semibold text-ink">{t('financial.payback.title')}</h2>
        <p className="m-0 mt-0.5 text-[12px] text-ink-muted">{t('financial.payback.sub')}</p>
      </div>
      {!data ? (
        <div className="px-5 pb-4">
          <BlockSkeleton lines={6} height={28} />
        </div>
      ) : rows.length === 0 ? (
        <div className="px-5 pb-3 border-t border-border-subtle">
          <BlockEmpty>{t('financial.payback.empty')}</BlockEmpty>
        </div>
      ) : (
        rows.map((r) => (
          <Link
            key={r.risk_id}
            to={`/risks?focus=${r.risk_id}&tab=miti`}
            className="grid grid-cols-[1fr_auto_auto] gap-3.5 items-center px-5 py-2.5 border-t border-border-subtle text-[12.5px] text-ink no-underline hover:bg-surface-2"
            data-testid="fin-payback-row"
          >
            <span className="min-w-0">
              <span className="block font-medium truncate">{r.title}</span>
              <span className="block text-[11.5px] text-ink-muted">
                #{r.risk_id.slice(0, 8)} ·{' '}
                {t('financial.payback.row', { cost: m(r.cost.xaf), reduction: m(r.reduction.xaf) })}
              </span>
            </span>
            <span className="mono font-semibold" style={{ color: paybackTone(r.payback_months) }}>
              {t('financial.payback.months', {
                n: formatNumber(locale, Math.round(r.payback_months)),
              })}
            </span>
            <ChevronRight size={14} className="text-ink-muted" aria-hidden="true" />
          </Link>
        ))
      )}
    </Panel>
  );
}

/* ---------------------------------------------------- display currency */

function CurrencyPicker({ current }: { current: string }) {
  const { t } = useI18n();
  // A tenant setting: the route is admin/root, whose token carries "*".
  const allowed = useAuthStore((s) => s.hasPermission)('*');
  const setCur = useSetCurrency();
  if (!allowed) return null;
  return (
    <Select
      value={SUPPORTED_CURRENCIES.includes(current as CurrencyCode) ? current : 'XAF'}
      onChange={(e) => setCur.mutate(e.target.value as CurrencyCode)}
      disabled={setCur.isPending}
      aria-label={t('financial.currencyLabel')}
      title={t('financial.currencyLabel')}
      className="w-auto"
      data-testid="fin-currency"
    >
      {SUPPORTED_CURRENCIES.map((c) => (
        <option key={c} value={c}>
          {currencyLabel(c)}
        </option>
      ))}
    </Select>
  );
}
