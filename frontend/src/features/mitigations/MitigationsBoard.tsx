// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Mitigations Kanban (OpenRisk.dc.html §6.4) — wired to the real /mitigations list,
// grouped into To do → In progress → In review → Done. Loading skeleton + empty state.

import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Clock, ShieldCheck } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Avatar, Skeleton, EmptyState } from '../../shared/ui';
import { critColor } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useMitigations, type Column, type UiMiti } from './useMitigations';

export function MitigationsBoard() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const { columns, items, isLoading } = useMitigations();

  const cols: [Column, string, string][] = [
    ['todo', L.col_todo, 'var(--text-muted)'],
    ['progress', L.col_doing, 'var(--high)'],
    ['review', L.col_review, 'var(--info)'],
    ['done', L.col_done, 'var(--low)'],
  ];

  return (
    <PageFrame wide>
      <PageHeader title={L.mitiTitle} actions={<Btn label={L.addPlan} icon={Plus} primary onClick={() => navigate('/risks')} />} />

      {!isLoading && items.length === 0 ? (
        <EmptyState
          icon={ShieldCheck}
          title={lang === 'fr' ? 'Aucune mitigation' : 'No mitigations yet'}
          sub={lang === 'fr' ? 'Créez une mitigation depuis un risque pour lancer son traitement.' : 'Create a mitigation from a risk to start treating it.'}
          cta={<Btn label={L.n_risks} onClick={() => navigate('/risks')} primary />}
        />
      ) : (
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
