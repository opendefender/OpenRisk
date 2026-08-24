// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// DSI / asset-owner dashboard persona (UX-2). Leads with the estate: inventory
// size, criticality mix, and the most critical assets.
//
// W0-06 moved the counting to the server. Every number here used to be a
// reduction over the whole GET /assets collection — which returns the entire
// inventory WITH its many-to-many risk associations preloaded, because that is
// what the topology graph needs. The counts were right; obtaining them cost the
// estate crossing the wire on every dashboard paint, and grew with the tenant.
//
// GET /assets/statistics answers the same question in one grouped pass, and
// answers two the client could not: how many assets carry no category
// (everything written before typed attributes shipped) and how many distinct
// type labels exist beyond the twelve shown. The list of critical assets still
// comes from the collection — it needs the rows, not a count.

import { useMemo } from 'react';
import { useNavigate } from 'react-router';
import { Database, ShieldAlert, AlertTriangle, Boxes } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useAssets } from '../assets/useAssets';
import { useAssetStatistics } from './useCommandCenter';
import { useDashboardPeriod, periodLabel } from './period';
import { deepLink } from './deepLinks';
import { PeriodControl } from './PeriodControl';
import { WidgetState } from './WidgetState';
import { DashboardShell, PersonaHeader, KpiRow, Card, type KpiSpec } from './shared';

const CRIT_COL: Record<string, string> = {
  CRITICAL: 'var(--critical)', HIGH: 'var(--high)', MEDIUM: 'var(--medium)', LOW: 'var(--low)',
};
const CRIT_ORDER = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'];

