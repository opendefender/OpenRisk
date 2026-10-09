// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Activity on the October 2026 redesign (#905): the organisation's journal,
// filtered server-side to what the member may read. One sentence per entry
// ("Fatou Ndiaye a créé la mitigation Bastion d'administration (PAM)"), grouped
// by day on a vertical rail, with the time on the right. System entries carry
// the product's mark. Older entries load when the end of the list scrolls into
// view, or with the button.
//
// An object with a drawer opens it in place, so reading the journal and looking
// into an entry are the same gesture; other objects link to their page.

import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router';

import { useI18n } from '../../hooks/useI18n';
import { formatDate, formatTime } from '../../i18n/format';
import { OpenRiskLogo } from '../../shared/Logo';
import { Button } from '../../shared/ds';
import { PageFrame, PageHeader } from '../../shared/ui';
import { BlockEmpty, BlockError, BlockSkeleton } from '../dashboard/console/Panel';
import { useDrawerController } from '../entity-drawer/drawerState';
import { isEntityType, type TimelineEvent } from '../entity-drawer/types';
import { useTenantTimeline } from '../entity-drawer/useEntityDrawer';
import {
  JOURNAL_FILTERS,
  groupByDay,
  initialsOf,
  phraseKeys,
  relativeDay,
  type JournalFilter,
} from './journal';

export function ActivityPage() {
  const { t, locale } = useI18n();
  const [filter, setFilter] = useState<JournalFilter>('');
  const feed = useTenantTimeline(filter || undefined);
  const events = useMemo(() => feed.data?.pages.flatMap((p) => p.events) ?? [], [feed.data]);
  const days = useMemo(() => groupByDay(events), [events]);
  const now = new Date();

  // Load older entries as the end of the list comes into view.
  const sentinel = useRef<HTMLDivElement | null>(null);
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = feed;
  useEffect(() => {
    const el = sentinel.current;
    if (!el || !hasNextPage || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting) && !isFetchingNextPage) void fetchNextPage();
    });
    io.observe(el);
    return () => io.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const dayLabel = (key: string, sample: string) => {
    const rel = relativeDay(sample, now);
    if (rel) return t(`activity.${rel}`);
    const sameYear = key.slice(0, 4) === String(now.getFullYear());
    return formatDate(locale, sample, {
      day: 'numeric',
      month: 'short',
      ...(sameYear ? {} : { year: 'numeric' }),
    });
  };

  return (
    <PageFrame>
      <div className="mx-auto max-w-[900px]">
        <PageHeader
          className="!mb-[18px]"
          title={t('activity.title')}
          subtitle={t('activity.subtitle')}
        />

        <div
          className="flex gap-1.5 flex-wrap mb-[22px]"
          role="group"
          aria-label={t('activity.filterLabel')}
        >
          {JOURNAL_FILTERS.map((f) => {
            const on = f === filter;
            return (
              <button
                key={f || 'all'}
                type="button"
                aria-pressed={on}
                onClick={() => setFilter(f)}
                data-testid={`act-filter-${f || 'all'}`}
                className={`h-[30px] px-3 rounded-full border text-[12.5px] font-semibold transition-colors ${
                  on
                    ? 'bg-surface-3 border-border-strong text-ink'
                    : 'bg-transparent border-border-default text-ink-soft hover:border-border-strong'
                }`}
              >
                {t(`activity.filters.${f || 'all'}`)}
              </button>
            );
          })}
        </div>

        {feed.isLoading ? (
          <BlockSkeleton lines={8} height={28} />
        ) : feed.isError ? (
          <BlockError onRetry={() => void feed.refetch()} />
        ) : events.length === 0 ? (
          <div className="grid justify-items-start gap-2">
            <BlockEmpty>{filter ? t('activity.emptyFiltered') : t('activity.empty')}</BlockEmpty>
            {filter && (
              <Button variant="secondary" size="sm" onClick={() => setFilter('')}>
                {t('activity.showAll')}
              </Button>
            )}
          </div>
        ) : (
          <>
            {days.map((d) => (
              <section key={d.day} className="mb-6" data-testid="act-day">
                <h2 className="m-0 mb-1.5 text-[10.5px] tracking-[0.06em] uppercase font-semibold text-ink-muted">
                  {dayLabel(d.day, d.events[0].occurred_at)}
                </h2>
                <ol className="list-none m-0 p-0 border-l border-border-default ml-[15px]">
                  {d.events.map((e) => (
                    <JournalEntry key={e.id} e={e} />
                  ))}
                </ol>
              </section>
            ))}
            <div ref={sentinel} className="flex justify-center py-2">
              {feed.hasNextPage ? (
                <Button
                  variant="secondary"
                  size="sm"
                  loading={feed.isFetchingNextPage}
                  onClick={() => void feed.fetchNextPage()}
                  data-testid="act-more"
                >
                  {feed.isFetchingNextPage ? t('activity.loadingMore') : t('activity.loadMore')}
                </Button>
              ) : (
                <span className="text-[11.5px] text-ink-muted">{t('activity.end')}</span>
              )}
            </div>
          </>
        )}
      </div>
    </PageFrame>
  );
}

function JournalEntry({ e }: { e: TimelineEvent }) {
  const { t, locale } = useI18n();
  const { open } = useDrawerController();
  const system = !e.actor;
  const who = e.actor?.label || e.actor?.email || t('activity.system');
  const keys = phraseKeys(e);
  const noun = keys.noun ? t(keys.noun) : '';
  const object = e.object?.trim() || '';
  const type = e.target.type;

  const objectText = object || null;
  const objectClass = 'mono text-[12.5px] text-accent hover:underline';
  let objectNode = null;
  if (objectText) {
    if (type && isEntityType(type)) {
      objectNode = (
        <button type="button" className={objectClass} onClick={() => open(type, e.target.id)}>
          {objectText}
        </button>
      );
    } else if (e.target_url) {
      objectNode = (
        <Link to={e.target_url} className={objectClass}>
          {objectText}
        </Link>
      );
    } else {
      objectNode = <span className="mono text-[12.5px] text-ink">{objectText}</span>;
    }
  }

  return (
    <li className="flex gap-3 items-start py-2 -ml-4" data-testid="act-entry">
      <span
        className="w-[31px] h-[31px] rounded-full bg-surface-1 border border-border-default flex items-center justify-center shrink-0 text-[10.5px] font-semibold text-ink-soft"
        aria-hidden="true"
        data-testid={system ? 'act-avatar-system' : 'act-avatar'}
      >
        {system ? <OpenRiskLogo size={15} className="text-accent" title="" /> : initialsOf(who)}
      </span>
      <span className="flex-1 min-w-0 text-[13px] leading-normal pt-[5px] text-ink-soft break-words">
        <b className="text-ink font-semibold">{who}</b> {t(keys.verb)}
        {noun ? ` ${noun}` : ''} {objectNode}
      </span>
      <time
        dateTime={e.occurred_at}
        className="mono text-[11.5px] text-ink-muted pt-[7px] shrink-0"
      >
        {formatTime(locale, e.occurred_at)}
      </time>
    </li>
  );
}
