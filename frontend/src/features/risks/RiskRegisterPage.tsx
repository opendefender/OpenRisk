// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Risk Register (OpenRisk.dc.html §6.3) — wired to the real /risks store. Criticality
// chips, a filter/search bar, a dense table with multi-select + floating bulk bar
// (real bulk delete), a per-row action menu (view/edit/export/delete), and a
// right-side detail drawer with Details / Score / Mitigations tabs. From the drawer
// you can edit the risk, export it, and create a linked mitigation plan.

import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { Filter, Upload, Plus, X, MoreHorizontal, FileText, Pencil, Trash2, Eye, Download, ShieldCheck, ShieldAlert, Clock, Search, Rows3, LayoutGrid, Check, ArrowRight, ArrowLeft, RotateCcw, Coins, Route as RouteIcon } from 'lucide-react';
import {
  PageFrame, PageHeader, Btn, Chip, Card, CritBadge, StatusPill, Avatar, FwBadge, arcPath,
  SkeletonRows, EmptyState, softFill,
} from '../../shared/ui';
import { scoreColor, critColor } from '../../shared/riskColors';
import type { Criticality } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useRiskStore, type RiskPhase } from '../../hooks/useRiskStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { mapRisk, type UiRisk } from './riskMap';
import { EditRiskModal } from './components/EditRiskModal';
import { CreateMitigationModal } from '../mitigations/CreateMitigationModal';

type Tab = 'all' | 'critical' | 'high' | 'review';

