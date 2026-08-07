// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Mitigation plan detail — /risks/mitigations/:mitigationId.
//
// This was a drawer held in MitigationsBoard's `selected` state: unaddressable,
// invisible to the Back button, and closable only via its own X. As a route it
// is linkable (from a risk, from a notification, from a report) and Back works.
//
// It lives under /risks because a mitigation exists only to reduce a risk, so
// that is the unambiguous parent to return to.

import { useMemo } from 'react';
import { useParams, Link } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Clock } from 'lucide-react';
import { toast } from 'sonner';
import { DetailPage } from '../../shared/DetailPage';
import { Card, Avatar } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { critColor, softFill } from '../../shared/riskColors';
import { mitigationService } from '../../services/mitigationService';
import { useMitigations, type Column } from './useMitigations';
import type { BoardStatus } from '../../services/mitigationService';

const COL_TO_STATUS: Record<Column, BoardStatus> = {
  todo: 'PLANNED', progress: 'IN_PROGRESS', review: 'REVIEW', done: 'DONE',
};
const STATUS_TO_COL: Record<string, Column> = {
  PLANNED: 'todo', TODO: 'todo', IN_PROGRESS: 'progress', REVIEW: 'review', DONE: 'done',
};

export function MitigationDetailPage() {
  const { mitigationId } = useParams<{ mitigationId: string }>();
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const qc = useQueryClient();

  const { items, isLoading } = useMitigations();
  const miti = useMemo(() => items.find((m) => m.id === mitigationId), [items, mitigationId]);

  const setStatus = useMutation({
    mutationFn: (status: BoardStatus) => mitigationService.setStatus(mitigationId as string, status),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['mitigations'] });
      toast.success(tr('Statut mis à jour', 'Status updated'));
    },
    onError: () => toast.error(tr('Échec de la mise à jour', 'Update failed')),
  });

  const current = miti ? (STATUS_TO_COL[miti.rawStatus] ?? miti.column) : 'todo';
  const steps: [Column, string][] = [
    ['todo', L.col_todo],
    ['progress', L.col_doing],
    ['review', L.col_review],
    ['done', L.col_done],
  ];

  return (
    <DetailPage
      title={miti?.title}
      backLabel={L.n_mitigations}
      loading={isLoading}
      notFound={!isLoading && !miti}
      subtitle={
        miti ? (
          <>
            <span className="w-1.5 h-1.5 rounded-full" style={{ background: critColor[miti.crit] }} />
            <span className="mono text-[11.5px] text-ink-muted">{miti.risk}</span>
            <span
              className="text-[11.5px] font-semibold px-2 py-[2px] rounded-full"
              style={{ color: critColor[miti.crit], background: softFill(critColor[miti.crit], 15) }}
            >
              {miti.crit}
            </span>
          </>
        ) : null
      }
    >
      {miti && (
        <Card style={{ padding: '18px 20px' }}>
          {/* Status control — how a plan moves across the board. */}
          <div className="mb-6">
            <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">
              {tr('Statut', 'Status')}
            </div>
            <div
              className="inline-flex rounded-[10px] p-0.5 w-full"
              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-secondary)' }}
            >
              {steps.map(([col, label]) => {
                const active = current === col;
                return (
                  <button
                    key={col}
                    disabled={setStatus.isPending}
                    onClick={() => !active && setStatus.mutate(COL_TO_STATUS[col])}
                    className="flex-1 h-9 rounded-[8px] text-[12px] font-semibold transition-colors disabled:opacity-60"
                    style={{ background: active ? 'var(--accent)' : 'transparent', color: active ? 'var(--text-on-solid)' : 'var(--text-secondary)' }}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="mb-6">
            <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">
              {tr('Avancement', 'Progress')}
            </div>
            <div className="flex items-center gap-2">
              <div className="flex-1 h-1.5 rounded overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                <div
                  className="h-full rounded"
                  style={{ width: `${miti.progress}%`, background: miti.progress === 100 ? 'var(--low)' : 'var(--accent)' }}
                />
              </div>
              <span className="mono text-[11px] text-ink-muted w-9 text-right">{miti.progress}%</span>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4 mb-6">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">
                {tr('Échéance', 'Due')}
              </div>
              <div
                className="text-[13px] inline-flex items-center gap-1.5"
                style={{ color: miti.overdue ? 'var(--critical)' : 'var(--text-secondary)' }}
              >
                {miti.overdue && <Clock size={13} />}
                {miti.deadline}
              </div>
            </div>
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">
                {tr('Responsable', 'Owner')}
              </div>
              <Avatar initials={miti.owner} size={26} />
            </div>
          </div>

          {miti.description && (
            <div className="mb-6">
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">Description</div>
              <p className="text-[13px] text-ink-soft leading-relaxed">{miti.description}</p>
            </div>
          )}

          <Link to="/risks" className="text-[12.5px] font-semibold text-accent hover:underline">
            {tr('Voir le registre des risques', 'View the risk register')}
          </Link>
        </Card>
      )}
    </DetailPage>
  );
}
