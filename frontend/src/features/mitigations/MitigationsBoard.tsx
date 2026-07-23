// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Mitigations (OpenRisk.dc.html §6.4) — wired to the real /mitigations list, with
// three selectable views: Kanban (To do → In progress → In review → Done), a dense
// Table, and a Gantt timeline positioned by real start/due dates. Loading skeleton
// + empty state on all three.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient, useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Plus, Clock, ShieldCheck, KanbanSquare, Rows3, GanttChartSquare, X, ChevronDown, Check, Loader2 } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Avatar, Skeleton, EmptyState, softFill } from '../../shared/ui';
import { critColor } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { mitigationService, type BoardStatus } from '../../services/mitigationService';
import { useMitigations, type Column, type UiMiti } from './useMitigations';
import { useMitigationEvents } from './useMitigationEvents';

// Backend status ⇄ board column. A freshly-created plan is PLANNED (the "todo" column).
const COL_TO_STATUS: Record<Column, BoardStatus> = { todo: 'PLANNED', progress: 'IN_PROGRESS', review: 'REVIEW', done: 'DONE' };
const STATUS_TO_COL: Record<string, Column> = { PLANNED: 'todo', TODO: 'todo', IN_PROGRESS: 'progress', REVIEW: 'review', DONE: 'done' };

type View = 'kanban' | 'table' | 'gantt';

export function MitigationsBoard() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { columns, items, isLoading } = useMitigations();
  const [view, setView] = useState<View>('kanban');
  const [selected, setSelected] = useState<UiMiti | null>(null);

  // Live scanner-driven auto-completions push over SSE → refresh the board so the
  // "Auto-detected" badge appears without a manual reload.
  useMitigationEvents();

  const cols: [Column, string, string][] = [
    ['todo', L.col_todo, 'var(--text-muted)'],
    ['progress', L.col_doing, 'var(--high)'],
    ['review', L.col_review, 'var(--info)'],
    ['done', L.col_done, 'var(--low)'],
  ];
  const statusLabel: Record<Column, string> = { todo: L.col_todo, progress: L.col_doing, review: L.col_review, done: L.col_done };
  const statusColor: Record<Column, string> = { todo: 'var(--text-muted)', progress: 'var(--high)', review: 'var(--info)', done: 'var(--low)' };

  const viewBtns: [View, typeof KanbanSquare, string][] = [
    ['kanban', KanbanSquare, tr('Kanban', 'Kanban')],
    ['table', Rows3, tr('Table', 'Table')],
    ['gantt', GanttChartSquare, tr('Gantt', 'Gantt')],
  ];

  return (
    <PageFrame wide>
      <PageHeader
        title={L.mitiTitle}
        actions={
          <>
            <div className="inline-flex rounded-[10px] p-0.5" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
              {viewBtns.map(([v, Icon, lbl]) => (
                <button key={v} onClick={() => setView(v)} title={lbl} className="h-8 px-2.5 rounded-[8px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 transition-colors" style={{ background: view === v ? 'var(--accent-soft)' : 'transparent', color: view === v ? 'var(--accent)' : 'var(--text-secondary)' }}>
                  <Icon size={15} /> <span className="hidden sm:inline">{lbl}</span>
                </button>
              ))}
            </div>
            <Btn label={L.addPlan} icon={Plus} primary onClick={() => navigate('/risks')} />
          </>
        }
      />

      {!isLoading && items.length === 0 ? (
        <EmptyState
          icon={ShieldCheck}
          title={tr('Aucune mitigation', 'No mitigations yet')}
          sub={tr('Créez une mitigation depuis un risque pour lancer son traitement.', 'Create a mitigation from a risk to start treating it.')}
          cta={<Btn label={L.n_risks} onClick={() => navigate('/risks')} primary />}
        />
      ) : view === 'kanban' ? (
        <div className="overflow-x-auto -mx-1 px-1">
          <div className="grid gap-3.5 items-start" style={{ gridTemplateColumns: 'repeat(4,minmax(0,1fr))', minWidth: 760 }}>
            {cols.map(([key, label, col]) => (
              <div key={key} className="rounded-[14px] p-3" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', minHeight: 200 }}>
                <div className="flex items-center justify-between mb-3 px-1">
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full" style={{ background: col }} />
                    <span className="text-[12.5px] font-semibold text-ink">{label}</span>
                    <span className="text-[11px] font-semibold text-ink-muted">{columns[key].length}</span>
                  </div>
                  <button onClick={() => navigate('/risks')} className="w-[22px] h-[22px] rounded-md flex items-center justify-center text-ink-muted hover:bg-hover transition-colors"><Plus size={14} /></button>
                </div>
                {isLoading ? (
                  <div className="flex flex-col gap-2.5">{[0, 1].map((i) => <Skeleton key={i} style={{ height: 92 }} />)}</div>
                ) : (
                  columns[key].map((c) => <KanbanCard key={c.id} c={c} onOpen={() => setSelected(c)} />)
                )}
              </div>
            ))}
          </div>
        </div>
      ) : view === 'table' ? (
        <TableView items={items} isLoading={isLoading} statusLabel={statusLabel} statusColor={statusColor} onOpen={setSelected} />
      ) : (
        <GanttView items={items} isLoading={isLoading} statusColor={statusColor} />
      )}

      {selected && (
        <MitigationDrawer
          miti={selected}
          onClose={() => setSelected(null)}
          onChanged={(s) => setSelected({ ...selected, rawStatus: s, column: STATUS_TO_COL[s] ?? selected.column })}
        />
      )}
    </PageFrame>
  );
}

