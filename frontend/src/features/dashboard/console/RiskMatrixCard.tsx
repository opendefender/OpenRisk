// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// 5×5 risk matrix of the redesigned dashboard (#901).
//
// Binning (server side, /stats risk_matrix): probability 0–1 lands in column
// CEIL(p × 5) and impact 0–10 in row CEIL(i / 2), both clamped to 1–5, so a
// risk at exactly 0 still sits in the first cell rather than vanishing.
//
// The cell tint is a legend for the GRID, read off the cell's coordinates as
// the design does (P × I on the 1–25 grid: ≥ 16, ≥ 10, ≥ 5). It never labels a
// risk: a risk's own band comes from the Score Engine and is shown in the
// register.

import { Link } from 'react-router';
import { useI18n } from '../../../hooks/useI18n';
import { useDashboardStats } from '../useCommandCenter';
import { BlockEmpty, BlockError, Panel, PanelTitle, HeaderLink } from './Panel';

const ALL_TIME = { kind: 'preset', preset: 'all' } as const;
const FIVE = [1, 2, 3, 4, 5];

function gridBand(cell: number): { key: string; color: string } {
  if (cell >= 16) return { key: 'critical', color: 'var(--risk-critical)' };
  if (cell >= 10) return { key: 'high', color: 'var(--risk-high)' };
  if (cell >= 5) return { key: 'medium', color: 'var(--risk-moderate)' };
  return { key: 'low', color: 'var(--risk-low)' };
}

const tint = (c: string, pct: number) => `color-mix(in srgb, ${c} ${pct}%, transparent)`;

export function RiskMatrixCard({ className = '' }: { className?: string }) {
  const { t } = useI18n();
  const stats = useDashboardStats(ALL_TIME);
  const counts = new Map<string, number>();
  for (const c of stats.data?.risk_matrix ?? []) counts.set(`${c.probability}-${c.impact}`, c.count);
  const total = (stats.data?.risk_matrix ?? []).reduce((n, c) => n + c.count, 0);

  return (
    <Panel testId="dash-matrix" className={`px-5 py-[18px] ${className}`}>
      <PanelTitle
        className="mb-3"
        title={t('dashboard.matrix.title')}
        aside={<HeaderLink to="/risks?view=matrix">{t('dashboard.matrix.open')}</HeaderLink>}
      />
      {stats.isLoading ? (
        <div className="or-skeleton rounded-[10px] h-[230px]" aria-busy="true" />
      ) : stats.isError ? (
        <BlockError onRetry={() => void stats.refetch()} />
      ) : (
        <>
          <div className="grid grid-cols-[18px_repeat(5,minmax(0,1fr))] gap-1 items-stretch">
            {[5, 4, 3, 2, 1].map((p) => (
              <Row key={p} p={p} counts={counts} />
            ))}
            <span />
            {FIVE.map((n) => (
              <span key={n} className="mono text-[10.5px] text-ink-muted text-center pt-0.5">
                {n}
              </span>
            ))}
          </div>
          <div className="flex justify-between text-[10.5px] tracking-[0.06em] uppercase font-semibold text-ink-muted mt-1.5">
            <span>{t('dashboard.matrix.probability')}</span>
            <span>{t('dashboard.matrix.impact')}</span>
          </div>
          {total === 0 && <BlockEmpty>{t('dashboard.matrix.empty')}</BlockEmpty>}
        </>
      )}
    </Panel>
  );
}

function Row({ p, counts }: { p: number; counts: Map<string, number> }) {
  const { t } = useI18n();
  return (
    <>
      <span className="mono text-[10.5px] text-ink-muted flex items-center justify-center">{p}</span>
      {FIVE.map((i) => {
        const n = counts.get(`${p}-${i}`) ?? 0;
        const band = gridBand(p * i);
        const title = t('dashboard.matrix.cell', {
          p,
          i,
          band: t(`dashboard.band.${band.key}`),
          count: n,
        });
        return (
          <Link
            key={i}
            to="/risks?view=matrix"
            title={title}
            aria-label={title}
            className="h-[38px] rounded-[7px] flex items-center justify-center mono text-[13px] font-semibold text-ink hover:brightness-[1.15]"
            style={{
              background: tint(band.color, n ? 22 : 8),
              border: `1px solid ${tint(band.color, n ? 40 : 16)}`,
            }}
          >
            {n || ''}
          </Link>
        );
      })}
    </>
  );
}
