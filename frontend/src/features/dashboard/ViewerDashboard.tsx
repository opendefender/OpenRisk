// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Viewer dashboard persona (UX-2). A light, read-only overview of the security
// posture — score, a few headline risk counts, and the most recent risks. No
// authoring actions, since a viewer can only read.

import { useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldAlert, AlertTriangle, ShieldCheck } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useRiskStore } from '../../hooks/useRiskStore';
import { critColor, scoreColor, scoreToCriticality, type Criticality } from '../../shared/riskColors';
import { useDashboardStats } from './useStats';
import { DashboardShell, PersonaHeader, KpiRow, Card, ScoreHero, type KpiSpec } from './shared';

export function ViewerDashboard() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const risks = useRiskStore((s) => s.risks);
  const total = useRiskStore((s) => s.total);
  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const { stats } = useDashboardStats();

  useEffect(() => {
    fetchRisks?.().catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const critOf = (r: (typeof risks)[number]): Criticality => (r.level?.toLowerCase() as Criticality) || scoreToCriticality(r.score);
  const sev = stats?.risks_by_severity ?? {};

  const kpis: KpiSpec[] = [
    { label: tr('Risques', 'Risks'), val: stats?.total_risks ?? (total || risks.length), icon: ShieldAlert, col: 'var(--accent)', onClick: () => navigate('/risks') },
    { label: tr('Critiques', 'Critical'), val: (sev.CRITICAL ?? sev.critical) ?? risks.filter((r) => critOf(r) === 'critical').length, icon: AlertTriangle, col: 'var(--critical)', onClick: () => navigate('/risks') },
    { label: tr('Mitigés', 'Mitigated'), val: stats?.mitigated_risks ?? risks.filter((r) => /mitigat|resolv|closed|accept/i.test(r.status)).length, icon: ShieldCheck, col: 'var(--low)', onClick: () => navigate('/risks') },
  ];

  const recent = useMemo(() => risks.slice(0, 6), [risks]);

  return (
    <DashboardShell>
      <PersonaHeader
        title={tr('Vue d’ensemble', 'Overview')}
        subtitle={tr('Aperçu en lecture seule de la posture de sécurité.', 'A read-only snapshot of the security posture.')}
      />

      <div className="grid grid-cols-1 lg:grid-cols-[340px_1fr] gap-4 mb-4">
        <ScoreHero
          title={tr('Score de sécurité', 'Security score')}
          score={Math.round(stats?.global_risk_score ?? 0)}
          ctaLabel={tr('Voir les risques', 'View risks')}
          onDetails={() => navigate('/risks')}
        />
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 content-start">
          <div className="sm:col-span-3"><KpiRow items={kpis} /></div>
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
              <button key={r.id} onClick={() => navigate('/risks')} className="w-full flex items-center gap-3 px-2 py-[11px] rounded-[10px] hover:bg-hover transition-colors text-left">
                <span className="w-[9px] h-[9px] rounded-full shrink-0" style={{ background: critColor[crit] }} />
                <div className="flex-1 min-w-0">
                  <div className="text-[13px] font-medium text-ink truncate">{r.title}</div>
                  <div className="text-[11.5px] text-ink-muted mt-0.5 truncate">{r.assets?.[0]?.name ?? '—'}</div>
                </div>
                <span className="mono text-[13px] font-bold w-[34px] text-right" style={{ color: scoreColor(r.score) }}>{r.score.toFixed(1)}</span>
              </button>
            );
          })
        )}
      </Card>
    </DashboardShell>
  );
}
