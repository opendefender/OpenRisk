// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The last six events of the organisation's journal (#901), read from the same
// feed as the Activity page (GET /timeline), already filtered server-side to
// what the member may see. Each entry reads as the Activity journal's sentence
// (#905).

import { Link } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { useI18n } from '../../../hooks/useI18n';
import { formatTime } from '../../../i18n/format';
import { fetchTenantTimeline } from '../../entity-drawer/entityService';
import type { TimelineEvent } from '../../entity-drawer/types';
import { phraseKeys } from '../../activity/journal';
import { BlockEmpty, BlockError, BlockSkeleton, HeaderLink, Panel, PanelTitle } from './Panel';

export function RecentActivity({ className = '' }: { className?: string }) {
  const { t, locale } = useI18n();
  const feed = useQuery({
    queryKey: ['timeline', 'tenant', 'recent', 6],
    queryFn: () => fetchTenantTimeline({ limit: 6 }),
    staleTime: 30_000,
  });
  const events = feed.data?.events ?? [];
  // The journal's own sentence (#905); the raw summary only for an entry the
  // journal could not classify.
  const phrase = (e: TimelineEvent) => {
    if (!e.domain) return e.summary;
    const k = phraseKeys(e);
    return [t(k.verb), k.noun ? t(k.noun) : '', e.object ?? ''].filter(Boolean).join(' ');
  };

  return (
    <Panel testId="dash-activity" className={`pb-1.5 ${className}`}>
      <PanelTitle
        className="px-5 pt-4 pb-1.5"
        title={t('dashboard.activity.title')}
        aside={<HeaderLink to="/activity">{t('dashboard.activity.all')}</HeaderLink>}
      />
      {feed.isLoading ? (
        <div className="px-5 pb-3">
          <BlockSkeleton lines={6} />
        </div>
      ) : feed.isError ? (
        <div className="px-5">
          <BlockError onRetry={() => void feed.refetch()} />
        </div>
      ) : events.length === 0 ? (
        <div className="px-5">
          <BlockEmpty>{t('dashboard.activity.empty')}</BlockEmpty>
        </div>
      ) : (
        events.map((e) => (
          <div key={e.id} className="flex gap-2.5 px-5 py-2 text-[12.5px] leading-[1.45]">
            <span className="mono text-[11.5px] text-ink-muted min-w-[38px] whitespace-nowrap shrink-0 pt-px">
              {formatTime(locale, e.occurred_at)}
            </span>
            <span className="text-ink-soft min-w-0 break-words">
              <b className="text-ink font-semibold">
                {e.actor?.label ?? t('dashboard.activity.system')}
              </b>{' '}
              {e.target_url ? (
                <Link to={e.target_url} className="hover:underline">
                  {phrase(e)}
                </Link>
              ) : (
                phrase(e)
              )}
            </span>
          </div>
        ))
      )}
    </Panel>
  );
}
