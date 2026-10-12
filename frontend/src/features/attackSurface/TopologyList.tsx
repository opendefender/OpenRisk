// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The topology as a table (#907, D-069 D): the same assets and dependencies
// the graph draws, readable by keyboard and screen reader, and the quickest
// way to scan an estate too large to read as a picture.

import { useMemo } from 'react';

import { useI18n } from '../../hooks/useI18n';
import { CritBadge } from '../../shared/ui';
import type { Criticality } from '../../shared/riskColors';
import { neighbourhood } from './topologyModel';
import type { TopologyEdge, TopologyNode } from './topologyTypes';

const RANK: Record<string, number> = { CRITICAL: 4, HIGH: 3, MEDIUM: 2, LOW: 1 };

export function TopologyList({
  nodes,
  edges,
  nameOf,
  onShow,
}: {
  nodes: readonly TopologyNode[];
  edges: readonly TopologyEdge[];
  nameOf: (id: string) => string;
  onShow: (id: string) => void;
}) {
  const { t } = useI18n();
  const rows = useMemo(
    () =>
      [...nodes]
        .sort(
          (a, b) =>
            (RANK[(b.criticality ?? 'LOW') as string] ?? 0) -
              (RANK[(a.criticality ?? 'LOW') as string] ?? 0) ||
            (a.name ?? '').localeCompare(b.name ?? ''),
        )
        .map((n) => ({ n, nb: neighbourhood(edges, n.id as string) })),
    [nodes, edges],
  );
  const th = 'h-9 px-3 text-left font-semibold border-b border-border-subtle';
  const names = (ids: string[]) => (ids.length ? ids.map(nameOf).join(', ') : '—');

  return (
    <div className="overflow-auto max-h-[68vh]" data-testid="topo-list">
      <table className="w-full border-collapse text-[13px] min-w-[720px]">
        <caption className="sr-only">{t('topology.listCaption')}</caption>
        <thead className="sticky top-0 bg-surface-1">
          <tr className="text-ink-muted text-[10.5px] tracking-[.06em] uppercase">
            <th scope="col" className={th}>
              {t('topology.colAsset')}
            </th>
            <th scope="col" className={th}>
              {t('topology.colCrit')}
            </th>
            <th scope="col" className={th}>
              {t('topology.colExposed')}
            </th>
            <th scope="col" className={th}>
              {t('topology.colUp')}
            </th>
            <th scope="col" className={th}>
              {t('topology.colDown')}
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ n, nb }) => (
            <tr key={n.id} className="border-b border-border-subtle align-top">
              <th scope="row" className="px-3 py-2 text-left font-medium">
                <button
                  type="button"
                  onClick={() => onShow(n.id as string)}
                  className="text-ink hover:underline text-left"
                >
                  {n.name}
                </button>
              </th>
              <td className="px-3 py-2">
                <CritBadge
                  crit={((n.criticality ?? 'LOW') as string).toLowerCase() as Criticality}
                />
              </td>
              <td className="px-3 py-2 text-ink-soft">
                {n.internet_exposed ? t('topology.yes') : t('topology.no')}
              </td>
              <td className="px-3 py-2 text-ink-soft">{names(nb.up)}</td>
              <td className="px-3 py-2 text-ink-soft">{names(nb.down)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
