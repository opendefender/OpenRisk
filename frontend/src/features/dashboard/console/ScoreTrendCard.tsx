// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The exposure score month by month (#901), drawn as the design's area line.
// The points are the closing snapshot of each month (GET /score/history). A
// month with no snapshot is skipped rather than drawn at zero, and until two
// months exist the card says the history is building and from when.

import { useMemo } from 'react';
import { useI18n } from '../../../hooks/useI18n';
import { formatDate, formatNumber } from '../../../i18n/format';
import { useScoreHistory, type ScoreHistoryPoint } from '../useScoreHistory';
import { BlockEmpty, BlockError, Panel, PanelTitle } from './Panel';

const W = 620;
const H = 190;
const LEFT = 40;
const RIGHT = 600;
const TOP = 20;
const BOTTOM = 160;

export function ScoreTrendCard({ months = 12, className = '' }: { months?: number; className?: string }) {
  const { t, locale } = useI18n();
  const history = useScoreHistory(months);
  const points = useMemo(() => history.data?.points ?? [], [history.data]);
  const fmt = (v: number) => formatNumber(locale, v, { maximumFractionDigits: 0 });
  const monthLabel = (p: ScoreHistoryPoint) =>
    formatDate(locale, `${p.month}-01T12:00:00Z`, { month: 'short' });

  const chart = useMemo(() => {
    if (points.length < 2) return null;
    const values = points.map((p) => p.value);
    // Ten-point gridlines around the data, at least 30 points tall, so a flat
    // line is not drawn as a cliff.
    let lo = Math.floor((Math.min(...values) - 2) / 10) * 10;
    let hi = Math.ceil((Math.max(...values) + 2) / 10) * 10;
    if (hi - lo < 30) hi = lo + 30;
    lo = Math.max(0, lo);
    hi = Math.min(100, Math.max(hi, lo + 10));
    const x = (i: number) => LEFT + (i * (RIGHT - LEFT)) / (points.length - 1);
    const y = (v: number) => BOTTOM - ((v - lo) / (hi - lo)) * (BOTTOM - TOP);
    const pts = points.map((p, i) => ({ x: x(i), y: y(p.value), p }));
    const line = pts.map((q, i) => `${i ? 'L' : 'M'}${q.x.toFixed(1)} ${q.y.toFixed(1)}`).join(' ');
    const grid: number[] = [];
    for (let g = lo; g <= hi; g += 10) grid.push(g);
    return {
      pts,
      line,
      area: `${line} L${pts[pts.length - 1].x.toFixed(1)} ${BOTTOM} L${pts[0].x.toFixed(1)} ${BOTTOM} Z`,
      grid: grid.map((g) => ({ v: g, y: y(g) })),
    };
  }, [points]);

  const first = points[0];
  const last = points[points.length - 1];

  return (
    <Panel testId="dash-trend" className={`px-5 py-[18px] ${className}`}>
      <PanelTitle
        className="mb-2"
        title={t('dashboard.trend.title', { count: months })}
        aside={
          chart && first && last ? (
            <span className="text-[12px] text-ink-muted">
              {t('dashboard.trend.range', { from: fmt(first.value), to: fmt(last.value) })}
            </span>
          ) : undefined
        }
      />
      {history.isLoading ? (
        <div className="or-skeleton rounded-[10px] h-[170px]" aria-busy="true" />
      ) : history.isError ? (
        <BlockError onRetry={() => void history.refetch()} />
      ) : !chart ? (
        history.data?.since ? (
          <BlockEmpty>
            {t('dashboard.trend.building', {
              date: formatDate(locale, history.data.since, { dateStyle: 'long' }),
            })}
          </BlockEmpty>
        ) : (
          <BlockEmpty>{t('dashboard.score.unmeasuredBody')}</BlockEmpty>
        )
      ) : (
        <svg
          viewBox={`0 0 ${W} ${H}`}
          className="w-full h-auto block"
          role="img"
          aria-label={t('dashboard.trend.aria', {
            from: fmt(first.value),
            fromMonth: monthLabel(first),
            to: fmt(last.value),
            toMonth: monthLabel(last),
          })}
        >
          {chart.grid.map((g) => (
            <g key={g.v}>
              <line x1={34} x2={610} y1={g.y} y2={g.y} stroke="var(--chart-grid)" strokeWidth={1} />
              <text
                x={26}
                y={g.y + 3.5}
                textAnchor="end"
                fontSize={10.5}
                fontFamily="var(--font-mono)"
                fill="var(--chart-label)"
              >
                {g.v}
              </text>
            </g>
          ))}
          <path d={chart.area} fill="var(--accent-soft)" />
          <path
            d={chart.line}
            fill="none"
            stroke="var(--chart-1)"
            strokeWidth={2}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
          {chart.pts.map((q) => (
            <text
              key={q.p.month}
              x={q.x}
              y={184}
              textAnchor="middle"
              fontSize={10.5}
              fontFamily="var(--font-sans)"
              fill="var(--chart-label)"
            >
              {monthLabel(q.p)}
            </text>
          ))}
          <circle
            cx={chart.pts[chart.pts.length - 1].x}
            cy={chart.pts[chart.pts.length - 1].y}
            r={4}
            fill="var(--chart-1)"
            stroke="var(--surface-1)"
            strokeWidth={2}
          />
        </svg>
      )}
    </Panel>
  );
}
