// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Action Center of the October 2026 redesign (#902).
//
// One column at 1000 px: a sentence that says how much is waiting and how much
// of it is late, filter chips by kind with their counts, then the work grouped
// by deadline — overdue, this week, later, no date. Within a group the order
// is the server's (category rank, then each category's own key); the client
// only buckets by date and never re-sorts.
//
// The page reads up to 100 items, the API's maximum page, so the chips and
// groups count the whole list. Past 100 the pager takes over.

import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useQueryClient } from '@tanstack/react-query';
import { ChevronLeft, ChevronRight, CircleCheck } from 'lucide-react';

import { useI18n, interpolate } from '../../hooks/useI18n';
import { ErrorState, PageHeader, Skeleton } from '../../shared/ui';
import { useSoftDelete } from '../../shared/useSoftDelete';
import { useActionItems, PAGE_LIMIT as PAGE_SIZE } from './useActionItems';
import { linkableItems, type LinkableActionItem } from './actionLinks';
import { pageFromParam } from './paging';
import { ActionItemRow } from './ActionItemRow';
import { FILTER_ORDER, completionFor, filterOf, type ActionFilter } from './actionKinds';
import { dueGroup, type DueGroup } from './actionFacts';

const GROUPS: DueGroup[] = ['late', 'week', 'later', 'none'];

