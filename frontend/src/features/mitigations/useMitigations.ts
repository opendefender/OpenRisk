// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Real /mitigations list shaped into the dc.html Kanban columns.

import { useQuery } from '@tanstack/react-query';
import { mitigationService } from '../../services/mitigationService';
import type { Mitigation } from '../../types/mitigation';
import type { Criticality } from '../../shared/riskColors';
import { initialsOf } from '../risks/riskMap';

export type Column = 'todo' | 'progress' | 'review' | 'done';

export interface UiMiti {
  id: string;
  title: string;
  risk: string;
  owner: string;
  deadline: string;
  progress: number;
  crit: Criticality;
  overdue: boolean;
  column: Column;
}

const COL: Record<string, Column> = { TODO: 'todo', IN_PROGRESS: 'progress', REVIEW: 'review', DONE: 'done' };
const CRIT: Record<string, Criticality> = { critical: 'critical', high: 'high', medium: 'medium', low: 'low' };

function fmtDate(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleDateString('fr-FR', { day: '2-digit', month: 'short' });
}

export function mapMitigation(m: Mitigation): UiMiti {
  const mm = m as Mitigation & { assignee?: string; risk_title?: string };
  const column = COL[m.status] ?? 'todo';
  const overdue = column !== 'done' && !!m.due_date && new Date(m.due_date).getTime() < Date.now();
  return {
    id: m.id,
    title: m.title,
    risk: mm.risk_title || (m.risk_id ? `#${m.risk_id.slice(0, 8)}` : '—'),
    owner: initialsOf(mm.assignee),
    deadline: fmtDate(m.due_date),
    progress: m.progress_percentage ?? 0,
    crit: CRIT[(m.priority ?? 'low').toLowerCase()] ?? 'low',
    overdue,
    column,
  };
}

export function useMitigations() {
  const query = useQuery({
    queryKey: ['mitigations', 'board'],
    queryFn: async () => {
      const res = await mitigationService.listMitigations({ page: 1, per_page: 200 });
      return (res.items ?? []).map(mapMitigation);
    },
  });
  const items = query.data ?? [];
  const columns: Record<Column, UiMiti[]> = { todo: [], progress: [], review: [], done: [] };
  for (const m of items) columns[m.column].push(m);
  return { items, columns, isLoading: query.isLoading };
}
