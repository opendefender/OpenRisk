// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Action Center panel (#430).
//
// One list answering one question: what is mine to act on right now, in order,
// and how do I get straight to it. Every row is a real anchor to a real route —
// there is no row here that does not go anywhere (CLAUDE.md rule 12).
//
// This is NOT the notification bell. The bell is chronological, "what happened",
// and is read then dismissed. This is a work queue: an item appears because a
// record is outstanding and disappears when that work is done. There is no
// dismiss and no read state, on purpose — see the epic's non-goals.
//
// Two things it deliberately does not do:
//
//   - it does not sort. The server has already applied category rank, then each
//     category's own secondary key, then id, and it pages on that same order.
//     Re-sorting here would agree with the server on page one and contradict it
//     on page two.
//   - it does not invent an empty state's meaning. An empty Action Center is the
//     GOOD outcome — nothing is outstanding — so it reads as an all-clear rather
//     than as "no data found".

import { useMemo } from 'react';
import { Link } from 'react-router';
import { CircleCheck } from 'lucide-react';

import { useI18n, interpolate } from '../../hooks/useI18n';
import { EmptyState, ErrorState, Skeleton } from '../../shared/ui';
import { useActionItems, PANEL_LIMIT } from './useActionItems';
import { linkableItems } from './actionLinks';
import { ActionItemRow } from './ActionItemRow';

const SKELETON_ROWS = 4;

export function ActionCenterPanel({ limit = PANEL_LIMIT }: { limit?: number }) {
  const { t } = useI18n();
  const { items, total, isLoading, isError, refetch } = useActionItems({ limit });

  // Filtering, not sorting: an item whose deep_link does not resolve to a real
  // route is dropped (and logged) rather than rendered as a dead row. Order is
  // the server's throughout.
  const rows = useMemo(() => linkableItems(items), [items]);

  return (
    <section
      aria-labelledby="action-center-title"
      className="or-card mb-4 p-4"
      data-testid="action-center"
    >
      <div className="mb-3 flex items-baseline justify-between gap-3">
        <div>
          <h2 id="action-center-title" className="text-md font-semibold text-fg-primary">
            {t('actionCenter.title')}
          </h2>
          <p className="mt-0.5 text-xs text-fg-muted">{t('actionCenter.subtitle')}</p>
        </div>
        {!isLoading && !isError && total > 0 && (
          <span className="shrink-0 text-xs text-fg-muted" data-testid="action-center-count">
            {interpolate(t('actionCenter.showing'), { shown: rows.length, total })}
          </span>
        )}
      </div>

      {isLoading && (
        <div data-testid="action-center-skeleton" aria-busy="true" aria-live="polite">
          <span className="sr-only">{t('actionCenter.loading')}</span>
          {Array.from({ length: SKELETON_ROWS }).map((_, index) => (
            <Skeleton key={index} className="mb-1.5 h-11 w-full" />
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

      {!isLoading && !isError && rows.length === 0 && (
        <div data-testid="action-center-empty">
          <EmptyState
            variant="first-use"
            icon={CircleCheck}
            title={t('actionCenter.emptyTitle')}
            description={t('actionCenter.emptyDescription')}
          />
        </div>
      )}

      {!isLoading && !isError && rows.length > 0 && (
        <>
          <ul className="m-0 list-none p-0" data-testid="action-center-list">
            {rows.map(({ item, href }) => (
              <ActionItemRow key={item.id} item={item} href={href} />
            ))}
          </ul>
          {total > rows.length && (
            <div className="mt-2 px-3">
              {/* #433 — this was a dead line of text saying how much was not on
                  screen, because the page it wanted to link to did not exist
                  yet. It does now, so the count is a real link rather than a
                  statement the user cannot act on. */}
              <Link
                to="/action-center"
                data-testid="action-center-view-all"
                className="text-xs text-accent-strong no-underline hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
              >
                {interpolate(t('actionCenter.viewAll'), { count: total - rows.length })}
              </Link>
            </div>
          )}
        </>
      )}
    </section>
  );
}

export default ActionCenterPanel;
