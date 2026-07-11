// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Reports (OpenRisk.dc.html §6.14): a grid of report templates (icon + copy +
// generate) and a recent-reports list. The Board Report has its own screen.

import { TrendingUp, FileText, ClipboardCheck, Siren, ShieldAlert, Atom, Sparkles, type LucideIcon } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { PageFrame, PageHeader, Card } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

export function ReportsScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const tpls: [string, string, LucideIcon, string | null][] = [
    [tr('Synthèse exécutive', 'Executive summary'), tr('Vue d’ensemble de la posture pour le COMEX', 'Posture overview for the executive committee'), TrendingUp, null],
    [tr('Rapport Conseil', 'Board report'), tr('Reporting trimestriel de gouvernance', 'Quarterly governance reporting'), FileText, '/reports/board'],
    [tr('Conformité ISO 27001', 'ISO 27001 compliance'), tr('État détaillé des 114 contrôles', 'Detailed status of 114 controls'), ClipboardCheck, null],
    [tr('Post-mortem d’incident', 'Incident post-mortem'), tr('Chronologie et actions correctives', 'Timeline and corrective actions'), Siren, null],
    [tr('Export du registre', 'Register export'), tr('Tous les risques en CSV ou PDF', 'All risks as CSV or PDF'), ShieldAlert, null],
    [tr('Rapport Asset Universe', 'Asset Universe report'), tr('Cartographie et chemins d’attaque', 'Topology and attack paths'), Atom, null],
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
        {tpls.map(([title, desc, Icon, to]) => (
          <Card key={title} style={{ padding: 20 }}>
            <div className="w-[42px] h-[42px] rounded-xl flex items-center justify-center mb-3.5" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}><Icon size={21} /></div>
            <div className="text-[14.5px] font-semibold text-ink mb-1.5">{title}</div>
            <div className="text-[12.5px] text-ink-soft leading-relaxed mb-4" style={{ minHeight: 36 }}>{desc}</div>
            <button onClick={() => to && navigate(to)} className="w-full h-9 rounded-[10px] text-[13px] font-semibold text-ink inline-flex items-center justify-center gap-1.5 hover:bg-hover transition-colors" style={{ border: '1px solid var(--border-strong)' }}>
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