/* ---------------- detail drawer with status control ---------------- */
function MitigationDrawer({ miti, onClose, onChanged }: { miti: UiMiti; onClose: () => void; onChanged: (status: BoardStatus) => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const qc = useQueryClient();
  const L = useUIStrings();

  const setStatus = useMutation({
    mutationFn: (status: BoardStatus) => mitigationService.setStatus(miti.id, status),
    onSuccess: (_data, status) => {
      onChanged(status);
      qc.invalidateQueries({ queryKey: ['mitigations'] });
      toast.success(tr('Statut mis à jour', 'Status updated'));
    },
    onError: () => toast.error(tr('Échec de la mise à jour', 'Update failed')),
  });

  const current = STATUS_TO_COL[miti.rawStatus] ?? miti.column;
  const steps: [Column, string][] = [
    ['todo', L.col_todo],
    ['progress', L.col_doing],
    ['review', L.col_review],
    ['done', L.col_done],
  ];

  return (
    <div className="fixed inset-0 z-50 flex" onClick={onClose}>
      <div className="absolute inset-0" style={{ background: 'rgba(0,0,0,0.45)' }} />
      <div
        className="or-slidein ml-auto relative w-full max-w-[460px] h-full flex flex-col"
        style={{ background: 'var(--bg-elevated)', borderLeft: '1px solid var(--border-strong)' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="shrink-0 flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <span className="text-[15px] font-semibold text-ink">{tr('Détail de la mitigation', 'Mitigation detail')}</span>
          <button onClick={onClose} className="p-1 rounded-md hover:bg-hover"><X size={18} className="text-ink-muted" /></button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
          <div>
            <div className="text-[16px] font-semibold text-ink leading-snug">{miti.title}</div>
            <div className="flex items-center gap-2 mt-1.5">
              <span className="w-1.5 h-1.5 rounded-full" style={{ background: critColor[miti.crit] }} />
              <span className="mono text-[11.5px] text-ink-muted">{miti.risk}</span>
              <span className="text-[11.5px] font-semibold px-2 py-[2px] rounded-full" style={{ color: critColor[miti.crit], background: softFill(critColor[miti.crit], 15) }}>{miti.crit}</span>
            </div>
          </div>

          {/* Status control — the answer to "how does it move from À faire to En cours". */}
          <div>
            <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">{tr('Statut', 'Status')}</div>
            <div className="inline-flex rounded-[10px] p-0.5 w-full" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-secondary)' }}>
              {steps.map(([col, label]) => {
                const active = current === col;
                return (
                  <button
                    key={col}
                    disabled={setStatus.isPending}
                    onClick={() => !active && setStatus.mutate(COL_TO_STATUS[col])}
                    className="flex-1 h-9 rounded-[8px] text-[12px] font-semibold transition-colors disabled:opacity-60"
                    style={{ background: active ? 'var(--accent)' : 'transparent', color: active ? '#fff' : 'var(--text-secondary)' }}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
            <div className="text-[11.5px] text-ink-muted mt-1.5">{tr('Cliquez un statut pour déplacer le plan sur le board.', 'Click a status to move the plan across the board.')}</div>
          </div>

          <div>
            <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">{tr('Avancement', 'Progress')}</div>
            <div className="flex items-center gap-2">
              <div className="flex-1 h-1.5 rounded overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                <div className="h-full rounded" style={{ width: `${miti.progress}%`, background: miti.progress === 100 ? 'var(--low)' : 'var(--accent)' }} />
              </div>
              <span className="mono text-[11px] text-ink-muted w-9 text-right">{miti.progress}%</span>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{tr('Échéance', 'Due')}</div>
              <div className="text-[13px] inline-flex items-center gap-1.5" style={{ color: miti.overdue ? 'var(--critical)' : 'var(--text-secondary)' }}>
                {miti.overdue && <Clock size={13} />}{miti.deadline}
              </div>
            </div>
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{tr('Responsable', 'Owner')}</div>
              <Avatar initials={miti.owner} size={26} />
            </div>
          </div>

          {miti.description && (
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">Description</div>
              <p className="text-[13px] text-ink-soft leading-relaxed">{miti.description}</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function KanbanCard({ c, onOpen }: { c: UiMiti; onOpen: () => void }) {
  const [hover, setHover] = useState(false);
  return (
    <div
      onClick={onOpen}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      className="rounded-[11px] p-[13px] mb-2.5 cursor-pointer transition-all"
      style={{
        background: 'var(--bg-elevated)',
        border: `1px solid ${c.overdue ? 'rgba(255,69,58,.4)' : 'var(--border)'}`,
        boxShadow: hover ? 'var(--shadow-md)' : 'var(--shadow-sm)',
        transform: hover ? 'translateY(-2px)' : 'none',
      }}
    >
      <div className="text-[13px] font-medium text-ink mb-2.5 leading-snug">{c.title}</div>
      <div className="flex items-center gap-2 mb-[11px]">
        <span className="w-1.5 h-1.5 rounded-full" style={{ background: critColor[c.crit] }} />
        <span className="mono text-[11px] text-ink-muted">{c.risk}</span>
      </div>
      <div className="h-1 rounded overflow-hidden mb-[11px]" style={{ background: 'var(--bg-hover)' }}>
        <div className="h-full rounded" style={{ width: `${c.progress}%`, background: c.progress === 100 ? 'var(--low)' : 'var(--accent)' }} />
      </div>
      <div className="flex items-center justify-between">
        <Avatar initials={c.owner} size={24} />
        <span className="text-[11px] font-semibold inline-flex items-center gap-1" style={{ color: c.overdue ? 'var(--critical)' : 'var(--text-muted)' }}>
          {c.overdue && <Clock size={12} />}{c.deadline}
        </span>
      </div>
    </div>
  );
}

/* ---------------- table view ---------------- */
// Click-to-edit status right in the table row (ghost edit + optimistic autosave).
function InlineMitiStatus({ c, statusLabel, statusColor }: { c: UiMiti; statusLabel: Record<Column, string>; statusColor: Record<Column, string> }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const setStatus = useMutation({
    mutationFn: (col: Column) => mitigationService.setStatus(c.id, COL_TO_STATUS[col]),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['mitigations'] }); toast.success(tr('Statut mis à jour', 'Status updated')); },
    onError: () => toast.error(tr('Échec', 'Failed')),
  });
  const cols: Column[] = ['todo', 'progress', 'review', 'done'];
  const pill = (col: Column) => (
    <span className="inline-flex items-center gap-1.5 text-[12px] font-medium text-ink-soft">
      <span className="w-[7px] h-[7px] rounded-full" style={{ background: statusColor[col] }} />
      {statusLabel[col]}
    </span>
  );
  return (
    <div className="relative inline-block" onClick={(e) => e.stopPropagation()}>
      <button
        onClick={() => setOpen((o) => !o)}
        disabled={setStatus.isPending}
        title={tr('Changer le statut', 'Change status')}
        className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 -mx-1 hover:bg-hover transition-colors"
      >
        {pill(c.column)}
        {setStatus.isPending ? <Loader2 size={12} className="animate-spin text-ink-muted" /> : <ChevronDown size={12} className="text-ink-muted" />}
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} aria-hidden="true" />
          <div className="absolute left-0 top-full mt-1 z-50 min-w-[150px] rounded-[10px] p-1 shadow-card-lg" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
            {cols.map((col) => (
              <button
                key={col}
                onClick={() => { setOpen(false); if (col !== c.column) setStatus.mutate(col); }}
                className="w-full flex items-center justify-between gap-2 px-2 py-1.5 rounded-[7px] hover:bg-hover transition-colors text-left"
              >
                {pill(col)}
                {col === c.column && <Check size={13} className="text-accent" />}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function TableView({ items, isLoading, statusLabel, statusColor, onOpen }: { items: UiMiti[]; isLoading: boolean; statusLabel: Record<Column, string>; statusColor: Record<Column, string>; onOpen: (m: UiMiti) => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  if (isLoading) return <div className="flex flex-col gap-2">{[0, 1, 2, 3].map((i) => <Skeleton key={i} style={{ height: 44 }} />)}</div>;
  return (
    <div className="rounded-[14px] overflow-hidden" style={{ border: '1px solid var(--border)' }}>
      <div className="overflow-x-auto">
        <table className="w-full border-collapse" style={{ minWidth: 780 }}>
          <thead style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-secondary)' }}>
            <tr>
              {[tr('Plan', 'Plan'), tr('Risque', 'Risk'), tr('Priorité', 'Priority'), tr('Statut', 'Status'), tr('Avancement', 'Progress'), tr('Échéance', 'Due'), tr('Resp.', 'Owner')].map((t) => (
                <th key={t} className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3.5 py-2.5">{t}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.map((c) => (
              <tr key={c.id} onClick={() => onOpen(c)} style={{ borderBottom: '1px solid var(--border)', cursor: 'pointer' }} className="hover:bg-hover transition-colors">
                <td className="px-3.5 py-3 text-[13.5px] font-medium text-ink max-w-[300px] truncate">{c.title}</td>
                <td className="px-3.5 py-3"><span className="mono text-[11.5px] text-ink-muted">{c.risk}</span></td>
                <td className="px-3.5 py-3">
                  <span className="inline-flex items-center gap-1.5 text-[11.5px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: critColor[c.crit], background: softFill(critColor[c.crit], 15) }}>
                    <span className="w-1.5 h-1.5 rounded-full" style={{ background: critColor[c.crit] }} />{c.crit}
                  </span>
                </td>
                <td className="px-3.5 py-3"><InlineMitiStatus c={c} statusLabel={statusLabel} statusColor={statusColor} /></td>
                <td className="px-3.5 py-3">
                  <div className="flex items-center gap-2 min-w-[120px]">
                    <div className="flex-1 h-1.5 rounded overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                      <div className="h-full rounded" style={{ width: `${c.progress}%`, background: c.progress === 100 ? 'var(--low)' : 'var(--accent)' }} />
                    </div>
                    <span className="mono text-[11px] text-ink-muted w-8 text-right">{c.progress}%</span>
                  </div>
                </td>
                <td className="px-3.5 py-3 text-[12.5px] whitespace-nowrap" style={{ color: c.overdue ? 'var(--critical)' : 'var(--text-secondary)' }}>{c.deadline}</td>
                <td className="px-3.5 py-3"><Avatar initials={c.owner} size={24} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/* ---------------- gantt view ---------------- */
function GanttView({ items, isLoading, statusColor }: { items: UiMiti[]; isLoading: boolean; statusColor: Record<Column, string> }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const DAY = 864e5;

  const { rows, ticks, hasDates } = useMemo(() => {
    const now = Date.now();
    const dated = items
      .map((c) => {
        const due = c.dueISO ? new Date(c.dueISO).getTime() : NaN;
        let start = c.startISO ? new Date(c.startISO).getTime() : NaN;
        if (Number.isNaN(start)) start = Number.isNaN(due) ? now : Math.min(now, due - 3 * DAY);
        return { c, start, due: Number.isNaN(due) ? start + 7 * DAY : due };
      })
      .filter((r) => !Number.isNaN(r.start));
    if (dated.length === 0) return { rows: [], ticks: [] as { pct: number; label: string }[], hasDates: false };
    let min = Math.min(now, ...dated.map((r) => r.start));
    let max = Math.max(now, ...dated.map((r) => r.due));
    if (max - min < 7 * DAY) max = min + 7 * DAY;
    const pad = (max - min) * 0.04;
    min -= pad; max += pad;
    const range = max - min || 1;
    const pct = (t: number) => ((t - min) / range) * 100;
    const rows = dated
      .sort((a, b) => a.due - b.due)
      .map((r) => ({ c: r.c, left: pct(r.start), width: Math.max(2, pct(r.due) - pct(r.start)) }));
    const ticks: { pct: number; label: string }[] = [];
    for (let k = 0; k <= 4; k++) {
      const t = min + (range * k) / 4;
      ticks.push({ pct: (k / 4) * 100, label: new Date(t).toLocaleDateString(lang === 'fr' ? 'fr-FR' : 'en-US', { day: '2-digit', month: 'short' }) });
    }
    return { rows, ticks, hasDates: true };
  }, [items, lang, DAY]);

  if (isLoading) return <div className="flex flex-col gap-2">{[0, 1, 2, 3].map((i) => <Skeleton key={i} style={{ height: 40 }} />)}</div>;
  if (!hasDates) return <EmptyState icon={GanttChartSquare} title={tr('Aucune date planifiée', 'No scheduled dates')} sub={tr('Ajoutez une échéance aux plans pour les voir sur le Gantt.', 'Add due dates to plans to see them on the Gantt.')} />;

  return (
    <div className="rounded-[14px] p-4 overflow-x-auto" style={{ border: '1px solid var(--border)' }}>
      <div style={{ minWidth: 720 }}>
        {/* axis */}
        <div className="relative h-5 mb-2 ml-[220px]" style={{ borderBottom: '1px solid var(--border)' }}>
          {ticks.map((t, i) => (
            <span key={i} className="absolute text-[10.5px] text-ink-muted" style={{ left: `${t.pct}%`, transform: i === ticks.length - 1 ? 'translateX(-100%)' : 'none' }}>{t.label}</span>
          ))}
        </div>
        {rows.map(({ c, left, width }) => (
          <div key={c.id} className="flex items-center gap-3 mb-1.5">
            <div className="w-[208px] shrink-0 text-[12.5px] text-ink truncate" title={c.title}>{c.title}</div>
            <div className="relative flex-1 h-7 rounded-[8px]" style={{ background: 'var(--bg-hover)' }}>
              <div
                className="absolute top-1 bottom-1 rounded-[6px] flex items-center overflow-hidden"
                style={{ left: `${left}%`, width: `${width}%`, background: softFill(c.overdue ? 'var(--critical)' : statusColor[c.column], 26), border: `1px solid ${softFill(c.overdue ? 'var(--critical)' : statusColor[c.column], 45)}` }}
                title={`${c.title} · ${c.deadline}`}
              >
                <div className="h-full rounded-[6px]" style={{ width: `${c.progress}%`, background: c.overdue ? 'var(--critical)' : statusColor[c.column], opacity: 0.55 }} />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
