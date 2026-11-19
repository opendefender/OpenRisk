// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Matrix mode of the register (#913): the 5×5 grid on the left, and on the
// right either the ranking by score or the risks of the cell the user picked.
//
// Our P (0–1) and I (0–10) are binned with the same `toBucket` rule the pickers
// use. A CELL is tinted from its own coordinates (cellCrit, not a score band);
// a MARKER takes the colour of the risk's real, server-assigned band and prints
// its score in the one display format. It is a real grid: every cell is
// announced with its coordinates, band and count.

import { useMemo, useState } from 'react';
import { useI18n } from '../../hooks/useI18n';
import { chart, type MatrixBucket } from '../../shared/ds';
import { critColor, softFill } from '../../shared/riskColors';
import { formatScore } from './riskFormat';
import { LEVELS, cellCrit, impactLevel, probabilityLevel } from './riskLevels';
import type { UiRisk } from './riskMap';

const ROWS: readonly MatrixBucket[] = [5, 4, 3, 2, 1];
/** Markers drawn in a cell before it collapses into "+n". */
const PER_CELL = 4;

type Cell = readonly [MatrixBucket, MatrixBucket];

export function RiskMatrixPanel({
  risks,
  onOpen,
}: {
  risks: readonly UiRisk[];
  onOpen: (id: string) => void;
}) {
  const { t, locale } = useI18n();
  const [cell, setCell] = useState<Cell | null>(null);

  // A closed risk has left the exposure the matrix draws.
  const open = useMemo(() => risks.filter((r) => r.statusValue !== 'closed'), [risks]);

  const grid = useMemo(() => {
    const m = new Map<string, UiRisk[]>();
    for (const r of open) {
      const key = `${probabilityLevel(r.prob)}:${impactLevel(r.impact)}`;
      const list = m.get(key);
      if (list) list.push(r);
      else m.set(key, [r]);
    }
    return m;
  }, [open]);

  const listed = useMemo(() => {
    if (cell) return grid.get(`${cell[0]}:${cell[1]}`) ?? [];
    return [...open].sort((a, b) => b.score - a.score);
  }, [cell, grid, open]);

  const pName = (k: MatrixBucket) => t(`risks.level.probability.${k}`);
  const iName = (k: MatrixBucket) => t(`risks.level.impact.${k}`);

  return (
    <div className="flex gap-4 flex-wrap items-start" data-testid="risk-matrix">
      <section
        aria-label={t('riskRegister.matrix.caption')}
        className="flex-[1.6_1_560px] min-w-0 bg-surface-1 border border-border-subtle rounded-[14px] p-[18px] pb-3.5 overflow-x-auto"
      >
        <table className="w-full table-fixed border-separate border-spacing-1.5 min-w-[520px]">
          <caption className="sr-only">{t('riskRegister.matrix.caption')}</caption>
          <thead>
            <tr>
              <th scope="col" className="w-7">
                <span className="sr-only">{t('riskRegister.matrix.probability')}</span>
              </th>
              {LEVELS.map((i) => (
                <th
                  key={i}
                  scope="col"
                  title={iName(i)}
                  className="mono text-center text-[11px] font-medium text-ink-muted"
                >
                  {i}
                  <span className="sr-only"> — {iName(i)}</span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {ROWS.map((p) => (
              <tr key={p}>
                <th
                  scope="row"
                  title={pName(p)}
                  className="mono w-7 text-center text-[11px] font-medium text-ink-muted"
                >
                  {p}
                  <span className="sr-only"> — {pName(p)}</span>
                </th>
                {LEVELS.map((i) => {
                  const tone = chart.severity[cellCrit(p, i)];
                  const items = grid.get(`${p}:${i}`) ?? [];
                  const on = cell?.[0] === p && cell?.[1] === i;
                  const shown = items.slice(0, PER_CELL);
                  const more = items.length - shown.length;
                  return (
                    <td
                      key={i}
                      className="h-[92px] align-top rounded-[10px] p-1.5"
                      style={{
                        background: softFill(tone, items.length ? 16 : 7),
                        border: `1px solid ${softFill(tone, items.length ? 34 : 14)}`,
                        boxShadow: on ? '0 0 0 2px var(--accent)' : undefined,
                      }}
                    >
                      <button
                        type="button"
                        aria-pressed={on}
                        onClick={() => setCell(on ? null : [p, i])}
                        aria-label={t('riskRegister.matrix.cellAria', {
                          p,
                          pName: pName(p),
                          i,
                          iName: iName(i),
                          count: items.length,
                        })}
                        data-testid={`risk-cell-${p}-${i}`}
                        className="mono block w-full text-left text-[10.5px] text-ink-muted rounded-md hover:text-ink"
                      >
                        {items.length > 0 ? items.length : '·'}
                      </button>
                      <div className="mt-1 flex flex-wrap gap-1">
                        {shown.map((r) => (
                          <button
                            key={r.id}
                            type="button"
                            onClick={() => onOpen(r.id)}
                            title={r.name}
                            aria-label={t('riskRegister.matrix.markerAria', {
                              title: r.name,
                              score: formatScore(locale, r.score),
                              band: t(`risks.band.${r.crit}`),
                            })}
                            data-testid={`risk-marker-${r.id}`}
                            className="mono h-[22px] px-1.5 rounded-full text-[10.5px] font-semibold text-fg-on-solid motion-safe:hover:scale-105 transition-transform"
                            style={{ background: chart.severity[r.crit] }}
                          >
                            {formatScore(locale, r.score)}
                          </button>
                        ))}
                        {more > 0 && (
                          <span className="self-center text-[10.5px] font-semibold text-ink-soft">
                            +{more}
                          </span>
                        )}
                      </div>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
        <div className="flex justify-between mt-2 pl-[34px] text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted">
          <span>↑ {t('riskRegister.matrix.probability')}</span>
          <span>{t('riskRegister.matrix.impact')} →</span>
        </div>
      </section>

      <section
        aria-live="polite"
        className="flex-[1_1_320px] min-w-0 bg-surface-1 border border-border-subtle rounded-[14px] overflow-hidden"
      >
        <div className="px-[18px] pt-4 pb-2.5">
          <h2 className="m-0 text-[13.5px] font-semibold text-ink">
            {cell
              ? t('riskRegister.matrix.cellTitle', { p: cell[0], i: cell[1] })
              : t('riskRegister.matrix.ranking')}
          </h2>
          <p className="m-0 mt-0.5 text-[12px] text-ink-muted">
            {cell
              ? t('riskRegister.matrix.cellSub', {
                  pName: pName(cell[0]),
                  iName: iName(cell[1]),
                  count: listed.length,
                })
              : t('riskRegister.matrix.hint')}
          </p>
        </div>
        {listed.length === 0 ? (
          <p className="m-0 px-[18px] py-6 text-[13px] text-ink-muted border-t border-border-subtle">
            {t('riskRegister.matrix.cellEmpty')}
          </p>
        ) : (
          <ul className="m-0 p-0 list-none max-h-[480px] overflow-y-auto">
            {listed.map((r) => (
              <li key={r.id} className="border-t border-border-subtle">
                <button
                  type="button"
                  onClick={() => onOpen(r.id)}
                  className="w-full flex items-center gap-2.5 px-[18px] py-2.5 text-left hover:bg-surface-2"
                >
                  <span
                    aria-hidden="true"
                    className="size-2 rounded-full shrink-0"
                    style={{ background: critColor[r.crit] }}
                  />
                  <span className="sr-only">{t(`risks.band.${r.crit}`)}</span>
                  <span className="mono text-[11.5px] text-ink-muted shrink-0">
                    {r.id.slice(0, 8)}
                  </span>
                  <span className="flex-1 min-w-0 truncate text-[13px] text-ink">{r.name}</span>
                  <span className="mono text-[12px] font-semibold text-ink">
                    {formatScore(locale, r.score)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