export function ActionCenterPage() {
  const { t } = useI18n();
  const qc = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [filter, setFilter] = useState<ActionFilter>('all');

  const page = pageFromParam(params.get('page'));
  const offset = (page - 1) * PAGE_SIZE;
  const { items, total, isLoading, isError, refetch } = useActionItems({ limit: PAGE_SIZE, offset });
  const linkable = useMemo(() => linkableItems(items), [items]);

  const done = useSoftDelete<LinkableActionItem>({
    idOf: (row) => row.item.id,
    message: (row) => interpolate(t('actionCenter.doneToast'), { title: row.item.title }),
    failureMessage: () => t('actionCenter.doneFailed'),
    onCommit: async (id) => {
      const row = linkable.find((r) => r.item.id === id);
      const complete = row ? completionFor(row.item) : undefined;
      if (!complete) return;
      await complete();
      // Wait for the lists to re-read before the row is un-hidden, so it does
      // not flash back between the call and the refetch.
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['action-center'] }),
        qc.invalidateQueries({ queryKey: ['nav-counts'] }),
      ]);
    },
  });

  const rows = useMemo(
    () => linkable.filter((r) => !done.pending.has(r.item.id)),
    [linkable, done.pending],
  );

  const counts = useMemo(() => {
    const c: Record<ActionFilter, number> = {
      all: rows.length,
      approval: 0,
      vuln: 0,
      plans: 0,
      incident: 0,
      compliance: 0,
      vendor: 0,
    };
    for (const r of rows) {
      const f = filterOf(r.item);
      if (f) c[f] += 1;
    }
    return c;
  }, [rows]);

  const shown = filter === 'all' ? rows : rows.filter((r) => filterOf(r.item) === filter);
  const grouped = GROUPS.map((g) => ({ g, rows: shown.filter((r) => dueGroup(r.item) === g) })).filter(
    (x) => x.rows.length > 0,
  );
  const late = rows.filter((r) => dueGroup(r.item) === 'late').length;

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const goTo = (next: number) => {
    const clamped = Math.min(Math.max(1, next), pageCount);
    const updated = new URLSearchParams(params);
    if (clamped === 1) updated.delete('page');
    else updated.set('page', String(clamped));
    setParams(updated);
  };
  const ready = !isLoading && !isError;
  const nothingAtAll = rows.length === 0 && page === 1;

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="mx-auto max-w-[1064px] px-5 sm:px-8 pt-6 pb-16 motion-safe:animate-or-fadeup">
        <PageHeader
          title={t('actionCenter.title')}
          subtitle={
            ready ? (
              <span data-testid="action-center-summary">
                {t('actionCenter.summary', { count: total })}
                {late > 0 && ` · ${t('actionCenter.summaryLate', { count: late })}`}
              </span>
            ) : (
              <span className="or-skeleton inline-block h-3 w-48 rounded" aria-hidden="true" />
            )
          }
        />

        {ready && rows.length > 0 && (
          <div
            className="flex gap-1.5 flex-wrap mb-[18px]"
            role="group"
            aria-label={t('actionCenter.filters.label')}
          >
            {FILTER_ORDER.filter((f) => f !== 'incident' || counts.incident > 0).map((f) => {
              const on = f === filter;
              return (
                <button
                  key={f}
                  type="button"
                  aria-pressed={on}
                  onClick={() => setFilter(f)}
                  data-testid={`action-filter-${f}`}
                  className={`h-[30px] px-3 rounded-full border text-[12.5px] font-semibold flex items-center gap-1.5 transition-colors hover:border-border-strong ${
                    on
                      ? 'bg-surface-3 border-border-strong text-ink'
                      : 'bg-transparent border-border-default text-ink-soft'
                  }`}
                >
                  {t(`actionCenter.filters.${f}`)}
                  <span className="mono text-[11px] text-ink-muted">{counts[f]}</span>
                </button>
              );
            })}
          </div>
        )}

        {isLoading && (
          <div data-testid="action-center-skeleton" aria-busy="true" aria-live="polite">
            <span className="sr-only">{t('actionCenter.loading')}</span>
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="mb-1.5 h-[62px] w-full rounded-[12px]" />
            ))}
          </div>
        )}

        {!isLoading && isError && (
          <div data-testid="action-center-error">
            <ErrorState
              title={t('actionCenter.errorTitle')}
              description={t('actionCenter.errorDescription')}
              onRetry={refetch}
              retryLabel={t('actionCenter.retry')}
            />
          </div>
        )}

        {ready && grouped.length === 0 && (
          <div
            data-testid="action-center-empty"
            className="px-6 py-14 text-center border border-dashed border-border-default rounded-[14px]"
          >
            <CircleCheck size={28} className="mx-auto text-success-text" aria-hidden="true" />
            <div className="text-[15px] font-semibold text-ink mt-2.5">
              {page > 1
                ? t('actionCenter.emptyPageTitle')
                : nothingAtAll
                  ? t('actionCenter.emptyTitle')
                  : t('actionCenter.emptyFilterTitle')}
            </div>
            <div className="text-[13px] text-ink-muted mt-1">
              {page > 1
                ? t('actionCenter.emptyPageDescription')
                : nothingAtAll
                  ? t('actionCenter.emptyDescription')
                  : t('actionCenter.emptyFilterBody')}
            </div>
          </div>
        )}

        {ready &&
          grouped.map(({ g, rows: list }) => (
            <section key={g} className="mb-[22px]" data-testid={`action-group-${g}`}>
              <div className="flex items-center gap-2 mb-2">
                <h2
                  className="m-0 text-[10.5px] tracking-[0.06em] uppercase font-semibold"
                  style={{ color: g === 'late' ? 'var(--danger-text)' : 'var(--fg-muted)' }}
                >
                  {t(`actionCenter.groups.${g}`)}
                </h2>
                <span className="mono text-[11px] text-ink-muted">{list.length}</span>
              </div>
              <ul
                className="m-0 p-0 list-none bg-surface-1 border border-border-subtle rounded-[14px] overflow-hidden"
                data-testid="action-center-list"
              >
                {list.map((row) => (
                  <ActionItemRow
                    key={row.item.id}
                    item={row.item}
                    href={row.href}
                    onDone={completionFor(row.item) ? () => done.remove(row) : undefined}
                  />
                ))}
              </ul>
            </section>
          ))}

        {ready && (total > PAGE_SIZE || page > 1) && (
          <div className="mt-3 flex items-center justify-end gap-1">
            <button
              type="button"
              onClick={() => goTo(page - 1)}
              disabled={page <= 1}
              aria-label={t('actionCenter.previousPage')}
              data-testid="action-center-prev"
              className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border-default text-fg-secondary hover:bg-surface-3 disabled:pointer-events-none disabled:opacity-40"
            >
              <ChevronLeft size={15} aria-hidden="true" />
            </button>
            <span className="px-1.5 text-xs text-fg-primary" data-testid="action-center-page-indicator">
              {interpolate(t('actionCenter.pageOf'), { page, pageCount })}
            </span>
            <button
              type="button"
              onClick={() => goTo(page + 1)}
              disabled={page >= pageCount}
              aria-label={t('actionCenter.nextPage')}
              data-testid="action-center-next"
              className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border-default text-fg-secondary hover:bg-surface-3 disabled:pointer-events-none disabled:opacity-40"
            >
              <ChevronRight size={15} aria-hidden="true" />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

export default ActionCenterPage;
