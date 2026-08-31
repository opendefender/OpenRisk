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

import { useMemo, type KeyboardEvent } from 'react';
import { Link, useNavigate } from 'react-router';
import { ArrowRight, CircleCheck } from 'lucide-react';

import { useI18n, interpolate } from '../../hooks/useI18n';
import { EmptyState, ErrorState, Skeleton } from '../../shared/ui';
import { useActionItems } from './useActionItems';
import { linkableItems, presentationFor } from './actionLinks';
import type { ActionItem } from './actionCenterService';

const SKELETON_ROWS = 4;

/** Formats a due date in the active locale, or null when the category has none. */
function formatDue(dueAt: string | null | undefined, locale: string): string | null {
  if (!dueAt) return null;
  const date = new Date(dueAt);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleDateString(locale === 'fr' ? 'fr-FR' : 'en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}

function isOverdue(dueAt: string | null | undefined): boolean {
  if (!dueAt) return false;
  const date = new Date(dueAt);
  return !Number.isNaN(date.getTime()) && date.getTime() < Date.now();
}

function ActionRow({ item, href }: { item: ActionItem; href: string }) {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const { icon: Icon, labelKey, tone } = presentationFor(item.type);
  const due = formatDue(item.due_at, locale);
  const overdue = isOverdue(item.due_at);
  const typeLabel = t(`actionCenter.types.${labelKey}`);

  // An anchor already activates on Enter. Space does not, and the acceptance
  // criterion names both, so it is handled explicitly rather than left to be
  // discovered as a dead key by whoever navigates this list without a mouse.
  const onKeyDown = (event: KeyboardEvent<HTMLAnchorElement>) => {
    if (event.key === ' ' || event.key === 'Spacebar') {
      event.preventDefault();
      navigate(href);
    }
  };

  return (
    <li>
      <Link
        to={href}
        onKeyDown={onKeyDown}
        data-testid="action-center-item"
        data-action-type={item.type}
        className="group flex items-center gap-3 rounded-md px-3 py-2.5 no-underline transition-colors hover:bg-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
      >
        <Icon size={16} strokeWidth={1.75} className={`shrink-0 ${tone}`} aria-hidden="true" />

        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-text-primary">{item.title}</span>
          <span className="mt-0.5 block text-xs text-text-muted">
            {typeLabel}
            {due && (
              <>
                {' · '}
                <span className={overdue ? 'text-danger-text font-medium' : undefined}>
                  {overdue ? `${t('actionCenter.overdue')} — ${due}` : interpolate(t('actionCenter.due'), { date: due })}
                </span>
              </>
            )}
            {!due && ` · ${t('actionCenter.noDueDate')}`}
          </span>
        </span>

        {/* The row's accessible name is the title plus its category; the arrow is
            decoration, so it is hidden from the accessibility tree and the link
            carries an explicit label instead of relying on the glyph. */}
        <ArrowRight
          size={15}
          strokeWidth={1.75}
          aria-hidden="true"
          className="shrink-0 text-text-muted opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
        />
        <span className="sr-only">{interpolate(t('actionCenter.openItem'), { title: item.title })}</span>
      </Link>
    </li>
  );
}

export function ActionCenterPanel({ limit = 8 }: { limit?: number }) {
  const { t } = useI18n();
  const { items, total, isLoading, isError, refetch } = useActionItems(limit);

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
          <h2 id="action-center-title" className="text-md font-semibold text-text-primary">
            {t('actionCenter.title')}
          </h2>
          <p className="mt-0.5 text-xs text-text-muted">{t('actionCenter.subtitle')}</p>
        </div>
        {!isLoading && !isError && total > 0 && (
          <span className="shrink-0 text-xs text-text-muted" data-testid="action-center-count">
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
              <ActionRow key={item.id} item={item} href={href} />
            ))}
          </ul>
          {total > rows.length && (
            <div className="mt-2 px-3">
              {/* No dedicated /action-center route ships in this issue, so this
                  says how much is not on screen rather than offering a link to a
                  page that does not exist. Fast-follow: the full-page view. */}
              <span className="text-xs text-text-muted" data-testid="action-center-more">
                {interpolate(t('actionCenter.moreOutstanding'), { count: total - rows.length })}
              </span>
            </div>
          )}
        </>
      )}
    </section>
  );
}

export default ActionCenterPanel;
