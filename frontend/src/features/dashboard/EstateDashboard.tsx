// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// DSI / asset-owner dashboard persona (UX-2). Leads with the estate: inventory
// size, criticality mix, and the most critical assets (each deep-linking into the
// asset editor). Real data from the assets endpoint.

import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Database, ShieldAlert, AlertTriangle, Boxes } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useAssets } from '../assets/useAssets';
import { DashboardShell, PersonaHeader, KpiRow, Card, type KpiSpec } from './shared';

const CRIT_COL: Record<string, string> = {
  CRITICAL: 'var(--critical)', HIGH: 'var(--high)', MEDIUM: 'var(--medium)', LOW: 'var(--low)',
};
const CRIT_ORDER = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'];

export function EstateDashboard() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { assets, isLoading } = useAssets();

  const norm = (c?: string) => (c ?? 'LOW').toUpperCase();
  const byCrit = useMemo(() => {
    const m: Record<string, number> = { CRITICAL: 0, HIGH: 0, MEDIUM: 0, LOW: 0 };
    assets.forEach((a) => { m[norm(a.criticality)] = (m[norm(a.criticality)] ?? 0) + 1; });
    return m;
  }, [assets]);
  const typeCount = useMemo(() => new Set(assets.map((a) => a.type).filter(Boolean)).size, [assets]);

  const critical = useMemo(
    () => [...assets]
      .filter((a) => ['CRITICAL', 'HIGH'].includes(norm(a.criticality)))
      .sort((a, b) => CRIT_ORDER.indexOf(norm(a.criticality)) - CRIT_ORDER.indexOf(norm(b.criticality)))
      .slice(0, 6),
    [assets]
  );

  const kpis: KpiSpec[] = [
    { label: tr('Actifs', 'Assets'), val: assets.length, icon: Database, col: 'var(--accent)', onClick: () => navigate('/assets') },
    { label: tr('Critiques', 'Critical'), val: byCrit.CRITICAL, icon: AlertTriangle, col: 'var(--critical)', onClick: () => navigate('/assets') },
    { label: tr('Élevés', 'High'), val: byCrit.HIGH, icon: ShieldAlert, col: 'var(--high)', onClick: () => navigate('/assets') },
    { label: tr('Types', 'Types'), val: typeCount, icon: Boxes, col: 'var(--low)', onClick: () => navigate('/assets') },
  ];

  const maxCrit = Math.max(1, ...CRIT_ORDER.map((c) => byCrit[c] ?? 0));

  return (
    <DashboardShell>
      <PersonaHeader
        title={tr('Patrimoine', 'Estate')}
        subtitle={tr('Ce que vous protégez : inventaire, criticité et dépendances.', 'What you protect: inventory, criticality and dependencies.')}
        actionLabel={tr('Vue Univers', 'Universe view')}
        onAction={() => navigate('/assets/universe')}
      />

      <div className="mb-4">
        <KpiRow items={kpis} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[1.6fr_1fr] gap-4">
        <Card style={{ padding: '18px 14px' }}>
          <div className="flex items-center justify-between mb-2 px-2">
            <div className="text-[14px] font-semibold text-ink">{tr('Actifs critiques', 'Critical assets')}</div>
            <button onClick={() => navigate('/assets')} className="text-[12px] font-semibold text-accent hover:underline">{tr('Inventaire', 'Inventory')}</button>
          </div>
          {isLoading ? (
            <div className="space-y-2 px-2 py-2">{[0, 1, 2, 3].map((i) => <div key={i} className="h-11 rounded-lg animate-pulse" style={{ background: 'var(--bg-hover)' }} />)}</div>
          ) : critical.length === 0 ? (
            <div className="px-2 py-10 text-center">
              <div className="text-[13px] text-ink-soft mb-3">{tr('Aucun actif critique — ajoutez votre inventaire.', 'No critical asset — add your inventory.')}</div>
              <button onClick={() => navigate('/assets')} className="h-[34px] px-4 rounded-[9px] text-[12.5px] font-semibold text-white" style={{ background: 'var(--accent)' }}>
                {tr('Nouvel actif', 'New asset')}
              </button>
            </div>
          ) : (
            critical.map((a) => {
              const col = CRIT_COL[norm(a.criticality)] ?? 'var(--text-muted)';
              return (
                <button key={a.id} onClick={() => navigate(`/assets?focus=${a.id}`)} className="w-full flex items-center gap-3 px-2 py-[11px] rounded-[10px] hover:bg-hover transition-colors text-left">
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
            })
          )}
        </Card>

        <Card style={{ padding: '18px 20px' }}>
          <div className="text-[14px] font-semibold text-ink mb-4">{tr('Par criticité', 'By criticality')}</div>
          <div className="space-y-3">
            {CRIT_ORDER.map((c) => {
              const n = byCrit[c] ?? 0;
              const col = CRIT_COL[c];
              return (
                <div key={c}>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-[12px] font-medium capitalize" style={{ color: col }}>{c.toLowerCase()}</span>
                    <span className="mono text-[12px] font-semibold text-ink-soft">{n}</span>
                  </div>
                  <div className="h-[6px] rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                    <div className="h-full rounded-full" style={{ width: `${(n / maxCrit) * 100}%`, background: col, transition: 'width .7s cubic-bezier(.2,.8,.2,1)' }} />
                  </div>
                </div>
              );
            })}
          </div>
        </Card>
      </div>
    </DashboardShell>
  );
}
