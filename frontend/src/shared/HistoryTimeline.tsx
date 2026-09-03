// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Generalized time-travel timeline (UX-25): a dated + attributed history of an
// entity (who changed what, and when). Feed it normalized HistoryEntry[] from any
// source (risk timeline, asset snapshots, control audit events…) so every major
// entity gets the same "time travel" affordance. Presentational + theme-aware.

import { type ReactNode } from 'react';
import { Plus, Pencil, ShieldCheck, Trash2, Clock, User, Dot } from 'lucide-react';

export interface HistoryField {
  label: string;
  value: string;
}
export interface HistoryEntry {
  id: string;
  /** Semantic kind driving the dot colour + icon: create | update | mitigate | delete | … */
  kind?: string;
  title: string;
  detail?: string;
  /** Who made the change (resolved name/email, a short id, or a "System" label). */
  actor: string;
  /** ISO timestamp of the change. */
  at: string;
  /** Optional snapshot fields shown as compact chips (e.g. Score / Status). */
  fields?: HistoryField[];
}

interface HistoryTimelineProps {
  entries: HistoryEntry[];
  isLoading?: boolean;
  error?: boolean;
  emptyLabel?: string;
  errorLabel?: string;
  /** Format the ISO timestamp for display; defaults to the browser locale. */
  formatDate?: (iso: string) => string;
}

const KIND_META: Record<string, { color: string; Icon: typeof Plus }> = {
  create: { color: 'var(--low)', Icon: Plus },
  mitigate: { color: 'var(--accent-500)', Icon: ShieldCheck },
  update: { color: 'var(--text-soft)', Icon: Pencil },
  delete: { color: 'var(--critical)', Icon: Trash2 },
};
function metaFor(kind?: string) {
  return (kind && KIND_META[kind]) || { color: 'var(--fg-muted)', Icon: Dot };
}

export function HistoryTimeline({
  entries,
  isLoading,
  error,
  emptyLabel,
  errorLabel,
  formatDate,
}: HistoryTimelineProps): ReactNode {
  const fmt = formatDate ?? ((iso: string) => new Date(iso).toLocaleString());

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3">
        {[0, 1, 2].map((i) => (
          <div key={i} className="or-skeleton h-16 rounded-[12px]" />
        ))}
      </div>
    );
  }
  if (error) {
    return (
      <div className="text-center py-8 text-[13px]" style={{ color: 'var(--critical)' }}>
        {errorLabel ?? 'Failed to load history.'}
      </div>
    );
  }
  if (!entries.length) {
    return (
      <div className="flex flex-col items-center justify-center text-center py-10 gap-2">
        <div
          className="w-10 h-10 rounded-full flex items-center justify-center"
          style={{ background: 'var(--bg-hover)', color: 'var(--fg-muted)' }}
        >
          <Clock size={20} />
        </div>
        <div className="text-[13px] text-ink-soft">{emptyLabel ?? 'No changes recorded yet.'}</div>
      </div>
    );
  }

  return (
    <div className="flex flex-col">
      {entries.map((e, i) => {
        const { color, Icon } = metaFor(e.kind);
        const last = i === entries.length - 1;
        return (
          <div
            key={e.id}
            className="flex gap-3"
            style={{
              animation: 'or-fadeup .3s ease both',
              animationDelay: `${Math.min(i * 0.03, 0.24)}s`,
            }}
          >
            {/* rail: dot + connecting line */}
            <div className="flex flex-col items-center shrink-0">
              <div
                className="w-7 h-7 rounded-full flex items-center justify-center"
                style={{ background: `color-mix(in srgb,${color} 16%,transparent)`, color }}
              >
                <Icon size={14} />
              </div>
              {!last && (
                <div className="w-px flex-1 my-1" style={{ background: 'var(--border)' }} />
              )}
            </div>
            {/* content */}
            <div className={`flex-1 min-w-0 ${last ? '' : 'pb-4'}`}>
              <div className="text-[13.5px] font-semibold text-ink">{e.title}</div>
              {e.detail && <div className="text-[12.5px] text-ink-soft mt-0.5">{e.detail}</div>}
              {e.fields && e.fields.length > 0 && (
                <div className="flex flex-wrap gap-1.5 mt-2">
                  {e.fields.map((f) => (
                    <span
                      key={f.label}
                      className="text-[11px] px-2 py-1 rounded-[7px]"
                      style={{ background: 'var(--bg-hover)', color: 'var(--text-soft)' }}
                    >
                      <span className="text-ink-muted">{f.label}</span>{' '}
                      <span className="mono text-ink">{f.value}</span>
                    </span>
                  ))}
                </div>
              )}
              <div className="flex items-center gap-3 mt-1.5 text-[11.5px] text-ink-muted">
                <span className="inline-flex items-center gap-1">
                  <User size={12} /> {e.actor}
                </span>
                <span className="inline-flex items-center gap-1">
                  <Clock size={12} /> {fmt(e.at)}
                </span>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