export function EstateDashboard() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { selection, setSelection } = useDashboardPeriod();
  const statsQuery = useAssetStatistics(selection);
  const stats = statsQuery.data;
  const { assets, isLoading: assetsLoading, error: assetsError } = useAssets();

  const norm = (c?: string) => (c ?? 'LOW').toUpperCase();
  const byCrit = stats?.by_criticality ?? {};

  const critical = useMemo(
    () => [...assets]
      .filter((a) => ['CRITICAL', 'HIGH'].includes(norm(a.criticality)))
      .sort((a, b) => CRIT_ORDER.indexOf(norm(a.criticality)) - CRIT_ORDER.indexOf(norm(b.criticality)))
      .slice(0, 6),
    [assets]
  );

  // Each tile carries its own filter into the inventory. "Critical — 7" opens
  // the seven assets it counted, not the whole estate.
  const kpis: KpiSpec[] = [
    {
      label: tr('Actifs', 'Assets'), val: stats?.total ?? 0, icon: Database, col: 'var(--accent)',
      onClick: () => navigate(deepLink('assets')),
    },
    {
      label: tr('Critiques', 'Critical'), val: byCrit.CRITICAL ?? 0, icon: AlertTriangle, col: 'var(--critical)',
      onClick: () => navigate(deepLink('assets', { filters: { criticality: 'critical' } })),
    },
    {
      label: tr('Élevés', 'High'), val: byCrit.HIGH ?? 0, icon: ShieldAlert, col: 'var(--high)',
      onClick: () => navigate(deepLink('assets', { filters: { criticality: 'high' } })),
    },
    {
      label: tr('Types', 'Types'), val: stats?.distinct_types ?? 0, icon: Boxes, col: 'var(--low)',
      onClick: () => navigate(deepLink('assets')),
    },
  ];

  const maxCrit = Math.max(1, ...CRIT_ORDER.map((c) => byCrit[c] ?? 0));

  return (
    <DashboardShell>
      <div className="flex items-start justify-between flex-wrap gap-3.5 mb-[22px]">
        <div className="flex-1 min-w-[240px]">
          <PersonaHeader
            title={tr('Patrimoine', 'Estate')}
            subtitle={tr('Ce que vous protégez : inventaire, criticité et dépendances.', 'What you protect: inventory, criticality and dependencies.')}
            actionLabel={tr('Vue Univers', 'Universe view')}
            onAction={() => navigate('/assets/topology')}
          />
        </div>
        <PeriodControl
          selection={selection}
          onChange={setSelection}
          lang={lang}
          // Only one number on this screen moves with the period, and the note
          // says which. The inventory counters are a stock: how many critical
          // assets exist is not a question a date range changes.
          scopeNote={tr(
            'Filtre uniquement « ajoutés sur la période ». L’inventaire et la répartition par criticité donnent l’état actuel du parc.',
            'Filters "added in period" only. The inventory counters and the criticality mix show the estate as it stands now.'
          )}
        />
      </div>

      <div className="mb-4">
        <WidgetState
          lang={lang}
          isLoading={statsQuery.isLoading}
          error={statsQuery.error}
          skeletonHeight={140}
          retry={() => void statsQuery.refetch()}
        >
          <>
            <KpiRow items={kpis} />
            <div className="mt-2 text-[11.5px] text-ink-muted flex flex-wrap gap-x-4 gap-y-1">
              <span>
                {tr(
                  `${stats?.added_in_period ?? 0} ajouté(s) sur la période (${periodLabel(selection, lang)})`,
                  `${stats?.added_in_period ?? 0} added in the selected period (${periodLabel(selection, lang)})`
                )}
              </span>
              {/* Stated rather than dropped: these rows are part of the total
                  above, and a breakdown that hid them would not add up to it. */}
              {!!stats?.uncategorised && (
                <span>
                  {tr(
                    `${stats.uncategorised} sans catégorie`,
                    `${stats.uncategorised} with no category`
                  )}
                </span>
              )}
              {!!stats?.types_truncated && (
                <span>
                  {tr(
                    `+${stats.types_truncated} autre(s) type(s) non affiché(s)`,
                    `+${stats.types_truncated} more type(s) not shown`
                  )}
                </span>
              )}
            </div>
          </>
        </WidgetState>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[1.6fr_1fr] gap-4">
        <Card style={{ padding: '18px 14px' }}>
          <div className="flex items-center justify-between mb-2 px-2">
            <div className="text-[14px] font-semibold text-ink">{tr('Actifs critiques', 'Critical assets')}</div>
            <button
              onClick={() => navigate(deepLink('assets', { filters: { criticality: ['critical', 'high'] } }))}
              className="text-[12px] font-semibold text-accent hover:underline"
            >
              {tr('Inventaire', 'Inventory')}
            </button>
          </div>
          <WidgetState
            lang={lang}
            isLoading={assetsLoading}
            error={assetsError}
            skeletonHeight={200}
            isEmpty={critical.length === 0}
            emptyIcon={Database}
            emptyTitle={tr('Aucun actif critique', 'No critical asset')}
            emptyDescription={tr(
              'Les actifs classés critiques ou élevés apparaissent ici, les plus critiques en premier.',
              'Assets rated critical or high appear here, most critical first.'
            )}
            emptyAction={
              <button
                onClick={() => navigate('/assets')}
                className="h-[34px] px-4 rounded-[9px] text-[12.5px] font-semibold text-text-primary"
                style={{ background: 'var(--accent)' }}
              >
                {tr('Nouvel actif', 'New asset')}
              </button>
            }
          >
            <>
              {critical.map((a) => {
                const col = CRIT_COL[norm(a.criticality)] ?? 'var(--text-muted)';
                return (
                  <button key={a.id} onClick={() => navigate(deepLink('assets', { focus: String(a.id) }))} className="w-full flex items-center gap-3 px-2 py-[11px] rounded-[10px] hover:bg-hover transition-colors text-left">
                    <span className="w-[9px] h-[9px] rounded-full shrink-0" style={{ background: col }} />
                    <div className="flex-1 min-w-0">
                      <div className="text-[13px] font-medium text-ink truncate">{a.name}</div>
                      <div className="text-[11.5px] text-ink-muted mt-0.5 truncate">{a.type || '—'}</div>
                    </div>
                    <span className="text-[10px] font-semibold uppercase px-1.5 py-0.5 rounded shrink-0" style={{ color: col, background: `color-mix(in srgb, ${col} 15%, transparent)` }}>
                      {norm(a.criticality)}
                    </span>
                  </button>
                );
              })}
            </>
          </WidgetState>
        </Card>

        <Card style={{ padding: '18px 20px' }}>
          <div className="text-[14px] font-semibold text-ink mb-4">{tr('Par criticité', 'By criticality')}</div>
          <WidgetState
            lang={lang}
            isLoading={statsQuery.isLoading}
            error={statsQuery.error}
            skeletonHeight={160}
            retry={() => void statsQuery.refetch()}
          >
            <div className="space-y-3">
              {CRIT_ORDER.map((c) => {
                const n = byCrit[c] ?? 0;
                const col = CRIT_COL[c];
                return (
                  <button
                    key={c}
                    onClick={() => navigate(deepLink('assets', { filters: { criticality: c.toLowerCase() } }))}
                    className="w-full text-left"
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[12px] font-medium capitalize" style={{ color: col }}>{c.toLowerCase()}</span>
                      <span className="mono text-[12px] font-semibold text-ink-soft">{n}</span>
                    </div>
                    <div className="h-[6px] rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                      <div className="h-full rounded-full" style={{ width: `${(n / maxCrit) * 100}%`, background: col, transition: 'width .7s cubic-bezier(.2,.8,.2,1)' }} />
                    </div>
                  </button>
                );
              })}
            </div>
          </WidgetState>
        </Card>
      </div>
    </DashboardShell>
  );
}
