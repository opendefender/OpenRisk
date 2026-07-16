// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Mitigations (OpenRisk.dc.html §6.4) — wired to the real /mitigations list, with
// three selectable views: Kanban (To do → In progress → In review → Done), a dense
// Table, and a Gantt timeline positioned by real start/due dates. Loading skeleton
// + empty state on all three.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Clock, ShieldCheck, KanbanSquare, Rows3, GanttChartSquare } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Avatar, Skeleton, EmptyState, softFill } from '../../shared/ui';
import { critColor } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useMitigations, type Column, type UiMiti } from './useMitigations';
import { useMitigationEvents } from './useMitigationEvents';

type View = 'kanban' | 'table' | 'gantt';

export function MitigationsBoard() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { columns, items, isLoading } = useMitigations();
  const [view, setView] = useState<View>('kanban');

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
                  columns[key].map((c) => <KanbanCard key={c.id} c={c} />)
                )}
              </div>
            ))}
          </div>
        </div>
      ) : view === 'table' ? (
        <TableView items={items} isLoading={isLoading} statusLabel={statusLabel} statusColor={statusColor} />
      ) : (
        <GanttView items={items} isLoading={isLoading} statusColor={statusColor} />
      )}
    </PageFrame>
  );
}

function KanbanCard({ c }: { c: UiMiti }) {
  const [hover, setHover] = useState(false);
  return (
    <div
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
function TableView({ items, isLoading, statusLabel, statusColor }: { items: UiMiti[]; isLoading: boolean; statusLabel: Record<Column, string>; statusColor: Record<Column, string> }) {
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
              <tr key={c.id} style={{ borderBottom: '1px solid var(--border)' }} className="hover:bg-hover transition-colors">
                <td className="px-3.5 py-3 text-[13.5px] font-medium text-ink max-w-[300px] truncate">{c.title}</td>
                <td className="px-3.5 py-3"><span className="mono text-[11.5px] text-ink-muted">{c.risk}</span></td>
                <td className="px-3.5 py-3">
                  <span className="inline-flex items-center gap-1.5 text-[11.5px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: critColor[c.crit], background: softFill(critColor[c.crit], 15) }}>
                    <span className="w-1.5 h-1.5 rounded-full" style={{ background: critColor[c.crit] }} />{c.crit}
                  </span>
                </td>
                <td className="px-3.5 py-3"><span className="inline-flex items-center gap-1.5 text-[12px] font-medium text-ink-soft"><span className="w-[7px] h-[7px] rounded-full" style={{ background: statusColor[c.column] }} />{statusLabel[c.column]}</span></td>
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
