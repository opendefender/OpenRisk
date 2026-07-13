// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Compliance (OpenRisk.dc.html §6.10) — wired to real /compliance/frameworks +
// per-framework progress. Posture hero (aggregate radial + copy + CTAs) and a grid
// of framework cards; clicking a card downloads its official PDF report.

import { FileText, AlertTriangle, Download, ClipboardCheck, ChevronRight } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { PageFrame, PageHeader, Btn, Card, RingGauge, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useComplianceOverview, frameworkColorFor, type FrameworkWithProgress } from './complianceOverview';
import { useComplianceReport } from './useCompliance';

export function ComplianceScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: fws = [], isLoading } = useComplianceOverview();
  const report = useComplianceReport();

  const overall = fws.length ? Math.round(fws.reduce((a, f) => a + f.pct, 0) / fws.length) : 0;
  const totalControls = fws.reduce((a, f) => a + f.total, 0);
  const gaps = fws.reduce((a, f) => a + Math.max(0, f.total - f.passed), 0);
  const overallColor = overall >= 70 ? 'var(--low)' : overall >= 40 ? 'var(--high)' : 'var(--critical)';

  const download = (f: FrameworkWithProgress) => {
    toast.promise(report.mutateAsync({ frameworkId: f.id, locale: lang }), {
      loading: tr('Génération du rapport…', 'Generating report…'),
      success: tr('Rapport téléchargé', 'Report downloaded'),
      error: tr('Échec de la génération', 'Report generation failed'),
    });
  };

  return (
    <PageFrame>
      <PageHeader title={L.n_compliance} />

      {isLoading ? (
        <Card style={{ padding: 8 }}><SkeletonRows rows={4} height={64} /></Card>
      ) : fws.length === 0 ? (
        <Card>
          <EmptyState
            icon={ClipboardCheck}
            title={tr('Aucun référentiel', 'No frameworks yet')}
            sub={tr('Importez un référentiel (ISO 27001, SOC 2, BCEAO…) pour suivre votre conformité.', 'Import a framework (ISO 27001, SOC 2, BCEAO…) to track your compliance.')}
          />
        </Card>
      ) : (
        <>
          <Card style={{ padding: '22px 24px', marginBottom: 16 }}>
            <div className="flex items-center gap-6 flex-wrap">
              <RingGauge value={overall} size={128} color={overallColor}>
                <span className="disp mono text-[32px] font-bold text-ink">{overall}%</span>
                <span className="text-[11px] text-ink-muted">{tr('conforme', 'compliant')}</span>
              </RingGauge>
              <div className="flex-1 min-w-[280px]">
                <div className="disp text-[19px] font-bold text-ink mb-1.5">{tr('Posture de conformité', 'Compliance posture')}</div>
                <div className="text-[13.5px] text-ink-soft leading-relaxed mb-3.5 max-w-[520px]">
                  {tr(
                    `${totalControls} contrôles suivis sur ${fws.length} référentiel${fws.length > 1 ? 's' : ''}. ${gaps} contrôle${gaps > 1 ? 's' : ''} requièrent une action.`,
                    `${totalControls} controls tracked across ${fws.length} framework${fws.length > 1 ? 's' : ''}. ${gaps} control${gaps > 1 ? 's' : ''} need action.`
                  )}
                </div>
                <div className="flex gap-2.5 flex-wrap">
                  <Btn label={L.genReport} icon={FileText} primary onClick={() => navigate('/reports')} />
                  <Btn label={tr('Voir les écarts', 'View gaps')} icon={AlertTriangle} />
                </div>
              </div>
            </div>
          </Card>

          <div className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))' }}>
            {fws.map((f, i) => {
              const col = frameworkColorFor(f.name, i);
              return (
                <Card key={f.id} style={{ padding: 18 }}>
                  <button onClick={() => navigate(`/compliance/${f.id}`)} className="w-full flex items-center gap-3.5 mb-3.5 text-left group">
                    <RingGauge value={f.pct} size={56} color={col} thickness={6}>
                      <span className="mono text-[13px] font-bold text-ink">{f.pct}</span>
                    </RingGauge>
                    <div className="flex-1 min-w-0">
                      <div className="text-[14px] font-semibold text-ink truncate group-hover:text-accent transition-colors" title={f.name}>{f.name}</div>
                      <div className="text-[12px] text-ink-soft mt-0.5">{f.passed} / {f.total} {tr('contrôles', 'controls')}</div>
                    </div>
                    <ChevronRight size={16} className="text-ink-muted shrink-0 group-hover:text-accent transition-colors" />
                  </button>
                  <div className="flex gap-2">
                    <button
                      onClick={() => navigate(`/compliance/${f.id}`)}
                      className="flex-1 h-8 rounded-[9px] text-[12.5px] font-semibold text-ink inline-flex items-center justify-center gap-1.5 hover:bg-hover transition-colors"
                      style={{ border: '1px solid var(--border-strong)' }}
                    >
                      {tr('Voir les contrôles', 'View controls')}
                    </button>
                    <button
                      onClick={() => download(f)}
                      disabled={report.isPending}
                      className="h-8 px-3 rounded-[9px] text-[12.5px] font-semibold text-ink inline-flex items-center justify-center gap-1.5 hover:bg-hover transition-colors disabled:opacity-60"
                      style={{ border: '1px solid var(--border-strong)' }}
                      title={L.exportPdf}
                    >
                      <Download size={14} />
                    </button>
                  </div>
                </Card>
              );
            })}
          </div>
        </>
      )}
    </PageFrame>
  );
}
