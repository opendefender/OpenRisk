// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Courbe de dépassement des pertes" (#904): the probability that a year's
// total loss exceeds an amount, for the register as it stands and once the
// treatment plans are carried out, with the board's risk appetite as a
// threshold. The curve comes from the API's percentiles (one shared Monte
// Carlo run); the appetite is a tenant setting an administrator moves here.
//
// The mockup's chart has no axis titles; this one names both axes (#899).

import { useEffect, useMemo, useRef, useState } from 'react';
import { isAxiosError } from 'axios';
import { toast } from 'sonner';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { formatNumber } from '../../i18n/format';
import { BlockEmpty, Panel } from '../dashboard/console/Panel';
import type { FinancialSummary } from './financialService';
import { axisMax, curvePath, exceedance, ticks } from './lec';
import { currencyLabel, toDisplay } from './money';
import { useSetAppetite } from './useFinancial';

const C_INH = 'var(--chart-5)';
const C_RES = 'var(--chart-3)';
// Plot box inside the 900 × 284 viewBox: the design's, with room for axis titles.
const BOX = { x0: 64, x1: 880, y0: 16, y1: 230 };
const SAVE_DELAY_MS = 600;

function snapTo(v: number, step: number, max: number): number {
  return Math.min(max, Math.max(step, Math.round(v / step) * step));
}

