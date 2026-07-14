// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Reports (OpenRisk.dc.html §6.14): a grid of report templates wired to real
// destinations/exports (Board report, Compliance PDFs, risk-register CSV export…),
// plus a recent-reports list.

import { TrendingUp, FileText, ClipboardCheck, Siren, ShieldAlert, Atom, Sparkles, type LucideIcon } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { PageFrame, PageHeader, Card } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { riskService } from '../../services/riskService';
import { exportIncidentsCsv } from '../incidents/incidentService';

export function ReportsScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

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
    [tr('Synthèse exécutive', 'Executive summary'), tr('Vue d’ensemble de la posture pour le COMEX', 'Posture overview for the executive committee'), TrendingUp, () => navigate('/analytics')],
    [tr('Rapport Conseil', 'Board report'), tr('Reporting trimestriel de gouvernance', 'Quarterly governance reporting'), FileText, () => navigate('/reports/board')],
    [tr('Conformité', 'Compliance'), tr('Rapport PDF détaillé par référentiel', 'Detailed PDF report per framework'), ClipboardCheck, () => navigate('/compliance')],
    [tr('Registre d’incidents', 'Incident register'), tr('Tous les incidents en CSV', 'All incidents as CSV'), Siren, exportIncidents],
    [tr('Export du registre', 'Register export'), tr('Tous les risques en CSV', 'All risks as CSV'), ShieldAlert, exportRegister],
    [tr('Rapport Asset Universe', 'Asset Universe report'), tr('Cartographie et chemins d’attaque', 'Topology and attack paths'), Atom, () => navigate('/assets/universe')],
  ];
  const recent: [string, string, string][] = [
    [tr('Synthèse exécutive — Juin 2026', 'Executive summary — June 2026'), 'PDF', tr('02 juil. 2026', 'Jul 02, 2026')],
    [tr('Conformité ISO 27001', 'ISO 27001 compliance'), 'PDF', tr('28 juin 2026', 'Jun 28, 2026')],
    [tr('Export du registre des risques', 'Risk register export'), 'CSV', tr('15 juin 2026', 'Jun 15, 2026')],
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
      <Card style={{ padding: '18px 22px' }}>
        <div className="text-[14px] font-semibold text-ink mb-3.5">{tr('Rapports récents', 'Recent reports')}</div>
        {recent.map(([name, fmt, date], i) => (
          <div key={name} className="flex items-center gap-3.5 py-3 px-1" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
            <div className="w-[34px] h-[34px] rounded-[9px] flex items-center justify-center text-ink-soft shrink-0" style={{ background: 'var(--bg-hover)' }}><FileText size={17} /></div>
            <div className="flex-1">
              <div className="text-[13.5px] font-medium text-ink">{name}</div>
              <div className="text-[11.5px] text-ink-muted mt-0.5">{fmt} · {date}</div>
            </div>
            <button className="h-8 px-3 rounded-lg text-[12.5px] font-semibold text-ink" style={{ border: '1px solid var(--border-strong)' }}>{tr('Télécharger', 'Download')}</button>
          </div>
        ))}
      </Card>
    </PageFrame>
  );
}
