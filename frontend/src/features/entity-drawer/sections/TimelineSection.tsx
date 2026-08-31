// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The entity timeline (W1-02 §17-§22).
//
// Newest first, cursor-paginated, and made of real events: the canonical audit
// trail plus the domain journals it cannot see (a score the worker recomputed
// outside any request, an incident's own log). Nothing here is derived from a
// timestamp on the record — "updated 2 hours ago" rendered from `updated_at` is
// exactly the fabricated history §17 forbids.

import { Clock, Plus, Pencil, ShieldCheck, Trash2, MessageSquare, UserCheck } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Button, ErrorState, SkeletonRows } from '../../../shared/ds';
import { EmptyState } from '../../../shared/EmptyState';
import { formatDate, shortId } from './SummarySection';
import type { EntityType, TimelineEvent, TimelineSourceName } from '../types';

const KIND_ICON: Record<string, { icon: LucideIcon; tone: string }> = {
  create: { icon: Plus, tone: 'var(--low)' },
  update: { icon: Pencil, tone: 'var(--text-soft)' },
  delete: { icon: Trash2, tone: 'var(--critical)' },
  mitigate: { icon: ShieldCheck, tone: 'var(--accent)' },
  approve: { icon: ShieldCheck, tone: 'var(--low)' },
  reject: { icon: Trash2, tone: 'var(--critical)' },
  comment: { icon: MessageSquare, tone: 'var(--text-soft)' },
  assign: { icon: UserCheck, tone: 'var(--accent)' },
};

function iconFor(kind: string) {
  return KIND_ICON[kind] ?? { icon: Clock, tone: 'var(--fg-muted)' };
}

/** Where an event came from, said plainly. A reader who sees a score change with
 *  no actor should be able to tell it was a worker, not a missing name. */
const SOURCE_LABEL: Record<TimelineSourceName, string> = {
  audit: 'Audit trail',
  risk_history: 'Score engine',
  incident_timeline: 'Incident log',
  asset_snapshot: 'Asset history',
};

interface Props {
  events: TimelineEvent[];
  isLoading: boolean;
  error: unknown;
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore: () => void;
  onRetry: () => void;
  /** Present on the tenant-wide feed, where an event's subject is not the
   *  entity you are already looking at. */
  onOpenTarget?: (type: EntityType, id: string) => void;
}

export function TimelineSection({
  events,
  isLoading,
  error,
  hasMore,
  isLoadingMore,
  onLoadMore,
  onRetry,
  onOpenTarget,
}: Props) {
  if (isLoading) return <SkeletonRows rows={5} />;

  if (error) {
    return (
      <ErrorState
        title="History could not be loaded"
        description="The rest of this record is still readable."
        onRetry={onRetry}
      />
    );
  }

  if (events.length === 0) {
    return (
      <EmptyState
        variant="first-use"
        title="No activity recorded"
        description="Changes to this record will appear here, with who made them and when."
      />
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <ol className="relative flex flex-col gap-0" aria-label="Activity, newest first">
        {events.map((ev, i) => {
          const { icon: Icon, tone } = iconFor(ev.kind);
          const last = i === events.length - 1;
          return (
            <li key={ev.id} className="relative flex gap-3 pb-4 last:pb-0">
              {/* The rail is decorative; the list semantics carry the order. */}
              {!last && (
                <span
                  aria-hidden
                  className="absolute left-[11px] top-6 bottom-0 w-px"
                  style={{ background: 'var(--border-subtle)' }}
                />
              )}
              <span
                aria-hidden
                className="relative z-1 mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-subtle bg-surface-2"
                style={{ color: tone }}
              >
                <Icon size={12} />
              </span>
              <div className="min-w-0 flex-1">
                <p className="text-sm text-fg-primary">{ev.summary}</p>
                <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-2xs text-fg-muted">
                  <time dateTime={ev.occurred_at} className="font-mono tabular-nums">
                    {formatDate(ev.occurred_at)}
                  </time>
                  <span aria-hidden>·</span>
                  <span>
                    {ev.actor
                      ? ev.actor.email || ev.actor.label || shortId(ev.actor.id)
                      : 'System'}
                  </span>
                  <span aria-hidden>·</span>
                  <span>{SOURCE_LABEL[ev.source] ?? ev.source}</span>
                </p>
                {ev.changes && ev.changes.length > 0 && (
                  <p className="mt-1 flex flex-wrap gap-1">
                    {ev.changes.map((c) => (
                      <span
                        key={c.field}
                        className="rounded-xs bg-surface-3 px-1.5 py-0.5 font-mono text-2xs text-fg-secondary"
                      >
                        {c.field}
                        {c.from || c.to ? `: ${c.from || '—'} → ${c.to || '—'}` : ''}
                      </span>
                    ))}
                  </p>
                )}
                {onOpenTarget && ev.target.type && (
                  <button
                    type="button"
                    onClick={() => onOpenTarget(ev.target.type as EntityType, ev.target.id)}
                    className="mt-1 text-2xs text-accent underline underline-offset-2 hover:opacity-80"
                  >
                    Open {ev.target.type}
                  </button>
                )}
              </div>
            </li>
          );
        })}
      </ol>

      {hasMore && (
        <Button variant="secondary" size="sm" onClick={onLoadMore} loading={isLoadingMore}>
          Load more
        </Button>
      )}
    </div>
  );
}
