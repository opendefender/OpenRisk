// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The full Action Center, at /action-center (#433).
//
// The dashboard panel shows the top of the queue. This is the whole of it. It
// exists because the panel could show eight rows and then tell a user with
// forty outstanding items that thirty-two more were waiting somewhere they
// could not go — a true statement the user was unable to act on.
//
// THE PAGE LIVES IN THE URL. `?page=3` is a real, shareable, back-button-able
// address, not component state. That is the route tree's own rule (see the
// header of shared/routeModel.ts: "every sub-view is a route, never component
// state"), and it is why the pager reads and writes search params rather than
// useState.
//
// It does NOT use DataTable, deliberately. DataTable sorts, filters and pages
// over rows it holds in memory; this list is ordered by the server, paged by the
// server, and must not be re-sorted at any point (#430 AC1, #433 AC4). Handing
// it to a component whose job is to reorder rows would be arguing with the
// server about the one thing the server is authoritative on.

import { useMemo } from 'react';
import { useSearchParams } from 'react-router';
import { ChevronLeft, ChevronRight, CircleCheck } from 'lucide-react';

import { useI18n, interpolate } from '../../hooks/useI18n';
import { EmptyState, ErrorState, PageFrame, PageHeader, Skeleton } from '../../shared/ui';
import { useActionItems, PAGE_LIMIT } from './useActionItems';
import { linkableItems } from './actionLinks';
import { pageFromParam } from './paging';
import { ActionItemRow } from './ActionItemRow';

const SKELETON_ROWS = 8;

export function ActionCenterPage() {
  const { t } = useI18n();
  const [params, setParams] = useSearchParams();

  const page = pageFromParam(params.get('page'));
  const offset = (page - 1) * PAGE_LIMIT;

  const { items, total, isLoading, isError, refetch } = useActionItems({ limit: PAGE_LIMIT, offset });

  // Filtering, never sorting: an item whose deep_link does not resolve is
  // dropped and logged rather than rendered as a row that goes nowhere. Order
  // is the server's, on this page as on every other.
  const rows = useMemo(() => linkableItems(items), [items]);

  const pageCount = Math.max(1, Math.ceil(total / PAGE_LIMIT));
  const firstOnPage = total === 0 ? 0 : offset + 1;
  const lastOnPage = Math.min(offset + rows.length, total);

  const goTo = (next: number) => {
    const clamped = Math.min(Math.max(1, next), pageCount);
    const updated = new URLSearchParams(params);
    // Page 1 is the bare URL. Carrying ?page=1 around makes two addresses for
    // one view, which is the sort of thing that ends up in a shared link.
    if (clamped === 1) updated.delete('page');
    else updated.set('page', String(clamped));
    setParams(updated);
  };

  // The pager stays mounted while a page change is in flight. Gating it on
  // `!isLoading` made the control disappear under the cursor on every click:
  // moving to page 2 is a new query key, so React Query reports isLoading again
  // and the whole pager unmounted mid-interaction. It stays put and goes
  // disabled instead — and while the count is unknown it renders a dash rather
  // than a number it does not have, because "2/1" would be worse than "2/—".
  const countKnown = !isLoading && !isError;
  const showPager = !isError && (total > PAGE_LIMIT || page > 1);

  return (
    <PageFrame>
      <PageHeader
        title={t('actionCenter.title')}
        count={!isLoading && !isError && total > 0 ? String(total) : null}
      />

      <p className="mb-4 text-xs text-text-muted">{t('actionCenter.subtitle')}</p>

      <div className="or-card p-4" data-testid="action-center-page">
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
              title={
                // Page 1 empty and page 7 empty are different facts with
                // different remedies: one means nothing is outstanding, the
                // other means this page number is past the end of the list.
                page > 1 ? t('actionCenter.emptyPageTitle') : t('actionCenter.emptyTitle')
              }
              description={
                page > 1 ? t('actionCenter.emptyPageDescription') : t('actionCenter.emptyDescription')
              }
            />
          </div>
        )}

        {!isLoading && !isError && rows.length > 0 && (
          <ul className="m-0 list-none p-0" data-testid="action-center-list">
            {rows.map(({ item, href }) => (
              <ActionItemRow key={item.id} item={item} href={href} />
            ))}
          </ul>
        )}
      </div>

      {showPager && (
        <div className="mt-3 flex flex-wrap items-center gap-3">
          <span className="text-xs text-text-muted" data-testid="action-center-range" aria-live="polite">
            {countKnown ? interpolate(t('actionCenter.range'), { from: firstOnPage, to: lastOnPage, total }) : ''}
          </span>
          <div className="flex-1" />
          <div className="inline-flex items-center gap-1">
            <button
              type="button"
              onClick={() => goTo(page - 1)}
              disabled={!countKnown || page <= 1}
              aria-label={t('actionCenter.previousPage')}
              data-testid="action-center-prev"
              className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border-default text-text-secondary hover:bg-hover disabled:pointer-events-none disabled:opacity-40"
            >
              <ChevronLeft size={15} aria-hidden="true" />
            </button>
            <span className="px-1.5 text-xs text-text-primary" data-testid="action-center-page-indicator">
              {interpolate(t('actionCenter.pageOf'), { page, pageCount: countKnown ? pageCount : '—' })}
            </span>
            <button
              type="button"
              onClick={() => goTo(page + 1)}
              disabled={!countKnown || page >= pageCount}
              aria-label={t('actionCenter.nextPage')}
              data-testid="action-center-next"
              className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border-default text-text-secondary hover:bg-hover disabled:pointer-events-none disabled:opacity-40"
            >
              <ChevronRight size={15} aria-hidden="true" />
            </button>
          </div>
        </div>
      )}
    </PageFrame>
  );
}

export default ActionCenterPage;
