// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Viewer dashboard persona (UX-2). A light, read-only overview of the security
// posture — score, a few headline risk counts, and the most recent risks. No
// authoring actions, since a viewer can only read.
//
// W0-06 fixed three things here, all of the same kind.
//
// The score was `stats.global_risk_score`, a SECOND formula (100 − avg × 4)
// computed from the register alone, on the same 0–100 scale as the canonical
// model. A viewer and an admin looking at the same tenant saw two different
// security scores. It now reads the same query key as every other score surface
// in the product, so there is one answer in memory and they cannot disagree.
//
// The counters fell back to `risks.filter(...).length` whenever /stats failed —
// counts over ONE PAGE of the register, printed as tenant totals, on the persona
// least equipped to notice. The fallbacks are gone; a failed read renders an
// error state.
//
// The tiles and the rows now carry their filter into the register instead of
// dropping the user at the top of an unfiltered list.

import { useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router';
import { ShieldAlert, AlertTriangle, ShieldCheck } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useRiskStore } from '../../hooks/useRiskStore';
import { useScore } from '../../hooks/useScore';
import { ScoreGauge } from '../../shared/ScoreGauge';
import { critColor, type Criticality } from '../../shared/riskColors';
import { useDashboardStats } from './useCommandCenter';
import { useDashboardPeriod } from './period';
import { deepLink } from './deepLinks';
import { WidgetState } from './WidgetState';
import { DashboardShell, PersonaHeader, KpiRow, Card, type KpiSpec } from './shared';

export function ViewerDashboard() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const risks = useRiskStore((s) => s.risks);
  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const { selection } = useDashboardPeriod();
  const statsQuery = useDashboardStats(selection);
  const stats = statsQuery.data;
  // The canonical score, from the shared query key.
  const { data: tenantScore, isLoading: scoreLoading } = useScore('tenant');

  useEffect(() => {
    fetchRisks?.().catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // The server's criticality, or nothing. No client fallback from the number:
  // that is the mapping that drifted from the value beside it.
  const critOf = (r: (typeof risks)[number]): Criticality =>
    ((r.criticality ?? r.level)?.toLowerCase() as Criticality) || 'low';
  const sev = stats?.risks_by_severity ?? {};
  const sort = { key: 'score', dir: 'desc' } as const;

  const kpis: KpiSpec[] = [
    {
      label: tr('Risques', 'Risks'),
      val: stats?.total_risks ?? 0,
      icon: ShieldAlert,
      col: 'var(--accent)',
      onClick: () => navigate(deepLink('risks', { sort })),
    },
    {
      label: tr('Critiques', 'Critical'),
      val: (sev.CRITICAL ?? sev.critical) ?? 0,
      icon: AlertTriangle,
      col: 'var(--critical)',
      onClick: () => navigate(deepLink('risks', { filters: { criticality: 'critical' }, sort })),
    },
    {
      label: tr('Atténués', 'Mitigated'),
      val: stats?.mitigated_risks ?? 0,
      icon: ShieldCheck,
      col: 'var(--low)',
      onClick: () => navigate(deepLink('risks', { filters: { status: 'mitigated' }, sort })),
    },
  ];

  const recent = useMemo(() => risks.slice(0, 6), [risks]);

  return (
    <DashboardShell>
      <PersonaHeader
        title={tr('Vue d’ensemble', 'Overview')}
        subtitle={tr('Aperçu en lecture seule de la posture de sécurité.', 'A read-only snapshot of the security posture.')}
      />

      <div className="grid grid-cols-1 lg:grid-cols-[340px_1fr] gap-4 mb-4">
        <ScoreGauge
          score={tenantScore}
          loading={scoreLoading}
          title={tr('Score de sécurité', 'Security score')}
          ctaLabel={tr('Voir le détail', 'View details')}
          onDetails={() => navigate('/score')}
        />
        <div className="grid grid-cols-1 gap-4 content-start">
          <WidgetState
            lang={lang}
            isLoading={statsQuery.isLoading}
            error={statsQuery.error}
            skeletonHeight={160}
            retry={() => void statsQuery.refetch()}
          >
            <KpiRow items={kpis} />
          </WidgetState>
        </div>
      </div>

      <Card style={{ padding: '18px 14px' }}>
        <div className="text-[14px] font-semibold text-ink mb-2 px-2">{tr('Risques récents', 'Recent risks')}</div>
        {recent.length === 0 ? (
          <div className="px-2 py-8 text-center text-[13px] text-ink-muted">{tr('Aucun risque à afficher.', 'No risks to show.')}</div>
        ) : (
          recent.map((r) => {
            const crit = critOf(r);
            return (
              <button
                key={r.id}
                // Opens this row's drawer, not the top of the list.
                onClick={() => navigate(deepLink('risks', { focus: r.id }))}
                className="w-full flex items-center gap-3 px-2 py-[11px] rounded-[10px] hover:bg-hover transition-colors text-left"
              >
                <span className="w-[9px] h-[9px] rounded-full shrink-0" style={{ background: critColor[crit] }} />
                <div className="flex-1 min-w-0">
                  <div className="text-[13px] font-medium text-ink truncate">{r.title}</div>
                  <div className="text-[11.5px] text-ink-muted mt-0.5 truncate">{r.assets?.[0]?.name ?? '—'}</div>
                </div>
                <span className="mono text-[13px] font-bold w-[34px] text-right" style={{ color: critColor[crit] }}>{r.score.toFixed(1)}</span>
              </button>
            );
          })
        )}
      </Card>
    </DashboardShell>
  );
}
