// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Simulations — Risk Digital Twin (OpenRisk.dc.html §6.7): impact gauge + blast
// radius chain, AI-suggested scenarios, run history, and the "propagating…"
// overlay (pulsing rings + floating cpu, ~1.9s).

import { useState } from 'react';
import { Plus, Sparkles, Cpu, ChevronRight } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, RingGauge } from '../../shared/ui';
import { scoreColor, critColor, softFill, type Criticality } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

export function SimulationsPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [running, setRunning] = useState(false);

  const run = () => { setRunning(true); setTimeout(() => setRunning(false), 1900); };

  const vectorCol: Record<string, string> = { network: 'var(--info)', ransomware: 'var(--critical)', insider: 'var(--high)' };
  const scenarios: [string, string, string, number][] = [
    [tr('Ransomware via poste RH', 'Ransomware via HR endpoint'), 'ransomware', 'iot-badge', 8.9],
    [tr('Compromission chaîne CI/CD', 'CI/CD supply-chain compromise'), 'network', 'ci-runner', 7.2],
    [tr('Menace interne — accès DBA', 'Insider threat — DBA access'), 'insider', 'db-core-01', 6.5],
  ];
  const hist: [string, string, string, number, string][] = [
    [tr('Serveur de paie compromis', 'Payroll server compromised'), 'ransomware', 'srv-paie-01', 8.4, tr('il y a 2 j', '2 d ago')],
    [tr('Fuite passerelle bancaire', 'Banking gateway breach'), 'network', 'gw-bank-02', 6.1, tr('il y a 5 j', '5 d ago')],
    [tr('Escalade IAM cloud', 'Cloud IAM escalation'), 'insider', 'aws-prod', 7.8, tr('la semaine dernière', 'last week')],
  ];
  const blast: [string, Criticality, string][] = [
    ['srv-paie-01', 'critical', tr('Origine', 'Origin')], ['db-core-01', 'high', '+3'],
    ['aws-prod', 'critical', '+6'], ['backup-nas', 'medium', '+4'],
  ];

  return (
    <>
      <PageFrame>
        <PageHeader title={L.simTitle} actions={<Btn label={L.simNew} icon={Plus} primary onClick={run} />} />
        <div className="text-[13.5px] text-ink-soft -mt-2 mb-5">{L.simSub}</div>

        {/* hero */}
        <Card style={{ padding: '22px 24px', marginBottom: 16 }}>
          <div className="flex items-center gap-2 mb-4">
            <span className="text-[11px] font-bold uppercase tracking-[.06em] text-ink-muted">{L.simLast}</span>
            <span className="text-[11.5px] text-ink-muted">· {tr('il y a 2 j', '2 d ago')}</span>
          </div>
          <div className="flex gap-6 items-center flex-wrap">
            <div className="flex flex-col items-center gap-2">
              <RingGauge value={84} size={150} color="var(--critical)">
                <span className="disp mono text-[38px] font-bold text-ink leading-none">8.4</span>
                <span className="text-[11px] text-ink-muted mt-0.5">/ 10</span>
              </RingGauge>
              <span className="text-[12px] font-semibold" style={{ color: 'var(--critical)' }}>{L.simImpact}</span>
            </div>
            <div className="flex-1 min-w-[280px]">
              <div className="text-[13px] font-semibold text-ink mb-3.5">{L.simBlast} · 14 {L.simAffected}</div>
              <div className="flex items-center gap-2.5 flex-wrap">
                {blast.map(([name, crit, tag], i) => (
                  <div key={name} className="flex items-center gap-2.5">
                    <div className="flex flex-col items-center gap-1.5">
                      <div className="mono px-3 py-2 rounded-[11px] text-[12px] font-semibold text-ink" style={{ background: softFill(critColor[crit], 12), border: `1px solid ${critColor[crit]}` }}>{name}</div>
                      <span className="text-[10.5px] font-semibold" style={{ color: critColor[crit] }}>{tag}</span>
                    </div>
                    {i < blast.length - 1 && <ChevronRight size={18} className="text-ink-muted" />}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </Card>

        {/* AI scenarios */}
        <div className="text-[14px] font-semibold text-ink mb-3.5 flex items-center gap-2"><Sparkles size={17} /> {L.simSuggested}</div>
        <div className="grid gap-4 mb-4" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))' }}>
          {scenarios.map(([name, vec, asset, score]) => (
            <Card key={name} style={{ padding: 18 }}>
              <div className="flex items-center justify-between mb-3">
                <span className="text-[11px] font-semibold uppercase tracking-[.04em] px-[9px] py-[3px] rounded-full" style={{ color: vectorCol[vec], background: softFill(vectorCol[vec], 14) }}>{vec}</span>
                <span className="mono text-[15px] font-bold" style={{ color: scoreColor(score) }}>{score.toFixed(1)}</span>
              </div>
              <div className="text-[14px] font-semibold text-ink mb-2 leading-snug">{name}</div>
              <div className="text-[12px] text-ink-muted mb-4">{L.simTrigger} · {asset}</div>
              <button onClick={run} className="w-full h-[34px] rounded-[9px] text-[12.5px] font-semibold text-ink inline-flex items-center justify-center gap-1.5 hover:bg-hover transition-colors" style={{ border: '1px solid var(--border-strong)' }}>
                <Cpu size={15} /> {L.simRun}
              </button>
            </Card>
          ))}
        </div>

        {/* history */}
        <Card style={{ padding: '18px 22px' }}>
          <div className="text-[14px] font-semibold text-ink mb-3.5">{L.simHistory}</div>
          {hist.map(([name, vec, asset, score, date], i) => (
            <div key={name} onClick={run} className="flex items-center gap-3.5 py-3 px-1 rounded-lg cursor-pointer hover:bg-hover transition-colors" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
              <div className="w-[34px] h-[34px] rounded-[9px] flex items-center justify-center text-ink-soft shrink-0" style={{ background: 'var(--bg-hover)' }}><Cpu size={17} /></div>
              <div className="flex-1 min-w-0">
                <div className="text-[13.5px] font-medium text-ink">{name}</div>
                <div className="mono text-[11.5px] text-ink-muted mt-0.5">{asset} · {vec} · {date}</div>
              </div>
              <span className="mono text-[15px] font-bold" style={{ color: scoreColor(score) }}>{score.toFixed(1)}</span>
            </div>
          ))}
        </Card>
      </PageFrame>

      {running && (
        <div className="fixed inset-0 z-[90] flex flex-col items-center justify-center gap-5" style={{ background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(8px)', animation: 'or-fadein .16s ease' }}>
          <div className="relative" style={{ width: 90, height: 90 }}>
            {[0, 1, 2].map((i) => (
              <div key={i} className="absolute inset-0 rounded-full" style={{ border: '2px solid var(--accent)', opacity: 0.3 - i * 0.08, animation: `or-pulse 1.6s ${i * 0.3}s infinite` }} />
            ))}
            <div className="absolute rounded-2xl flex items-center justify-center text-white" style={{ inset: 26, background: 'linear-gradient(135deg,var(--accent),var(--accent-2))', animation: 'or-float 2s ease-in-out infinite' }}><Cpu size={22} /></div>
          </div>
          <div className="text-[15px] font-semibold text-white">{L.simRunning}</div>
        </div>
      )}
    </>
  );
}
