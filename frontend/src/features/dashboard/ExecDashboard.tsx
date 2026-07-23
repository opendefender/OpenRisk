// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Executive dashboard persona (UX-2). Leads with cost & KPIs, no technical detail:
// the A–F cyber score, financial exposure (ALE, FCFA) and the key risk indicators.
// Real data from the consolidated /analytics/executive endpoint.

import { useNavigate } from 'react-router-dom';
import { Coins, AlertTriangle, ShieldAlert, CheckCircle2 } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useExecutiveDashboard } from '../analytics/useExecutive';
import { DashboardShell, PersonaHeader, ScoreHero, StatCard, Card } from './shared';

const KRI_COL: Record<string, string> = {
  critical: 'var(--critical)', high: 'var(--high)', medium: 'var(--medium)', low: 'var(--low)', info: 'var(--text-muted)',
};

export function ExecDashboard() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data } = useExecutiveDashboard();
  const cyber = data?.cyber_score;
  const fin = data?.financial;
  const kris = data?.kris ?? [];

  const money = (n: number) =>
    new Intl.NumberFormat(lang === 'fr' ? 'fr-FR' : 'en-US', { notation: n >= 1_000_000 ? 'compact' : 'standard', maximumFractionDigits: 1 }).format(n) + ' FCFA';
  const kriUnit = (u: string) => (u === '%' ? '%' : u === 'days' ? tr(' j', ' d') : '');

  return (
    <DashboardShell>
      <PersonaHeader
        title={tr('Direction', 'Executive')}
        subtitle={tr('La posture en un écran : score, exposition financière, indicateurs clés.', 'Your posture at a glance: score, financial exposure, key indicators.')}
        actionLabel={tr('Rapport conseil', 'Board report')}
        onAction={() => navigate('/reports')}
      />

      <div className="grid grid-cols-1 lg:grid-cols-[340px_1fr] gap-4 mb-4">
        <ScoreHero
          title={tr('Cyber score', 'Cyber score')}
          score={Math.round(cyber?.score ?? 0)}
          grade={cyber?.grade}
          ctaLabel={tr('Voir Analytics', 'View analytics')}
          onDetails={() => navigate('/analytics')}
        />
        <div className="grid grid-cols-2 gap-4">
          <StatCard label={tr('Exposition annuelle (ALE)', 'Annual exposure (ALE)')} value={money(fin?.total_ale?.xaf ?? 0)} col="var(--accent)" icon={Coins} onClick={() => navigate('/analytics/financial')} />
          <StatCard label={tr('Pire cas', 'Worst case')} value={money(fin?.total_ale_worst?.xaf ?? 0)} col="var(--critical)" icon={AlertTriangle} onClick={() => navigate('/analytics/financial')} />
          <StatCard label={tr('Risques', 'Risks')} value={String(fin?.total_risks ?? 0)} col="var(--high)" icon={ShieldAlert} onClick={() => navigate('/risks')} />
          <StatCard label={tr('Quantifiés', 'Quantified')} value={String(fin?.quantified_risks ?? 0)} col="var(--low)" icon={CheckCircle2} onClick={() => navigate('/analytics/financial')} />
        </div>
      </div>

      <Card style={{ padding: '18px 20px' }}>
        <div className="flex items-center justify-between mb-4">
          <div className="text-[14px] font-semibold text-ink">{tr('Indicateurs clés de risque (KRI)', 'Key risk indicators (KRI)')}</div>
          <button onClick={() => navigate('/analytics')} className="text-[12px] font-semibold text-accent hover:underline">{tr('Détails', 'Details')}</button>
        </div>
        {kris.length === 0 ? (
          <div className="py-8 text-center text-[13px] text-ink-muted">{tr('Indicateurs indisponibles.', 'Indicators unavailable.')}</div>
        ) : (
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
            {kris.map((k) => (
              <div key={k.key} className="rounded-xl p-3.5" style={{ background: 'var(--bg-hover)' }}>
                <div className="mono text-[22px] font-bold leading-none" style={{ color: KRI_COL[k.severity] ?? 'var(--text-primary)' }}>
                  {new Intl.NumberFormat(lang === 'fr' ? 'fr-FR' : 'en-US', { maximumFractionDigits: 1 }).format(k.value)}
                  {kriUnit(k.unit)}
                </div>
                <div className="text-[11.5px] text-ink-soft mt-1.5">{k.label}</div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </DashboardShell>
  );
}
