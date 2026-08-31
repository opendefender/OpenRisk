// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Auditor / compliance dashboard persona (UX-2). Leads with controls: coverage per
// framework, the gap count, and shortcuts to audits & gap analysis. Real data from
// the same compliance overview the Compliance screen uses.
//
// W0-06: a failed compliance fetch rendered "No framework — import one to start",
// with all four KPIs at zero. That is not a milder version of the truth, it is a
// different statement: it tells an auditor their organisation has imported
// nothing, and points them at a task they have already done. An unreadable
// posture and an empty one need opposite responses, so they now render
// differently.

import { useNavigate } from 'react-router';
import { ClipboardCheck, Layers, AlertTriangle, ListChecks } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useComplianceOverview, frameworkColorFor } from '../compliance/complianceOverview';
import { WidgetState } from './WidgetState';
import { DashboardShell, PersonaHeader, KpiRow, Card, type KpiSpec } from './shared';

export function AuditDashboard() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const query = useComplianceOverview();
  const { data, isLoading } = query;
  const frameworks = data ?? [];

  const totalControls = frameworks.reduce((s, f) => s + f.total, 0);
  const passed = frameworks.reduce((s, f) => s + f.passed, 0);
  const gaps = Math.max(0, totalControls - passed);
  const avg = frameworks.length ? Math.round(frameworks.reduce((s, f) => s + f.pct, 0) / frameworks.length) : 0;

  const kpis: KpiSpec[] = [
    { label: tr('Référentiels', 'Frameworks'), val: frameworks.length, icon: Layers, col: 'var(--accent)', onClick: () => navigate('/compliance') },
    { label: tr('Couverture moy.', 'Avg. coverage'), val: avg, icon: ClipboardCheck, col: 'var(--low)', suffix: '%', onClick: () => navigate('/compliance') },
    { label: tr('Contrôles', 'Controls'), val: totalControls, icon: ListChecks, col: 'var(--high)', onClick: () => navigate('/compliance') },
    { label: tr('Écarts', 'Gaps'), val: gaps, icon: AlertTriangle, col: 'var(--critical)', onClick: () => navigate('/compliance/gaps') },
  ];

  return (
    <DashboardShell>
      <PersonaHeader
        title={tr('Conformité', 'Compliance')}
        subtitle={tr('Couverture, écarts et audits par référentiel.', 'Coverage, gaps and audits per framework.')}
        actionLabel={tr('Audits', 'Audits')}
        onAction={() => navigate('/compliance/audits')}
      />

      <div className="mb-4">
        <WidgetState
          lang={lang}
          isLoading={isLoading}
          error={query.error}
          skeletonHeight={140}
          retry={() => void query.refetch()}
        >
          <KpiRow items={kpis} />
        </WidgetState>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[1.6fr_1fr] gap-4">
        <Card style={{ padding: '18px 20px' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-semibold text-ink">{tr('Couverture par référentiel', 'Coverage by framework')}</div>
            <button onClick={() => navigate('/compliance')} className="text-[12px] font-semibold text-accent hover:underline">{tr('Gérer', 'Manage')}</button>
          </div>
          <WidgetState
            lang={lang}
            isLoading={isLoading}
            error={query.error}
            skeletonHeight={200}
            retry={() => void query.refetch()}
            isEmpty={frameworks.length === 0}
            emptyIcon={Layers}
            emptyTitle={tr('Aucun référentiel importé', 'No framework imported')}
            emptyDescription={tr(
              'Importez ISO 27001, NIST CSF, PCI DSS ou l’un des autres catalogues : la couverture par référentiel se calcule ensuite automatiquement.',
              'Import ISO 27001, NIST CSF, PCI DSS or one of the other catalogues: coverage is computed from there automatically.'
            )}
            emptyAction={
              <button onClick={() => navigate('/compliance')} className="h-[34px] px-4 rounded-[9px] text-[12.5px] font-semibold text-fg-primary" style={{ background: 'var(--accent)' }}>
                {tr('Ajouter un référentiel', 'Add a framework')}
              </button>
            }
          >
            <div className="space-y-3.5">
              {frameworks.map((f, i) => {
                const col = frameworkColorFor(f.name, i);
                return (
                  <button
                    key={f.id}
                    // The row names a framework; the link opens THAT framework.
                    onClick={() => navigate(`/compliance?framework=${encodeURIComponent(String(f.id))}`)}
                    className="w-full text-left group"
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[12.5px] font-medium text-ink truncate">{f.name}</span>
                      <span className="mono text-[12px] font-semibold shrink-0 ml-2" style={{ color: col }}>{f.pct}%</span>
                    </div>
                    <div className="h-[7px] rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                      <div className="h-full rounded-full" style={{ width: `${f.pct}%`, background: col, transition: 'width .7s cubic-bezier(.2,.8,.2,1)' }} />
                    </div>
                    <div className="text-[10.5px] text-ink-muted mt-0.5">{f.passed}/{f.total} {tr('contrôles', 'controls')}</div>
                  </button>
                );
              })}
            </div>
          </WidgetState>
        </Card>

        <Card style={{ padding: '18px 20px' }}>
          <div className="text-[14px] font-semibold text-ink mb-4">{tr('Actions', 'Actions')}</div>
          <div className="space-y-2.5">
            {[
              { label: tr('Analyse des écarts', 'Gap analysis'), sub: tr('Ce qui reste à implémenter', 'What is left to implement'), to: '/compliance/gaps', icon: AlertTriangle, col: 'var(--critical)' },
              { label: tr('Audits', 'Audits'), sub: tr('Planifier & suivre', 'Plan & track'), to: '/compliance/audits', icon: ClipboardCheck, col: 'var(--accent)' },
              { label: tr('Remédiations', 'Remediations'), sub: tr('Plans en cours', 'Open plans'), to: '/compliance/remediation', icon: ListChecks, col: 'var(--high)' },
            ].map((a) => {
              const Icon = a.icon;
              return (
                <button key={a.to} onClick={() => navigate(a.to)} className="w-full flex items-center gap-3 p-2.5 rounded-[10px] hover:bg-hover transition-colors text-left">
                  <div className="w-[32px] h-[32px] rounded-[9px] flex items-center justify-center shrink-0" style={{ color: a.col, background: `color-mix(in srgb, ${a.col} 14%, transparent)` }}>
                    <Icon size={16} strokeWidth={1.8} />
                  </div>
                  <div className="min-w-0">
                    <div className="text-[13px] font-medium text-ink">{a.label}</div>
                    <div className="text-[11px] text-ink-muted truncate">{a.sub}</div>
                  </div>
                </button>
              );
            })}
          </div>
        </Card>
      </div>
    </DashboardShell>
  );
}
