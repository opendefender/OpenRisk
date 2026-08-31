// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Security-analyst dashboard persona (UX-2). Leads with vulnerabilities: the
// prioritised KPI band (open / P1 / KEV / exploitable) and the top-priority queue,
// each row deep-linking into the vulnerability drawer. Real data from the same
// endpoints the Vulnerabilities screen uses.
//
// W0-06: the KPI band used to render `stats?.total ?? 0` and friends, so a failed
// /vulnerabilities/stats produced a tidy row of zeroes — an analyst's dashboard
// reporting no open vulnerabilities and no KEV, which is the most reassuring
// thing this screen can say and the least verifiable. It now renders an error
// state. Each tile also carries its own filter into the register: clicking
// "P1 — 4" opens those four, not the whole list.

import { useNavigate } from 'react-router';
import { Bug, ShieldAlert, Flame, Zap } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useVulnerabilities, useVulnStats } from '../vulnerabilities/useVulnerabilities';
import { deepLink } from './deepLinks';
import { WidgetState } from './WidgetState';
import { DashboardShell, PersonaHeader, KpiRow, Card, type KpiSpec } from './shared';

const SEV_COLOR: Record<string, string> = {
  critical: 'var(--critical)', high: 'var(--high)', medium: 'var(--medium)', low: 'var(--low)', info: 'var(--fg-muted)',
};
const TIER_COLOR: Record<string, string> = { P1: 'var(--critical)', P2: 'var(--high)', P3: 'var(--medium)', P4: 'var(--low)' };
const soft = (c: string, p = 15) => `color-mix(in srgb, ${c} ${p}%, transparent)`;

