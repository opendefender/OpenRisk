// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Global Timeline (W1-02 §35-§36).
//
// One tenant-wide feed of what actually happened, built from the canonical audit
// trail. It answers the question no single register can: "what changed here
// today, and who changed it" — across risks, assets, findings, controls,
// incidents and evidence at once.
//
// Two properties make it more than a log viewer:
//
//   - Every row deep-links to its subject's drawer. Reading the feed and
//     investigating what you find are the same gesture (§36).
//   - It is RBAC-filtered server-side. A caller sees an event only if they may
//     read the kind of thing it is about; events about entities with no drawer
//     (automation rules, delegations) are governance surface and reach only a
//     holder of the audit permission. A page can therefore come back shorter
//     than its limit — that is the filter working, not a bug.
//
// It is not the audit trail. It carries no before/after snapshots; that is
// /governance/audit-trail, behind its own permission (§23).

import { useMemo, useState } from 'react';
import { History } from 'lucide-react';
import { Button, ErrorState, SkeletonRows } from '../../shared/ds';
import { EmptyState } from '../../shared/EmptyState';
import { useDrawerController } from './drawerState';
import { TimelineSection } from './sections/TimelineSection';
import { useUIStrings } from '../../shared/uiStrings';
import { useTenantTimeline } from './useEntityDrawer';

/** The verbs worth filtering by. Deliberately short: §22 warns against adding
 *  filter machinery nobody asked for, and these four cover the real questions
 *  ("what was created today", "what was deleted"). */
const KINDS = [
  { id: '', labelKey: 'act_all' },
  { id: 'create', labelKey: 'act_created' },
  { id: 'update', labelKey: 'act_updated' },
  { id: 'delete', labelKey: 'act_deleted' },
] as const;

export function GlobalTimelinePage() {
  const L = useUIStrings();
  const [kind, setKind] = useState('');
  const feed = useTenantTimeline(kind || undefined);
  const { open } = useDrawerController();

  const events = useMemo(() => feed.data?.pages.flatMap((p) => p.events) ?? [], [feed.data]);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5 overflow-y-auto px-5 py-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-lg font-semibold text-fg-primary">
            <History size={18} aria-hidden />
            {L.act_title}
          </h1>
          <p className="mt-1 max-w-[60ch] text-sm text-fg-secondary">{L.act_intro}</p>
        </div>
      </header>

      <div className="flex flex-wrap gap-1.5" role="group" aria-label={L.act_filterLabel}>
        {KINDS.map((k) => (
          <Button
            key={k.id || 'all'}
            variant={kind === k.id ? 'secondary' : 'ghost'}
            size="sm"
            aria-pressed={kind === k.id}
            onClick={() => setKind(k.id)}
          >
            {L[k.labelKey]}
          </Button>
        ))}
      </div>

      {feed.isError ? (
        <ErrorState
          title={L.act_loadFailed}
          description={L.ed_retryDesc}
          onRetry={() => void feed.refetch()}
        />
      ) : feed.isLoading ? (
        <SkeletonRows rows={8} />
      ) : events.length === 0 ? (
        <EmptyState
          variant={kind ? 'no-results' : 'first-use'}
          title={kind ? L.act_emptyFiltered : L.act_emptyNone}
          description={kind ? L.act_emptyFilteredDesc : L.act_emptyNoneDesc}
          primaryAction={
            kind ? (
              <Button variant="secondary" size="sm" onClick={() => setKind('')}>
                {L.act_clearFilter}
              </Button>
            ) : undefined
          }
        />
      ) : (
        <TimelineSection
          events={events}
          isLoading={false}
          error={null}
          hasMore={!!feed.hasNextPage}
          isLoadingMore={feed.isFetchingNextPage}
          onLoadMore={() => void feed.fetchNextPage()}
          onRetry={() => void feed.refetch()}
          // This is what makes the feed navigable: an event's subject is not the
          // record you are already looking at, so each row can open its own.
          onOpenTarget={open}
        />
      )}
    </div>
  );
}

export default GlobalTimelinePage;