export function LossExceedanceCard({ data }: { data: FinancialSummary }) {
  const { t, locale } = useI18n();
  const canSet = useAuthStore((s) => s.hasPermission)('*');
  const save = useSetAppetite();
  const cur = currencyLabel(data.currency);
  const rate = data.fx_rate_xaf;
  const lec = data.loss_exceedance;

  // Everything below is in display-currency millions; the API speaks XAF.
  const inh = useMemo(
    () => (lec ? lec.inherent.map((x) => toDisplay(x, rate) / 1e6) : []),
    [lec, rate],
  );
  const res = useMemo(
    () => (lec ? lec.residual.map((x) => toDisplay(x, rate) / 1e6) : []),
    [lec, rate],
  );
  const stored =
    data.risk_appetite_xaf != null ? toDisplay(data.risk_appetite_xaf, rate) / 1e6 : null;

  // The axis follows the stored appetite, not the one being dragged, so the
  // scale does not move under the pointer.
  const max = useMemo(() => axisMax(inh, stored), [inh, stored]);
  const step = max / 50;
  // While the slider moves it shows the value under the pointer; otherwise the
  // stored appetite (optimistically updated, rolled back on refusal), or the
  // median year as a starting point when none is set.
  const [drag, setDrag] = useState<number | null>(null);
  const draft = drag ?? snapTo(stored ?? inh[50] ?? step, step, max);

  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );
  const onSlide = (v: number) => {
    if (!canSet) return;
    setDrag(v);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      timer.current = null;
      save.mutate(v * 1e6 * (rate > 0 ? rate : 1), {
        // Back to the stored value, unless the slider has moved on since.
        onSettled: () => {
          if (!timer.current) setDrag(null);
        },
        onSuccess: () => toast.success(t('financial.lec.saved')),
        onError: (e) =>
          toast.error(
            isAxiosError(e) && e.response?.status === 403
              ? t('financial.lec.forbidden')
              : t('financial.lec.saveFailed'),
          ),
      });
    }, SAVE_DELAY_MS);
  };

  const fmtM = (v: number) => formatNumber(locale, v, { maximumFractionDigits: v < 10 ? 1 : 0 });
  const pct = (p: number) => `${Math.round(p * 100)} %`;
  const px = (v: number) => BOX.x0 + ((BOX.x1 - BOX.x0) * v) / max;
  const py = (p: number) => BOX.y1 - (BOX.y1 - BOX.y0) * p;
  const pi = inh.length ? exceedance(inh, draft) : 0;
  const pr = res.length ? exceedance(res, draft) : 0;

  return (
    <Panel testId="fin-lec" className="px-5 py-[18px] mb-4">
      <div className="flex justify-between items-start gap-5 flex-wrap">
        <div>
          <h2 className="m-0 text-[13.5px] font-semibold text-ink">{t('financial.lec.title')}</h2>
          <p className="m-0 mt-1 text-[12.5px] text-ink-soft">{t('financial.lec.sub')}</p>
        </div>
        {lec && (
          <label
            className="grid gap-1.5 flex-[0_1_320px] min-w-[260px]"
            title={canSet ? undefined : t('financial.lec.appetiteReadOnly')}
          >
            <span className="flex justify-between gap-3 text-[12.5px] whitespace-nowrap">
              <span className="font-semibold text-ink">{t('financial.lec.appetite')}</span>
              <span className="mono font-semibold text-ink" data-testid="fin-appetite-value">
                {fmtM(draft)} M {cur}
                {stored == null && (
                  <span className="font-normal text-ink-muted">
                    {' '}
                    · {t('financial.lec.appetiteUnset')}
                  </span>
                )}
              </span>
            </span>
            <input
              type="range"
              min={step}
              max={max}
              step={step}
              value={draft}
              disabled={!canSet}
              onChange={(e) => onSlide(Number(e.target.value))}
              aria-label={t('financial.lec.appetite')}
              aria-valuetext={`${fmtM(draft)} M ${cur}`}
              className="w-full accent-(--accent) disabled:opacity-60"
              data-testid="fin-appetite"
            />
          </label>
        )}
      </div>

      {!lec ? (
        <BlockEmpty>{t('financial.lec.empty')}</BlockEmpty>
      ) : (
        <>
          <svg
            viewBox="0 0 900 284"
            className="w-full h-auto block mt-3"
            role="img"
            aria-label={t('financial.lec.aria', { amount: fmtM(draft), currency: cur })}
          >
            {[0, 0.25, 0.5, 0.75, 1].map((p) => (
              <g key={p}>
                <line x1={BOX.x0} x2={BOX.x1} y1={py(p)} y2={py(p)} stroke="var(--chart-grid)" />
                <text
                  x={BOX.x0 - 8}
                  y={py(p) + 4}
                  textAnchor="end"
                  fontSize="11"
                  className="mono"
                  fill="var(--chart-label)"
                >
                  {Math.round(p * 100)}%
                </text>
              </g>
            ))}
            {ticks(max).map((v) => (
              <text
                key={v}
                x={px(v)}
                y="250"
                textAnchor="middle"
                fontSize="11"
                className="mono"
                fill="var(--chart-label)"
              >
                {fmtM(v)} M
              </text>
            ))}
            <text
              x={(BOX.x0 + BOX.x1) / 2}
              y="276"
              textAnchor="middle"
              fontSize="11"
              fill="var(--chart-label)"
            >
              {t('financial.lec.xAxis', { currency: cur })}
            </text>
            <text
              x="12"
              y={(BOX.y0 + BOX.y1) / 2}
              textAnchor="middle"
              fontSize="11"
              fill="var(--chart-label)"
              transform={`rotate(-90 12 ${(BOX.y0 + BOX.y1) / 2})`}
            >
              {t('financial.lec.yAxis')}
            </text>
            <path
              d={curvePath(inh, max, BOX)}
              fill="none"
              stroke={C_INH}
              strokeWidth="2"
              data-testid="fin-lec-inherent"
            />
            <path
              d={curvePath(res, max, BOX)}
              fill="none"
              stroke={C_RES}
              strokeWidth="2"
              data-testid="fin-lec-residual"
            />
            <line
              x1={px(draft)}
              x2={px(draft)}
              y1={BOX.y0}
              y2={BOX.y1}
              stroke="var(--fg-primary)"
              strokeWidth="1"
              strokeDasharray="4 4"
            />
            <circle
              cx={px(draft)}
              cy={py(pi)}
              r="4.5"
              fill={C_INH}
              stroke="var(--surface-1)"
              strokeWidth="2"
            />
            <circle
              cx={px(draft)}
              cy={py(pr)}
              r="4.5"
              fill={C_RES}
              stroke="var(--surface-1)"
              strokeWidth="2"
            />
          </svg>
          <div
            className="flex gap-6 flex-wrap text-[12.5px] mt-1.5 text-ink"
            data-testid="fin-lec-legend"
          >
            <span className="flex items-center gap-2">
              <span className="w-3.5 h-0.5" style={{ background: C_INH }} />
              {t('financial.lec.inherent')} <b className="mono font-semibold">{pct(pi)}</b>{' '}
              {t('financial.lec.exceed', { amount: fmtM(draft) })}
            </span>
            <span className="flex items-center gap-2">
              <span className="w-3.5 h-0.5" style={{ background: C_RES }} />
              {t('financial.lec.residual')} <b className="mono font-semibold">{pct(pr)}</b>
            </span>
          </div>
        </>
      )}
    </Panel>
  );
}