export function AnalystDashboard() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const statsQuery = useVulnStats();
  const stats = statsQuery.data;
  const listQuery = useVulnerabilities({ limit: 6, sort_by: 'priority_score', sort_dir: 'desc' });
  const { isLoading } = listQuery;
  const top = listQuery.data?.items ?? [];

  // The queue is sorted by priority, so every link carries that sort: the rows
  // the tile counted are the rows at the top of the page it opens.
  const sort = { key: 'priority_score', dir: 'desc' } as const;
  const kpis: KpiSpec[] = [
    {
      label: tr('Vulnérabilités', 'Vulnerabilities'), val: stats?.total ?? 0, icon: Bug, col: 'var(--accent)',
      onClick: () => navigate(deepLink('vulnerabilities', { sort })),
    },
    {
      label: tr('Ouvertes', 'Open'), val: stats?.open ?? 0, icon: ShieldAlert, col: 'var(--high)',
      onClick: () => navigate(deepLink('vulnerabilities', { filters: { status: 'open' }, sort })),
    },
    {
      label: tr('Priorité P1', 'Priority P1'), val: stats?.by_tier?.P1 ?? 0, icon: Flame, col: 'var(--critical)',
      onClick: () => navigate(deepLink('vulnerabilities', { filters: { tier: 'P1' }, sort })),
    },
    {
      label: 'KEV', val: stats?.kev_count ?? 0, icon: Zap, col: 'var(--critical)',
      onClick: () => navigate(deepLink('vulnerabilities', { filters: { kev: 'true' }, sort })),
    },
  ];

  const sevCounts = stats?.by_severity ?? {};
  const sevOrder = ['critical', 'high', 'medium', 'low', 'info'];
  const sevMax = Math.max(1, ...sevOrder.map((s) => sevCounts[s] ?? 0));

  return (
    <DashboardShell>
      <PersonaHeader
        title={tr('Vulnérabilités', 'Vulnerabilities')}
        subtitle={tr('Priorisez ce qui compte : P1, KEV et exploitables d’abord.', 'Fix what matters first: P1, KEV and exploitable.')}
        actionLabel={tr('Voir tout', 'View all')}
        onAction={() => navigate('/vulnerabilities')}
      />

      <div className="mb-4">
        <WidgetState
          lang={lang}
          isLoading={statsQuery.isLoading}
          error={statsQuery.error}
          skeletonHeight={140}
          retry={() => void statsQuery.refetch()}
        >
          <KpiRow items={kpis} />
        </WidgetState>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[1.6fr_1fr] gap-4">
        {/* top-priority queue */}
        <Card style={{ padding: '18px 14px' }}>
          <div className="flex items-center justify-between mb-2 px-2">
            <div className="text-[14px] font-semibold text-ink">{tr('File de priorité', 'Priority queue')}</div>
            <button onClick={() => navigate('/vulnerabilities')} className="text-[12px] font-semibold text-accent hover:underline">
              {tr('Tout voir', 'See all')}
            </button>
          </div>
          <WidgetState
            lang={lang}
            isLoading={isLoading}
            error={listQuery.error}
            skeletonHeight={220}
            retry={() => void listQuery.refetch()}
            isEmpty={top.length === 0}
            emptyIcon={Bug}
            emptyTitle={tr('Aucune vulnérabilité', 'No vulnerabilities')}
            emptyDescription={tr(
              'Importez un scan ou branchez un connecteur : les vulnérabilités priorisées apparaîtront ici, P1 et KEV en tête.',
              'Import a scan or connect a source: prioritised vulnerabilities appear here, P1 and KEV first.'
            )}
            emptyAction={
              <button onClick={() => navigate('/vulnerabilities')} className="h-[34px] px-4 rounded-[9px] text-[12.5px] font-semibold text-fg-primary" style={{ background: 'var(--accent)' }}>
                {tr('Importer des vulnérabilités', 'Import vulnerabilities')}
              </button>
            }
          >
            <>
            {top.map((v) => {
              const tierCol = TIER_COLOR[v.priority_tier] ?? 'var(--fg-muted)';
              return (
                <button
                  key={v.id}
                  onClick={() => navigate(deepLink('vulnerabilities', { focus: String(v.id) }))}
                  className="w-full flex items-center gap-3 px-2 py-[11px] rounded-[10px] hover:bg-hover transition-colors text-left"
                >
                  <span className="text-[10px] font-bold px-1.5 py-0.5 rounded shrink-0" style={{ color: tierCol, background: soft(tierCol) }}>
                    {v.priority_tier}
                  </span>
                  <div className="flex-1 min-w-0">
                    <div className="text-[13px] font-medium text-ink truncate flex items-center gap-1.5">
                      {v.kev && <Flame size={12} style={{ color: 'var(--critical)' }} />}
                      {v.title}
                    </div>
                    <div className="text-[11.5px] text-ink-muted mt-0.5 truncate">{v.cve_id || v.asset_name || '—'}</div>
                  </div>
                  <span className="text-[10px] font-semibold uppercase px-1.5 py-0.5 rounded shrink-0" style={{ color: SEV_COLOR[v.severity] ?? 'var(--fg-muted)', background: soft(SEV_COLOR[v.severity] ?? 'var(--fg-muted)') }}>
                    {v.severity}
                  </span>
                  <span className="mono text-[13px] font-bold w-[38px] text-right" style={{ color: tierCol }}>
                    {v.priority_score.toFixed(0)}
                  </span>
                </button>
              );
            })}
            </>
          </WidgetState>
        </Card>

        {/* severity distribution */}
        <Card style={{ padding: '18px 20px' }}>
          <div className="text-[14px] font-semibold text-ink mb-4">{tr('Par sévérité', 'By severity')}</div>
          <WidgetState
            lang={lang}
            isLoading={statsQuery.isLoading}
            error={statsQuery.error}
            skeletonHeight={200}
            retry={() => void statsQuery.refetch()}
          >
          <>
          <div className="space-y-3">
            {sevOrder.map((s) => {
              const c = sevCounts[s] ?? 0;
              const col = SEV_COLOR[s];
              return (
                <button
                  key={s}
                  onClick={() => navigate(deepLink('vulnerabilities', { filters: { severity: s }, sort }))}
                  className="w-full text-left"
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-[12px] font-medium capitalize" style={{ color: col }}>{s}</span>
                    <span className="mono text-[12px] font-semibold text-ink-soft">{c}</span>
                  </div>
                  <div className="h-[6px] rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                    <div className="h-full rounded-full" style={{ width: `${(c / sevMax) * 100}%`, background: col, transition: 'width .7s cubic-bezier(.2,.8,.2,1)' }} />
                  </div>
                </button>
              );
            })}
          </div>
          <div className="mt-4 pt-3 border-t border-border flex items-center justify-between text-[12px]">
            <span className="text-ink-soft">{tr('Exploitables', 'Exploitable')}</span>
            <span className="mono font-semibold text-ink">{stats?.exploit_count ?? 0}</span>
          </div>
          </>
          </WidgetState>
        </Card>
      </div>
    </DashboardShell>
  );
}
