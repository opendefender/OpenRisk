// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// CISO Analytics (OpenRisk.dc.html §6.9): 4 KPI tiles, animated horizontal bars
// (risks by framework) and a conic-gradient donut (distribution by criticality).

import { FileText, ArrowUp, ArrowDown } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useNavigate } from 'react-router-dom';

export function AnalyticsCiso() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const kpis: [string, string, string, boolean][] = [
    ['72/100', tr('Score global', 'Global score'), '+3', true],
    [tr('4,2 j', '4.2 d'), tr('MTTR moyen', 'Avg MTTR'), tr('-0,8 j', '-0.8 d'), true],
    ['€2,4M', tr('Exposition financière', 'Financial exposure'), '-6 %', true],
    ['78 %', tr('Couverture conformité', 'Compliance coverage'), '+4 %', true],
  ];
  const bars: [string, number, string][] = [
    ['ISO 27001', 42, '#7c6cff'], ['NIST', 31, '#0a84ff'], ['SOC 2', 24, '#64d2ff'],
    ['BCEAO', 19, '#30d158'], ['DORA', 15, '#ff2d92'], ['ANSSI', 11, '#ff9f0a'],
  ];
  const maxB = Math.max(...bars.map((b) => b[1]));
  const seg: [string, number, string][] = [
    [tr('Critique', 'Critical'), 12, 'var(--critical)'], [tr('Élevé', 'High'), 45, 'var(--high)'],
    [tr('Moyen', 'Medium'), 120, 'var(--medium)'], [tr('Faible', 'Low'), 70, 'var(--low)'],
  ];
  const totalR = seg.reduce((a, s) => a + s[1], 0);
  let acc = 0;
  const stops = seg.map(([, v, col]) => { const a0 = (acc / totalR) * 100; acc += v; const a1 = (acc / totalR) * 100; return `${col} ${a0}% ${a1}%`; }).join(',');

  return (
    <PageFrame>
      <PageHeader title={L.n_analytics} actions={<Btn label={L.genReport} icon={FileText} primary onClick={() => navigate('/reports')} />} />
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
        {kpis.map(([val, label, delta, up]) => (
          <Card key={label} style={{ padding: '16px 18px' }}>
            <div className="flex items-center justify-between mb-3">
              <span className="text-[12.5px] text-ink-soft">{label}</span>
              <span className="inline-flex items-center gap-0.5 text-[11.5px] font-semibold" style={{ color: up ? 'var(--low)' : 'var(--critical)' }}>
                {up ? <ArrowUp size={12} /> : <ArrowDown size={12} />}{delta}
              </span>
            </div>
            <div className="disp mono text-[27px] font-bold text-ink">{val}</div>
          </Card>
        ))}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card style={{ padding: '18px 22px' }}>
          <div className="text-[14px] font-semibold text-ink mb-3.5">{tr('Risques par référentiel', 'Risks by framework')}</div>
          <div className="flex flex-col gap-3.5">
            {bars.map(([lbl, v, col]) => (
              <div key={lbl} className="flex items-center gap-3">
                <span className="w-[78px] text-[12.5px] text-ink-soft shrink-0">{lbl}</span>
                <div className="flex-1 h-2.5 rounded-md overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                  <div className="h-full rounded-md" style={{ width: `${(v / maxB) * 100}%`, background: col, transition: 'width .9s cubic-bezier(.2,.8,.2,1)' }} />
                </div>
                <span className="mono w-[26px] text-[13px] font-semibold text-ink text-right">{v}</span>
              </div>
            ))}
          </div>
        </Card>
        <Card style={{ padding: '18px 22px' }}>
          <div className="text-[14px] font-semibold text-ink mb-3.5">{tr('Répartition par criticité', 'Distribution by criticality')}</div>
          <div className="flex items-center gap-[22px]">
            <div className="relative shrink-0" style={{ width: 132, height: 132, borderRadius: '50%', background: `conic-gradient(${stops})` }}>
              <div className="absolute flex flex-col items-center justify-center rounded-full" style={{ inset: 22, background: 'var(--bg-elevated)' }}>
                <span className="disp mono text-[24px] font-bold text-ink">{totalR}</span>
                <span className="text-[10.5px] text-ink-muted">{tr('risques', 'risks')}</span>
              </div>
            </div>
            <div className="flex-1 flex flex-col gap-[11px]">
              {seg.map(([lbl, v, col]) => (
                <div key={lbl} className="flex items-center gap-2.5">
                  <span className="w-2.5 h-2.5 rounded-[3px] shrink-0" style={{ background: col }} />
                  <span className="flex-1 text-[13px] text-ink-soft">{lbl}</span>
                  <span className="mono text-[13px] font-semibold text-ink">{v}</span>
                </div>
              ))}
            </div>
          </div>
        </Card>
      </div>
    </PageFrame>
  );
}