// exportRiskCsv downloads a single risk as CSV (client-side — no per-risk export
// endpoint). Mirrors the incident/risk-register CSV UX.
function exportRiskCsv(r: UiRisk) {
  const cols: [string, string | number][] = [
    ['id', r.id], ['name', r.name], ['description', r.desc ?? ''], ['asset', r.asset],
    ['score', r.score], ['criticality', r.crit], ['status', r.status], ['framework', r.fw],
    ['owner', r.ownerName], ['probability', r.prob], ['impact', r.impact], ['asset_criticality', r.ac],
    ['updated', r.mod],
  ];
  const esc = (v: unknown) => {
    const s = v == null ? '' : String(v);
    return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  const csv = `${cols.map((c) => c[0]).join(',')}\n${cols.map((c) => esc(c[1])).join(',')}`;
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `risk-${r.id.slice(0, 8)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

export function RiskRegisterPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const risks = useRiskStore((s) => s.risks);
  const total = useRiskStore((s) => s.total);
  const isLoading = useRiskStore((s) => s.isLoading);
  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const deleteRisk = useRiskStore((s) => s.deleteRisk);

  const [tab, setTab] = useState<Tab>('all');
  const [view, setView] = useState<'table' | 'map'>('table');
  const [sel, setSel] = useState<string[]>([]);
  const [drawerId, setDrawerId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showSearch, setShowSearch] = useState(false);
  const [query, setQuery] = useState('');
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [editRaw, setEditRaw] = useState<UiRisk['raw'] | null>(null);
  const [mitiRiskId, setMitiRiskId] = useState<string | null>(null);

  useEffect(() => { fetchRisks().catch(() => {}); }, [fetchRisks]);

  const ui: UiRisk[] = useMemo(() => risks.map((r) => mapRisk(r, lang)), [risks, lang]);
  const critCount = ui.filter((r) => r.crit === 'critical').length;
  const highCount = ui.filter((r) => r.crit === 'high').length;
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return ui
      .filter((r) => (tab === 'all' ? true : tab === 'critical' ? r.crit === 'critical' : tab === 'high' ? r.crit === 'high' : r.status === 'open'))
      .filter((r) => (q ? `${r.name} ${r.asset} ${r.fw} ${r.ownerName}`.toLowerCase().includes(q) : true));
  }, [ui, tab, query]);
  const drawer = drawerId ? ui.find((r) => r.id === drawerId) ?? null : null;

  const toggle = (id: string) => setSel((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  const removeRisk = async (r: UiRisk) => {
    setMenuFor(null);
    if (!window.confirm(tr(`Supprimer le risque « ${r.name} » ?`, `Delete risk "${r.name}"?`))) return;
    try {
      await deleteRisk(r.id);
      toast.success(tr('Risque supprimé', 'Risk deleted'));
      if (drawerId === r.id) setDrawerId(null);
    } catch {
      toast.error(tr('Suppression échouée', 'Delete failed'));
    }
  };

  const bulkDelete = async () => {
    if (!sel.length) return;
    setBusy(true);
    try {
      await Promise.all(sel.map((id) => deleteRisk(id)));
      toast.success(tr(`${sel.length} risque(s) supprimé(s)`, `${sel.length} risk(s) deleted`));
      setSel([]);
    } catch {
      toast.error(tr('Suppression échouée', 'Delete failed'));
    } finally {
      setBusy(false);
    }
  };

  const th = (t: string, w?: string) => (
    <th className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]" style={{ width: w }}>{t}</th>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={L.riskTitle}
        count={`${total} ${tr('risques', 'risks')}`}
        actions={
          <>
            <div className="inline-flex rounded-[10px] p-0.5" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
              {([['table', Rows3, tr('Table', 'Table')], ['map', LayoutGrid, tr('Matrice', 'Matrix')]] as const).map(([v, Icon, lbl]) => (
                <button
                  key={v}
                  onClick={() => setView(v)}
                  className="h-8 px-2.5 rounded-[8px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 transition-colors"
                  style={{ background: view === v ? 'var(--accent-soft)' : 'transparent', color: view === v ? 'var(--accent)' : 'var(--text-secondary)' }}
                  title={lbl}
                >
                  <Icon size={15} /> <span className="hidden sm:inline">{lbl}</span>
                </button>
              ))}
            </div>
            <Btn label={L.filters} icon={showSearch ? X : Filter} onClick={() => { setShowSearch((v) => !v); if (showSearch) setQuery(''); }} />
            <Btn label={L.importCsv} icon={Upload} onClick={() => navigate('/risks/import')} />
            <Btn label={L.newRisk} icon={Plus} primary onClick={() => window.dispatchEvent(new CustomEvent('openrisk:new-risk'))} />
          </>
        }
      />

      {showSearch && (
        <div className="mb-3 flex items-center gap-2.5 h-11 px-3.5 rounded-[12px]" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)', animation: 'or-fadeup .2s ease' }}>
          <Search size={16} className="text-ink-muted shrink-0" />
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={tr('Rechercher par nom, actif, référentiel, responsable…', 'Search by name, asset, framework, owner…')}
            className="flex-1 bg-transparent text-[13.5px] text-ink outline-none"
          />
          {query && <button onClick={() => setQuery('')} className="text-ink-muted hover:text-ink transition-colors"><X size={15} /></button>}
        </div>
      )}

      <div className="flex gap-2 mb-4 flex-wrap">
        <Chip label={L.all} active={tab === 'all'} onClick={() => setTab('all')} />
        <Chip label={`${L.critical} · ${critCount}`} active={tab === 'critical'} onClick={() => setTab('critical')} color="var(--critical)" />
        <Chip label={`${L.high} · ${highCount}`} active={tab === 'high'} onClick={() => setTab('high')} color="var(--high)" />
        <Chip label={L.pendingReview} active={tab === 'review'} onClick={() => setTab('review')} />
      </div>

      <Card style={{ padding: '8px 8px 4px', overflow: 'hidden' }}>
        {isLoading && ui.length === 0 ? (
          <SkeletonRows rows={6} />
        ) : ui.length === 0 ? (
          <EmptyState
            icon={ShieldAlert}
            title={tr('Aucun risque pour le moment', 'No risks yet')}
            sub={tr('Créez votre premier risque pour commencer à cartographier votre exposition.', 'Create your first risk to start mapping your exposure.')}
            cta={<Btn label={L.newRisk} icon={Plus} primary onClick={() => window.dispatchEvent(new CustomEvent('openrisk:new-risk'))} />}
          />
        ) : filtered.length === 0 ? (
          <EmptyState icon={Search} title={tr('Aucun résultat', 'No results')} sub={tr('Aucun risque ne correspond à votre recherche.', 'No risk matches your search.')} />
        ) : view === 'map' ? (
          <RiskMatrixView risks={filtered} onOpen={setDrawerId} />
        ) : (
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
                    <tr key={r.id} onClick={() => setDrawerId(r.id)} className="cursor-pointer transition-colors hover:bg-hover">
                      <td className="px-3 py-[13px]" onClick={(e) => { e.stopPropagation(); toggle(r.id); }}>
                        <div className="w-[17px] h-[17px] rounded-[5px] flex items-center justify-center" style={{ border: `1.5px solid ${checked ? 'var(--accent)' : 'var(--border-strong)'}`, background: checked ? 'var(--accent)' : 'transparent' }}>
                          {checked && <svg viewBox="0 0 24 24" width={12} height={12} fill="none" stroke="#fff" strokeWidth={3} strokeLinecap="round" strokeLinejoin="round"><path d="m5 12 5 5L20 7" /></svg>}
                        </div>
                      </td>
                      <td className="px-3 py-[13px]">
                        <div className="text-[13.5px] font-medium text-ink max-w-[340px] truncate">{r.name}</div>
                        <div className="mono text-[11px] text-ink-muted mt-0.5">#{r.id.slice(0, 8)} · {r.asset}</div>
                      </td>
                      <td className="px-3 py-[13px]"><span className="mono text-[15px] font-bold" style={{ color: scoreColor(r.score) }}>{r.score.toFixed(1)}</span></td>
                      <td className="px-3 py-[13px]"><CritBadge crit={r.crit} /></td>
                      <td className="px-3 py-[13px]"><StatusPill status={r.status} /></td>
                      <td className="px-3 py-[13px]">{r.fw !== '—' ? <FwBadge fw={r.fw} /> : <span className="text-ink-muted text-[12px]">—</span>}</td>
                      <td className="px-3 py-[13px]">{r.owner !== '—' ? <Avatar initials={r.owner} title={r.ownerName} /> : <span className="text-ink-muted text-[12px]">—</span>}</td>
                      <td className="px-3 py-[13px] text-[12px] text-ink-soft whitespace-nowrap">{r.mod}</td>
                      <td className="px-3 py-[13px] relative" onClick={(e) => e.stopPropagation()}>
                        <button
                          onClick={() => setMenuFor(menuFor === r.id ? null : r.id)}
                          className="w-7 h-7 rounded-[7px] flex items-center justify-center text-ink-muted hover:bg-hover transition-colors"
                          aria-label={tr('Actions', 'Actions')}
                        >
                          <MoreHorizontal size={17} />
                        </button>
                        {menuFor === r.id && (
                          <RowMenu
                            onView={() => { setMenuFor(null); setDrawerId(r.id); }}
                            onEdit={() => { setMenuFor(null); setEditRaw(r.raw); }}
                            onExport={() => { setMenuFor(null); exportRiskCsv(r); }}
                            onDelete={() => removeRisk(r)}
                            onClose={() => setMenuFor(null)}
                          />
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {sel.length > 0 && (
        <div className="fixed bottom-6 z-[60] glass-strong rounded-[14px] shadow-card-lg px-3.5 py-2.5 flex items-center gap-3.5" style={{ left: 'calc(50% + 100px)', transform: 'translateX(-50%)', animation: 'or-fadeup .2s ease' }}>
          <span className="text-[13px] font-semibold text-ink">{sel.length} {tr(`sélectionné${sel.length > 1 ? 's' : ''}`, 'selected')}</span>
          <span className="w-px h-5" style={{ background: 'var(--border-strong)' }} />
          <button onClick={bulkDelete} disabled={busy} className="h-8 px-3 rounded-lg text-[12.5px] font-semibold disabled:opacity-60" style={{ background: 'color-mix(in srgb,var(--critical) 14%,transparent)', color: 'var(--critical)' }}>{L.del}</button>
        </div>
      )}

      {drawer && (
        <RiskDrawer
          r={drawer}
          onClose={() => setDrawerId(null)}
          onEdit={() => setEditRaw(drawer.raw)}
          onExport={() => exportRiskCsv(drawer)}
          onCreateMiti={() => setMitiRiskId(drawer.id)}
        />
      )}

      <EditRiskModal
        isOpen={!!editRaw}
        risk={editRaw}
        onClose={() => setEditRaw(null)}
        onSuccess={() => { setEditRaw(null); void fetchRisks(); }}
      />
      <CreateMitigationModal
        isOpen={!!mitiRiskId}
        riskId={mitiRiskId ?? undefined}
        onClose={() => setMitiRiskId(null)}
        onCreated={() => { setMitiRiskId(null); void fetchRisks(); toast.success(tr('Plan de mitigation lié au risque', 'Mitigation plan linked to the risk')); }}
      />
    </PageFrame>
  );
}

/* ---------------- risk matrix (map) view ---------------- */
// Standard 5×5 GRC risk map: Impact (x, 1→5) × Probability (y, 5→1). Each risk is
// bucketed from its real prob (0–1) and impact (0–10); the cell tint is the cell's
// own severity (prob-bucket × impact-bucket), and each risk chip opens its drawer.
function cellCrit(p: number, i: number): Criticality {
  const v = p * i;
  return v >= 15 ? 'critical' : v >= 8 ? 'high' : v >= 4 ? 'medium' : 'low';
}
function RiskMatrixView({ risks, onOpen }: { risks: UiRisk[]; onOpen: (id: string) => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const bucket = (v: number, max: number) => Math.min(5, Math.max(1, Math.ceil((v / max) * 5) || 1));
  // grid[prob 1..5][impact 1..5] → risks
  const grid: Record<number, Record<number, UiRisk[]>> = {};
  for (let p = 1; p <= 5; p++) { grid[p] = {}; for (let i = 1; i <= 5; i++) grid[p][i] = []; }
  for (const r of risks) grid[bucket(r.prob, 1)][bucket(r.impact, 10)].push(r);

  return (
    <div className="p-4 overflow-x-auto">
      <div className="flex gap-2" style={{ minWidth: 640 }}>
        {/* y-axis label */}
        <div className="flex items-center">
          <span className="text-[11px] font-semibold uppercase tracking-[.06em] text-ink-muted" style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)' }}>{tr('Probabilité', 'Probability')}</span>
        </div>
        <div className="flex-1">
          <div className="grid gap-1.5" style={{ gridTemplateColumns: 'repeat(5,1fr)' }}>
            {[5, 4, 3, 2, 1].map((p) =>
              [1, 2, 3, 4, 5].map((i) => {
                const cell = grid[p][i];
                const crit = cellCrit(p, i);
                const col = critColor[crit];
                return (
                  <div key={`${p}-${i}`} className="rounded-[10px] p-1.5 min-h-[84px] flex flex-col gap-1" style={{ background: softFill(col, 10), border: `1px solid ${softFill(col, 22)}` }}>
                    <div className="flex flex-wrap gap-1 content-start">
                      {cell.slice(0, 6).map((r) => (
                        <button
                          key={r.id}
                          onClick={() => onOpen(r.id)}
                          title={`${r.name} · ${r.score.toFixed(1)}`}
                          className="w-5 h-5 rounded-full text-[9px] font-bold text-white flex items-center justify-center transition-transform hover:scale-110"
                          style={{ background: col }}
                        >
                          {r.score.toFixed(0)}
                        </button>
                      ))}
                      {cell.length > 6 && <span className="text-[10px] font-semibold self-center" style={{ color: col }}>+{cell.length - 6}</span>}
                    </div>
                  </div>
                );
              })
            )}
          </div>
          {/* x-axis ticks */}
          <div className="grid gap-1.5 mt-1.5" style={{ gridTemplateColumns: 'repeat(5,1fr)' }}>
            {[1, 2, 3, 4, 5].map((i) => <div key={i} className="text-center text-[10.5px] text-ink-muted">{i}</div>)}
          </div>
          <div className="text-center text-[11px] font-semibold uppercase tracking-[.06em] text-ink-muted mt-1">{tr('Impact', 'Impact')}</div>
        </div>
      </div>
    </div>
  );
}

/* ---------------- row action menu ---------------- */
function RowMenu({ onView, onEdit, onExport, onDelete, onClose }: { onView: () => void; onEdit: () => void; onExport: () => void; onDelete: () => void; onClose: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const item = (icon: React.ReactNode, label: string, onClick: () => void, danger?: boolean) => (
    <button onClick={onClick} className="w-full flex items-center gap-2.5 px-3 py-2 text-[13px] font-medium hover:bg-hover transition-colors" style={{ color: danger ? 'var(--critical)' : 'var(--text-primary)' }}>
      {icon} {label}
    </button>
  );
  return (
    <>
      <div className="fixed inset-0 z-[64]" onClick={onClose} aria-hidden="true" />
      <div className="absolute right-2 top-9 z-[65] w-[176px] rounded-[11px] overflow-hidden shadow-card-lg" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', animation: 'or-scalein .14s cubic-bezier(.2,.8,.2,1)' }}>
        {item(<Eye size={15} />, tr('Voir', 'View'), onView)}
        {item(<Pencil size={15} />, L.edit, onEdit)}
        {item(<Download size={15} />, tr('Exporter CSV', 'Export CSV'), onExport)}
        <div style={{ borderTop: '1px solid var(--border)' }} />
        {item(<Trash2 size={15} />, L.del, onDelete, true)}
      </div>
    </>
  );
}

/* ---------------- drawer ---------------- */
function RiskDrawer({ r, onClose, onEdit, onExport, onCreateMiti }: { r: UiRisk; onClose: () => void; onEdit: () => void; onExport: () => void; onCreateMiti: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [tab, setTab] = useState<'details' | 'lifecycle' | 'score' | 'financial' | 'miti' | 'timeline' | 'cti' | 'ai'>('details');
  const tabDef: [typeof tab, string][] = [
    ['details', L.tab_details], ['lifecycle', tr('Cycle de vie', 'Lifecycle')], ['score', L.tab_score],
    ['financial', tr('Financier', 'Financial')], ['miti', L.tab_miti],
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
              <div className="mono text-[11px] text-ink-muted mb-[5px]">#{r.id.slice(0, 8)}</div>
              <div className="disp text-[18px] font-bold text-ink leading-snug">{r.name}</div>
            </div>
            <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft" style={{ background: 'var(--bg-hover)' }}><X size={18} /></button>
          </div>
          <div className="flex items-center gap-2.5 flex-wrap">
            <CritBadge crit={r.crit} />
            <StatusPill status={r.status} />
            <PhasePill phase={r.phase} lang={lang} />
            <span className="mono text-[13px] font-bold ml-auto" style={{ color: scoreColor(r.score) }}>Score {r.score.toFixed(1)}</span>
          </div>
          <div className="flex gap-2 mt-3.5">
            <Btn label={L.edit} icon={Pencil} onClick={onEdit} />
            <Btn label={L.exportCsv} icon={FileText} onClick={onExport} />
          </div>
        </div>

        <div className="flex gap-0.5 px-[22px] overflow-x-auto" style={{ borderBottom: '1px solid var(--border)' }}>
          {tabDef.map(([k, lbl]) => (
            <button key={k} onClick={() => setTab(k)} className="px-3 py-[11px] text-[13px] whitespace-nowrap" style={{ color: tab === k ? 'var(--text-primary)' : 'var(--text-secondary)', fontWeight: tab === k ? 600 : 500, borderBottom: `2px solid ${tab === k ? 'var(--accent)' : 'transparent'}`, marginBottom: -1 }}>{lbl}</button>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto">
          {tab === 'details' && <DrawerDetails r={r} onCreateMiti={onCreateMiti} />}
          {tab === 'lifecycle' && <DrawerLifecycle r={r} />}
          {tab === 'score' && <DrawerScore r={r} />}
          {tab === 'financial' && <DrawerFinancial r={r} />}
          {tab === 'miti' && <DrawerMiti r={r} onCreateMiti={onCreateMiti} />}
          {(tab === 'timeline' || tab === 'cti' || tab === 'ai') && <div className="py-10 px-[22px] text-center text-[13px] text-ink-soft">{L.soon}</div>}
        </div>
      </div>
    </div>
  );
}

function DrawerDetails({ r, onCreateMiti }: { r: UiRisk; onCreateMiti: () => void }) {
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
      <button onClick={onCreateMiti} className="mt-2 w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-white transition-all hover:brightness-110" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
        <ShieldCheck size={16} /> {L.createMiti}
      </button>
    </div>
  );
}

function DrawerScore({ r }: { r: UiRisk }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const gauge = (val: number, max: number, lbl: string, col: string) => {
    const pct = Math.max(0, Math.min(1, val / max)), cx = 52, cy = 52, rr = 42;
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

/* ---------------- lifecycle (ISO 31000) ---------------- */
const PHASE_ORDER: RiskPhase[] = ['identified', 'analyzed', 'evaluated', 'treated', 'monitored', 'closed'];
const PHASE_LABELS: Record<RiskPhase, [string, string]> = {
  identified: ['Identifier', 'Identify'],
  analyzed: ['Analyser', 'Analyze'],
  evaluated: ['Évaluer', 'Evaluate'],
  treated: ['Traiter', 'Treat'],
  monitored: ['Surveiller', 'Monitor'],
  closed: ['Clôturer', 'Close'],
};
function phaseLabel(p: RiskPhase, lang: 'fr' | 'en') { return PHASE_LABELS[p][lang === 'fr' ? 0 : 1]; }
function phaseIdx(p: RiskPhase) { return PHASE_ORDER.indexOf(p); }
// Mirrors domain.RiskPhase.CanTransitionTo on the backend (±1 step, →closed from
// anywhere, or reopen from closed). The backend is the source of truth; this only
// decides which buttons to show.
function canTransition(from: RiskPhase, to: RiskPhase): boolean {
  const f = phaseIdx(from), t = phaseIdx(to);
  if (f < 0 || t < 0 || f === t) return false;
  if (to === 'closed') return true;
  if (from === 'closed') return true;
  return Math.abs(t - f) === 1;
}

function PhasePill({ phase, lang }: { phase: RiskPhase; lang: 'fr' | 'en' }) {
  const closed = phase === 'closed';
  const col = closed ? 'var(--text-secondary)' : 'var(--accent)';
  return (
    <span className="inline-flex items-center gap-1.5 h-[22px] px-2.5 rounded-full text-[11.5px] font-semibold" style={{ color: col, background: 'color-mix(in srgb,var(--accent) 12%,transparent)' }}>
      <RouteIcon size={12} /> {phaseLabel(phase, lang)}
    </span>
  );
}

function DrawerLifecycle({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const transitionPhase = useRiskStore((s) => s.transitionPhase);
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);
  const current = r.phase;
  const curIdx = phaseIdx(current);

  const go = async (to: RiskPhase) => {
    setBusy(true);
    try {
      await transitionPhase(r.id, to, note.trim() || undefined);
      toast.success(tr(`Phase : ${phaseLabel(to, lang)}`, `Phase: ${phaseLabel(to, lang)}`));
      setNote('');
    } catch {
      toast.error(tr('Transition refusée', 'Transition rejected'));
    } finally {
      setBusy(false);
    }
  };

  const prev = curIdx > 0 ? PHASE_ORDER[curIdx - 1] : null;
  const next = curIdx < PHASE_ORDER.length - 1 ? PHASE_ORDER[curIdx + 1] : null;

  return (
    <div className="px-[22px] py-5">
      <div className="text-[13px] text-ink-soft mb-4">{tr('Cycle de vie du risque — ISO 31000. La phase est indépendante du statut.', 'Risk lifecycle — ISO 31000. Phase is independent of status.')}</div>

      {/* Vertical stepper */}
      <div className="relative pl-1">
        {PHASE_ORDER.map((p, i) => {
          const done = curIdx > i || current === 'closed' && i < 5;
          const isCurrent = p === current;
          const dotBg = isCurrent ? 'var(--accent)' : done ? 'var(--low)' : 'var(--bg-hover)';
          const dotFg = isCurrent || done ? '#fff' : 'var(--text-muted)';
          return (
            <div key={p} className="flex items-start gap-3 pb-1">
              <div className="flex flex-col items-center">
                <div className="w-[26px] h-[26px] rounded-full flex items-center justify-center text-[12px] font-bold shrink-0" style={{ background: dotBg, color: dotFg, boxShadow: isCurrent ? '0 0 0 4px color-mix(in srgb,var(--accent) 20%,transparent)' : 'none' }}>
                  {done ? <Check size={14} /> : i + 1}
                </div>
                {i < PHASE_ORDER.length - 1 && <div className="w-[2px] h-6" style={{ background: curIdx > i ? 'var(--low)' : 'var(--border)' }} />}
              </div>
              <div className="pt-0.5">
                <div className="text-[13.5px] font-semibold" style={{ color: isCurrent ? 'var(--accent)' : 'var(--text-primary)' }}>{phaseLabel(p, lang)}</div>
                {isCurrent && <div className="text-[11.5px] text-ink-muted">{tr('Phase actuelle', 'Current phase')}</div>}
              </div>
            </div>
          );
        })}
      </div>

      {/* Actions */}
      {canUpdate ? (
        <div className="mt-5 rounded-[12px] p-3.5" style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}>
          <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">{tr('Note (optionnelle)', 'Note (optional)')}</div>
          <textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={2}
            placeholder={tr('Justification de la transition…', 'Transition rationale…')}
            className="w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none mb-3"
            style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}
          />
          <div className="flex flex-wrap gap-2">
            {prev && canTransition(current, prev) && (
              <button disabled={busy} onClick={() => go(prev)} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
                <ArrowLeft size={14} /> {phaseLabel(prev, lang)}
              </button>
            )}
            {next && canTransition(current, next) && (
              <button disabled={busy} onClick={() => go(next)} className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 text-white disabled:opacity-60" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
                {phaseLabel(next, lang)} <ArrowRight size={14} />
              </button>
            )}
            {current !== 'closed' && (
              <button disabled={busy} onClick={() => go('closed')} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
                <Check size={14} /> {tr('Clôturer', 'Close')}
              </button>
            )}
            {current === 'closed' && (
              <button disabled={busy} onClick={() => go('monitored')} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
                <RotateCcw size={14} /> {tr('Rouvrir', 'Reopen')}
              </button>
            )}
          </div>
        </div>
      ) : (
        <div className="mt-5 text-[12.5px] text-ink-muted">{tr('Lecture seule — permission « risks:update » requise pour faire évoluer la phase.', 'Read-only — the “risks:update” permission is required to advance the phase.')}</div>
      )}
    </div>
  );
}

/* ---------------- financial (CRQ) ---------------- */
function DrawerFinancial({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const updateRisk = useRiskStore((s) => s.updateRisk);
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const raw = r.raw;
  const [sle, setSle] = useState(raw.sle_xaf != null ? String(raw.sle_xaf) : '');
  const [aro, setAro] = useState(raw.aro != null ? String(raw.aro) : '');
  const [busy, setBusy] = useState(false);

  const fmtXAF = (v?: number) => (v == null ? '—' : `${Math.round(v).toLocaleString('fr-FR')} FCFA`);
  const fmtUSD = (v?: number) => (v == null ? '—' : `$${v.toLocaleString('en-US', { maximumFractionDigits: 0 })}`);

  const save = async () => {
    setBusy(true);
    try {
      await updateRisk(r.id, {
        sle_xaf: sle.trim() === '' ? null : Number(sle),
        aro: aro.trim() === '' ? null : Number(aro),
      });
      toast.success(tr('Exposition recalculée', 'Exposure recalculated'));
    } catch {
      toast.error(tr('Échec du recalcul', 'Recalculation failed'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="px-[22px] py-5">
      <div className="text-[13px] text-ink-soft mb-4">{tr('Quantification (CRQ) — perte annuelle attendue ALE = SLE × ARO.', 'Quantification (CRQ) — annual loss expectancy ALE = SLE × ARO.')}</div>

      <div className="grid grid-cols-2 gap-3 mb-3">
        <div className="rounded-[12px] p-4" style={{ border: '1px solid color-mix(in srgb,var(--accent) 30%,transparent)', background: 'color-mix(in srgb,var(--accent) 6%,transparent)' }}>
          <div className="text-[10.5px] uppercase tracking-[.06em] text-ink-muted">ALE (FCFA)</div>
          <div className="mono text-[20px] font-bold text-ink mt-1">{fmtXAF(raw.ale_xaf)}</div>
        </div>
        <div className="rounded-[12px] p-4" style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}>
          <div className="text-[10.5px] uppercase tracking-[.06em] text-ink-muted">ALE (USD)</div>
          <div className="mono text-[20px] font-bold text-ink mt-1">{fmtUSD(raw.ale_usd)}</div>
        </div>
      </div>
      <div className="text-[11.5px] text-ink-muted mb-4">
        {tr('Base : ', 'Basis: ')}
        {raw.ale_basis === 'explicit'
          ? tr('saisie explicite (SLE × ARO)', 'explicit input (SLE × ARO)')
          : tr('valeur de référence par criticité', 'reference value by criticality')}
      </div>

      {canUpdate && (
        <div className="rounded-[12px] p-3.5" style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}>
          <div className="grid grid-cols-2 gap-3 mb-3">
            <label className="block">
              <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{tr('SLE — Perte / sinistre (FCFA)', 'SLE — Loss / event (FCFA)')}</span>
              <input value={sle} onChange={(e) => setSle(e.target.value)} type="number" className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }} />
            </label>
            <label className="block">
              <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{tr('ARO — Fréquence / an', 'ARO — Frequency / yr')}</span>
              <input value={aro} onChange={(e) => setAro(e.target.value)} type="number" step="0.1" className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }} />
            </label>
          </div>
          <button disabled={busy} onClick={save} className="w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-white disabled:opacity-60" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
            <Coins size={16} /> {tr('Recalculer l’exposition', 'Recalculate exposure')}
          </button>
          <div className="text-[11px] text-ink-muted mt-2">{tr('Laissez vides pour utiliser la valeur de référence par criticité.', 'Leave empty to use the reference value by criticality.')}</div>
        </div>
      )}
    </div>
  );
}

function DrawerMiti({ r, onCreateMiti }: { r: UiRisk; onCreateMiti: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const linked = r.raw.mitigations ?? [];
  if (!linked.length) {
    return (
      <div className="py-10 px-[22px] text-center">
        <div className="text-[13px] text-ink-soft mb-3.5">{lang === 'fr' ? 'Aucun plan de mitigation lié.' : 'No linked mitigation plan.'}</div>
        <div className="flex justify-center"><Btn label={L.createMiti} icon={Plus} primary onClick={onCreateMiti} /></div>
      </div>
    );
  }
  return (
    <div className="px-[22px] py-5">
      {linked.map((x) => (
        <div key={x.id} className="p-3.5 rounded-[12px] mb-2.5" style={{ border: '1px solid var(--border)' }}>
          <div className="text-[13.5px] font-medium text-ink mb-2.5">{x.title}</div>
          <div className="h-[5px] rounded-[5px] overflow-hidden mb-2" style={{ background: 'var(--bg-hover)' }}>
            <div className="h-full rounded-[5px]" style={{ width: `${x.progress ?? 0}%`, background: 'var(--low)' }} />
          </div>
          <div className="flex items-center justify-between text-[11.5px] text-ink-muted">
            <span>{x.progress ?? 0}%</span>
            <span className="inline-flex items-center gap-1"><Clock size={12} /> {x.status}</span>
          </div>
        </div>
      ))}
      <div className="flex justify-center mt-2"><Btn label={L.createMiti} icon={Plus} onClick={onCreateMiti} /></div>
    </div>
  );
}
