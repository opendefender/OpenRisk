// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Reports (OpenRisk.dc.html §6.14): a grid of report templates wired to real
// destinations/exports (Board report, Compliance PDFs, risk-register CSV export…),
// plus a recent-reports list.

import { TrendingUp, FileText, ClipboardCheck, Siren, ShieldAlert, Atom, Sparkles, Plus, type LucideIcon } from 'lucide-react';
import { useState } from 'react';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';
import { PageFrame, PageHeader, Card, Btn, SkeletonRows } from '../../shared/ui';
import { EmptyState } from '../../shared/EmptyState';
import { useBoardReports } from './useBoardReports';
import { useGenerateReport } from './useReportJobs';
import { useFrameworks } from '../compliance/useCompliance';
import { FrameworkPickerDialog } from './FrameworkPickerDialog';
import { PremiumPeek } from '../../shared/PremiumPeek';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { riskService } from '../../services/riskService';
import { exportIncidentsCsv } from '../incidents/incidentService';

export function ReportsScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { reports, isLoading: reportsLoading, error: reportsError } = useBoardReports();
  const { frameworks } = useFrameworks();
  const generate = useGenerateReport();
  const [picking, setPicking] = useState(false);
  const recent = (reports ?? []).slice(0, 5);

  const exportRegister = async () => {
    try {
      const blob = await riskService.exportRisks({}, 'csv');
      const url = URL.createObjectURL(blob as Blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `risk-register-${new Date().toISOString().slice(0, 10)}.csv`;
      a.click();
      URL.revokeObjectURL(url);
      toast.success(tr('Registre exporté', 'Register exported'));
    } catch {
      toast.error(tr('Export échoué', 'Export failed'));
    }
  };

  const exportIncidents = async () => {
    try {
      const n = await exportIncidentsCsv();
      toast.success(tr(`${n} incident(s) exporté(s)`, `${n} incident(s) exported`));
    } catch {
      toast.error(tr('Export échoué', 'Export failed'));
    }
  };

  const tpls: [string, string, LucideIcon, () => void][] = [
    [tr('Synthèse exécutive', 'Executive summary'), tr('Vue d’ensemble de la posture pour le COMEX', 'Posture overview for the executive committee'), TrendingUp, () => navigate('/?view=executive')],
    [tr('Rapport Conseil', 'Board report'), tr('Reporting trimestriel de gouvernance', 'Quarterly governance reporting'), FileText, () => navigate('/reports/board')],
    // The other half of the loop. This used to navigate to /compliance, whose
    // "Generate report" button navigated straight back here — a closed circuit
    // that never produced a PDF. It now picks a framework and generates, so the
    // journey ends on the document.
    [tr('Conformité', 'Compliance'), tr('Rapport PDF détaillé par référentiel', 'Detailed PDF report per framework'), ClipboardCheck, () => setPicking(true)],
    [tr('Registre d’incidents', 'Incident register'), tr('Tous les incidents en CSV', 'All incidents as CSV'), Siren, exportIncidents],
    [tr('Export du registre', 'Register export'), tr('Tous les risques en CSV', 'All risks as CSV'), ShieldAlert, exportRegister],
    [tr('Rapport Asset Universe', 'Asset Universe report'), tr('Cartographie et chemins d’attaque', 'Topology and attack paths'), Atom, () => navigate('/assets/universe')],
  ];

  return (
    <PageFrame>
      <PageHeader title={L.n_reports} />
      <div className="grid gap-4 mb-7" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))' }}>
        {tpls.map(([title, desc, Icon, run], i) => (
          <Card key={title} style={{ padding: 20, animation: 'or-fadeup .4s ease both', animationDelay: `${i * 0.04}s` }}>
            <div className="w-[42px] h-[42px] rounded-xl flex items-center justify-center mb-3.5" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}><Icon size={21} /></div>
            <div className="text-[14.5px] font-semibold text-ink mb-1.5">{title}</div>
            <div className="text-[12.5px] text-ink-soft leading-relaxed mb-4" style={{ minHeight: 36 }}>{desc}</div>
            <button onClick={run} className="w-full h-9 rounded-[10px] text-[13px] font-semibold text-ink inline-flex items-center justify-center gap-1.5 hover:bg-hover transition-colors" style={{ border: '1px solid var(--border-strong)' }}>
              <Sparkles size={15} /> {tr('Générer', 'Generate')}
            </button>
          </Card>
        ))}
      </div>
      {/* Conversion moment "after a win" (UX-18/19): the user just generated reports —
          tease automatic scheduled delivery (not built yet) as a blurred premium peek
          with an honest "coming soon" CTA, never a hard paywall. */}
      <div className="mb-7">
        <PremiumPeek
          title={tr('Rapports programmés', 'Scheduled reports')}
          benefit={tr(
            'Recevez vos rapports automatiquement par e-mail (hebdo / mensuel) — plus jamais de rapport oublié avant un COMEX.',
            'Get your reports delivered automatically by email (weekly / monthly) — never scramble for a report before a board meeting again.',
          )}
          ctaLabel={tr('Bientôt disponible', 'Coming soon')}
          onUpgrade={() => toast(tr('Rapports programmés — bientôt disponible.', 'Scheduled reports — coming soon.'), { icon: '🗓️' })}
        >
          <div className="p-5">
            <div className="text-[13px] font-semibold text-ink mb-3">{tr('Planification automatique', 'Automatic scheduling')}</div>
            <div className="flex flex-col gap-2">
              {[
                [tr('Synthèse exécutive', 'Executive summary'), tr('Chaque lundi · 08:00', 'Every Monday · 08:00'), tr('E-mail COMEX', 'Board email')],
                [tr('Conformité ISO 27001', 'ISO 27001 compliance'), tr('1er du mois', '1st of the month'), tr('E-mail RSSI', 'CISO email')],
                [tr('Registre des risques', 'Risk register'), tr('Chaque vendredi', 'Every Friday'), tr('E-mail équipe', 'Team email')],
              ].map(([name, when, to]) => (
                <div key={name} className="flex items-center gap-3 rounded-[10px] px-3 py-2.5" style={{ border: '1px solid var(--border)' }}>
                  <div className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}><FileText size={16} /></div>
                  <div className="flex-1 min-w-0">
                    <div className="text-[13px] font-medium text-ink">{name}</div>
                    <div className="text-[11.5px] text-ink-muted">{when} · {to}</div>
                  </div>
                  <div className="w-9 h-5 rounded-full shrink-0" style={{ background: 'var(--accent)' }} />
                </div>
              ))}
            </div>
          </div>
        </PremiumPeek>
      </div>

      <Card style={{ padding: '18px 22px' }}>
        <div className="text-[14px] font-semibold text-ink mb-3.5">{tr('Rapports récents', 'Recent reports')}</div>
        {/* The tenant's actual generated board reports. This list used to be three
            invented PDFs with invented dates, offering a Download button that did
            nothing — a fresh tenant appeared to have a reporting history. */}
        {reportsLoading ? (
          <SkeletonRows rows={3} height={52} />
        ) : reportsError ? (
          <EmptyState
            variant="error"
            title={tr('Rapports indisponibles', 'Reports unavailable')}
            description={tr('Impossible de charger vos rapports générés.', 'Could not load your generated reports.')}
          />
        ) : recent.length === 0 ? (
          <EmptyState
            variant="first-use"
            icon={FileText}
            title={tr('Aucun rapport généré', 'No reports yet')}
            description={tr('Les rapports que vous générez sont archivés ici, prêts à être relus ou téléchargés. Commencez par un rapport Conseil.', 'Reports you generate are archived here, ready to review or download. Start with a board report.')}
            primaryAction={<Btn label={tr('Générer un rapport Conseil', 'Generate a board report')} icon={Plus} primary onClick={() => navigate('/reports/board')} />}
          />
        ) : (
          recent.map((r, i) => (
            <button
              key={r.id}
              onClick={() => navigate(`/reports/board?focus=${r.id}`)}
              className="w-full text-left flex items-center gap-3.5 py-3 px-1 hover:bg-hover transition-colors"
              style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}
            >
              <div className="w-[34px] h-[34px] rounded-[9px] flex items-center justify-center text-ink-soft shrink-0" style={{ background: 'var(--bg-hover)' }}><FileText size={17} /></div>
              <div className="flex-1 min-w-0">
                <div className="text-[13.5px] font-medium text-ink truncate">{r.title}</div>
                <div className="text-[11.5px] text-ink-muted mt-0.5">
                  {r.period_label} · {new Date(r.created_at).toLocaleDateString(lang === 'fr' ? 'fr-FR' : 'en-US', { day: '2-digit', month: 'short', year: 'numeric' })}
                </div>
              </div>
              <span className="text-[11.5px] font-semibold px-2 py-[3px] rounded-md shrink-0" style={{ color: 'var(--text-secondary)', background: 'var(--bg-hover)' }}>
                {r.status === 'approved' ? tr('Approuvé', 'Approved') : tr('Brouillon', 'Draft')}
              </span>
            </button>
          ))
        )}
      </Card>
      {picking && (
        <FrameworkPickerDialog
          frameworks={frameworks ?? []}
          busy={generate.isPending}
          onClose={() => setPicking(false)}
          onPick={(id) =>
            generate.mutate(
              { kind: 'compliance_framework', params: { framework_id: id, locale: lang } },
              { onSettled: () => setPicking(false) },
            )
          }
        />
      )}
    </PageFrame>
  );
}
