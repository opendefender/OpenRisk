// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Risk Register (OpenRisk.dc.html §6.3): criticality chips, dense table with
// multi-select + floating bulk bar, and a right-side detail drawer with
// Details / Score / Mitigations tabs. Fixture-driven to match the handoff.

import { useMemo, useState } from 'react';
import { Filter, Server, Plus, X, MoreHorizontal, FileText, Settings, ShieldCheck, Clock } from 'lucide-react';
import {
  PageFrame, PageHeader, Btn, Chip, Card, CritBadge, StatusPill, Avatar, FwBadge, arcPath,
  type RiskStatus,
} from '../../shared/ui';
import { scoreColor, critColor } from '../../shared/riskColors';
import { RISKS, ALL_MITIGATIONS, type FxRisk } from '../../shared/fixtures';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

type Tab = 'all' | 'critical' | 'high' | 'review';

export function RiskRegisterPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const [tab, setTab] = useState<Tab>('all');
  const [sel, setSel] = useState<string[]>([]);
  const [drawer, setDrawer] = useState<string | null>(null);

  const critCount = RISKS.filter((r) => r.crit === 'critical').length;
  const highCount = RISKS.filter((r) => r.crit === 'high').length;
  const filtered = useMemo(
    () => RISKS.filter((r) => (tab === 'all' ? true : tab === 'critical' ? r.crit === 'critical' : tab === 'high' ? r.crit === 'high' : r.status === 'open')),
    [tab]
  );
  const toggle = (id: string) => setSel((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  const th = (t: string, w?: string) => (
    <th className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]" style={{ width: w }}>{t}</th>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={L.riskTitle}
        count={`${RISKS.length} ${lang === 'fr' ? 'risques' : 'risks'}`}
        actions={
          <>
            <Btn label={L.filters} icon={Filter} />
            <Btn label={L.importCsv} icon={Server} />
            <Btn label={L.newRisk} icon={Plus} primary onClick={() => setDrawer('new')} />
          </>
        }
      />

      <div className="flex gap-2 mb-4 flex-wrap">
        <Chip label={L.all} active={tab === 'all'} onClick={() => setTab('all')} />
        <Chip label={`${L.critical} · ${critCount}`} active={tab === 'critical'} onClick={() => setTab('critical')} color="var(--critical)" />
        <Chip label={`${L.high} · ${highCount}`} active={tab === 'high'} onClick={() => setTab('high')} color="var(--high)" />
        <Chip label={L.pendingReview} active={tab === 'review'} onClick={() => setTab('review')} />
      </div>

      <Card style={{ padding: '8px 8px 4px', overflow: 'hidden' }}>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse" style={{ minWidth: 820 }}>
            <thead style={{ borderBottom: '1px solid var(--border)' }}>
              <tr>
                {th('', '34px')}{th(L.col_name)}{th(L.col_score)}{th(L.col_crit)}{th(L.col_status)}{th(L.col_fw)}{th(L.col_owner)}{th(L.col_mod)}{th('')}
              </tr>
            </thead>
            <tbody>
              {filtered.map((r) => {
                const checked = sel.includes(r.id);
                return (
                  <tr
                    key={r.id}
                    onClick={() => setDrawer(r.id)}
                    className="cursor-pointer transition-colors hover:bg-hover"
                  >
                    <td className="px-3 py-[13px]" onClick={(e) => { e.stopPropagation(); toggle(r.id); }}>
                      <div
                        className="w-[17px] h-[17px] rounded-[5px] flex items-center justify-center"
                        style={{ border: `1.5px solid ${checked ? 'var(--accent)' : 'var(--border-strong)'}`, background: checked ? 'var(--accent)' : 'transparent' }}
                      >
                        {checked && (
                          <svg viewBox="0 0 24 24" width={12} height={12} fill="none" stroke="#fff" strokeWidth={3} strokeLinecap="round" strokeLinejoin="round"><path d="m5 12 5 5L20 7" /></svg>
                        )}
                      </div>
                    </td>
                    <td className="px-3 py-[13px]">
                      <div className="text-[13.5px] font-medium text-ink max-w-[340px] truncate">{r.name}</div>
                      <div className="mono text-[11px] text-ink-muted mt-0.5">{r.id} · {r.asset}</div>
                    </td>
                    <td className="px-3 py-[13px]"><span className="mono text-[15px] font-bold" style={{ color: scoreColor(r.score) }}>{r.score.toFixed(1)}</span></td>
                    <td className="px-3 py-[13px]"><CritBadge crit={r.crit} /></td>
                    <td className="px-3 py-[13px]"><StatusPill status={r.status} /></td>
                    <td className="px-3 py-[13px]"><FwBadge fw={r.fw} /></td>
                    <td className="px-3 py-[13px]"><Avatar initials={r.owner} title={r.ownerName} /></td>
                    <td className="px-3 py-[13px] text-[12px] text-ink-soft whitespace-nowrap">{r.mod}</td>
                    <td className="px-3 py-[13px]" onClick={(e) => e.stopPropagation()}>
                      <button className="w-7 h-7 rounded-[7px] flex items-center justify-center text-ink-muted hover:bg-hover transition-colors"><MoreHorizontal size={17} /></button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Card>

      {sel.length > 0 && (
        <div
          className="fixed bottom-6 z-[60] glass-strong rounded-[14px] shadow-card-lg px-3.5 py-2.5 flex items-center gap-3.5"
          style={{ left: 'calc(50% + 100px)', transform: 'translateX(-50%)', animation: 'or-fadeup .2s ease' }}
        >
          <span className="text-[13px] font-semibold text-ink">
            {sel.length} {lang === 'fr' ? `sélectionné${sel.length > 1 ? 's' : ''}` : 'selected'}
          </span>
          <span className="w-px h-5" style={{ background: 'var(--border-strong)' }} />
          <Btn label={L.exportCsv} icon={FileText} />
          <button onClick={() => setSel([])} className="h-8 px-3 rounded-lg text-[12.5px] font-semibold" style={{ background: 'color-mix(in srgb,var(--critical) 14%,transparent)', color: 'var(--critical)' }}>{L.del}</button>
        </div>
      )}

      {drawer && <RiskDrawer id={drawer} onClose={() => setDrawer(null)} />}
    </PageFrame>
  );
}

/* ---------------- drawer ---------------- */

const NEW_RISK: FxRisk = {
  id: 'RSK-NEW', name: 'Nouveau risque', crit: 'medium', score: 5.0, prob: 0.5, impact: 6.0, ac: 1.0,
  asset: '—', fw: 'ISO27001', status: 'open', ownerName: 'Amir Diallo', owner: 'AD', mod: 'à l’instant', desc: 'Décrivez le risque…',
};

function RiskDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const L = useUIStrings();
  const [tab, setTab] = useState<'details' | 'score' | 'miti' | 'timeline' | 'cti' | 'ai'>('details');
  const r = id === 'new' ? NEW_RISK : RISKS.find((x) => x.id === id);
  if (!r) return null;

  const tabDef: [typeof tab, string][] = [
    ['details', L.tab_details], ['score', L.tab_score], ['miti', L.tab_miti],
    ['timeline', L.tab_timeline], ['cti', L.tab_cti], ['ai', L.tab_ai],
  ];

  return (
    <div className="fixed inset-0 z-[70] flex justify-end" style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)', animation: 'or-fadein .2s ease' }} onClick={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="h-full flex flex-col"
        style={{ width: 'min(94vw,560px)', background: 'var(--bg-secondary)', borderLeft: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-slidein .3s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="px-[22px] pt-5 pb-3.5">
          <div className="flex items-start gap-3 mb-3">
            <div className="flex-1">
              <div className="mono text-[11px] text-ink-muted mb-[5px]">{r.id}</div>
              <div className="disp text-[18px] font-bold text-ink leading-snug">{r.name}</div>
            </div>
            <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft" style={{ background: 'var(--bg-hover)' }}><X size={18} /></button>
          </div>
          <div className="flex items-center gap-2.5 flex-wrap">
            <CritBadge crit={r.crit} />
            <StatusPill status={r.status as RiskStatus} />
            <span className="mono text-[13px] font-bold ml-auto" style={{ color: scoreColor(r.score) }}>Score {r.score.toFixed(1)}</span>
          </div>
          <div className="flex gap-2 mt-3.5">
            <Btn label={L.edit} icon={Settings} />
            <Btn label={L.exportPdf} icon={FileText} />
            <Btn icon={MoreHorizontal} />
          </div>
        </div>

        <div className="flex gap-0.5 px-[22px] overflow-x-auto" style={{ borderBottom: '1px solid var(--border)' }}>
          {tabDef.map(([k, lbl]) => (
            <button
              key={k}
              onClick={() => setTab(k)}
              className="px-3 py-[11px] text-[13px] whitespace-nowrap"
              style={{ color: tab === k ? 'var(--text-primary)' : 'var(--text-secondary)', fontWeight: tab === k ? 600 : 500, borderBottom: `2px solid ${tab === k ? 'var(--accent)' : 'transparent'}`, marginBottom: -1 }}
            >
              {lbl}
            </button>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto">
          {tab === 'details' && <DrawerDetails r={r} onCreateMiti={() => setTab('miti')} />}
          {tab === 'score' && <DrawerScore r={r} />}
          {tab === 'miti' && <DrawerMiti r={r} />}
          {(tab === 'timeline' || tab === 'cti' || tab === 'ai') && (
            <div className="py-10 px-[22px] text-center text-[13px] text-ink-soft">{L.soon}</div>
          )}
        </div>
      </div>
    </div>
  );
}

function DrawerDetails({ r, onCreateMiti }: { r: FxRisk; onCreateMiti: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const field = (lbl: string, val: string) => (
    <div className="mb-4">
      <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">{lbl}</div>
      <div className="text-[13.5px] text-ink">{val}</div>
    </div>
  );
  return (
    <div className="px-[22px] py-5">
      {field('Description', r.desc || '—')}
      <div className="grid grid-cols-2 gap-x-5">
        {field(lang === 'fr' ? 'Actif concerné' : 'Asset', r.asset)}
        {field('Framework', r.fw)}
        {field(L.col_owner, r.ownerName)}
        {field(L.col_mod, r.mod)}
      </div>
      <button onClick={onCreateMiti} className="mt-2 w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
        <ShieldCheck size={16} /> {L.createMiti}
      </button>
    </div>
  );
}

function DrawerScore({ r }: { r: FxRisk }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const gauge = (val: number, max: number, lbl: string, col: string) => {
    const pct = val / max, cx = 52, cy = 52, rr = 42;
    const track = arcPath(cx, cy, rr, -130, 130);
    const prog = arcPath(cx, cy, rr, -130, -130 + 260 * pct);
    return (
      <div className="text-center">
        <div className="relative mx-auto" style={{ width: 104, height: 92 }}>
          <svg viewBox="0 0 104 96" width={104} height={96}>
            <path d={track} fill="none" stroke="var(--bg-hover)" strokeWidth={9} strokeLinecap="round" />
            <path d={prog} fill="none" stroke={col} strokeWidth={9} strokeLinecap="round" />
          </svg>
          <div className="mono absolute left-0 right-0 text-center text-[20px] font-bold text-ink" style={{ top: 30 }}>{val.toFixed(1)}</div>
        </div>
        <div className="text-[11.5px] text-ink-soft font-medium">{lbl}</div>
      </div>
    );
  };
  return (
    <div className="p-[22px]">
      <div className="flex justify-around mb-[22px]">
        {gauge(r.prob * 10, 10, L.proba, 'var(--accent)')}
        {gauge(r.impact, 10, L.impact, 'var(--high)')}
        {gauge(r.ac, 3, lang === 'fr' ? 'Criticité actif' : 'Asset criticality', 'var(--info)')}
      </div>
      <div className="text-center p-[18px] rounded-[14px]" style={{ background: 'var(--bg-hover)' }}>
        <div className="mono text-[15px] text-ink-soft">
          <span>{r.prob.toFixed(1)}</span><span className="mx-2 text-ink-muted">×</span>
          <span>{r.impact.toFixed(1)}</span><span className="mx-2 text-ink-muted">×</span>
          <span>{r.ac.toFixed(1)}</span><span className="mx-2.5 text-ink-muted">=</span>
          <span className="text-[22px] font-bold" style={{ color: scoreColor(r.score) }}>{r.score.toFixed(1)}</span>
        </div>
        <div className="text-[12px] text-ink-muted mt-2">{lang === 'fr' ? 'Probabilité × Impact × Criticité de l’actif' : 'Probability × Impact × Asset criticality'}</div>
      </div>
    </div>
  );
}

function DrawerMiti({ r }: { r: FxRisk }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const linked = ALL_MITIGATIONS.filter((x) => x.risk === r.id);
  if (!linked.length) {
    return (
      <div className="py-10 px-[22px] text-center">
        <div className="text-[13px] text-ink-soft mb-3.5">{lang === 'fr' ? 'Aucun plan de mitigation lié.' : 'No linked mitigation plan.'}</div>
        <div className="flex justify-center"><Btn label={L.createMiti} icon={Plus} primary /></div>
      </div>
    );
  }
  return (
    <div className="px-[22px] py-5">
      {linked.map((x) => (
        <div key={x.id} className="p-3.5 rounded-[12px] mb-2.5" style={{ border: '1px solid var(--border)' }}>
          <div className="text-[13.5px] font-medium text-ink mb-2.5">{x.title}</div>
          <div className="h-[5px] rounded-[5px] overflow-hidden mb-2" style={{ background: 'var(--bg-hover)' }}>
            <div className="h-full rounded-[5px]" style={{ width: `${x.progress}%`, background: 'var(--low)' }} />
          </div>
          <div className="flex items-center justify-between text-[11.5px] text-ink-muted">
            <span>{x.progress}%</span>
            <span className="inline-flex items-center gap-1"><Clock size={12} /> {x.deadline}</span>
          </div>
        </div>
      ))}
    </div>
  );
}

/* keep critColor referenced for tree-shaking parity */
void critColor;
