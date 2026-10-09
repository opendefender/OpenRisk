// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The first five items of the member's action center, filtered to what their
// view acts on (#901). The order is the server's; the list never re-sorts.

import { Link } from 'react-router';
import { useI18n } from '../../../hooks/useI18n';
import { useActionItems } from '../../action-center/useActionItems';
import type { ActionItemType } from '../../action-center/actionCenterService';
import { linkableItems } from '../../action-center/actionLinks';
import { iconFor, kindOf } from '../../action-center/actionKinds';
import type { DashboardVariant } from '../dashboardVariant';
import { DuePill } from '../../action-center/DuePill';
import { BlockEmpty, BlockError, BlockSkeleton, HeaderLink, Panel, PanelTitle } from './Panel';


// What each view acts on. The RSSI view takes everything.
const KINDS: Record<DashboardVariant, ActionItemType[] | null> = {
  rssi: null,
  rm: ['critical_risk', 'overdue_mitigation', 'mitigation_review', 'pending_approval', 'vendor_followup'],
  audit: ['expiring_evidence', 'overdue_remediation', 'pending_approval'],
  exec: ['pending_approval', 'critical_risk'],
};

export function PriorityList({
  variant,
  className = '',
}: {
  variant: DashboardVariant;
  className?: string;
}) {
  const { t } = useI18n();
  const { items, isLoading, isError, refetch } = useActionItems({ limit: 20 });
  const kinds = KINDS[variant];
  // Only items whose link resolves to a real in-app route earn a row; the
  // server's order is kept.
  const shown = linkableItems(items)
    .filter(({ item }) => !kinds || kinds.includes(item.type))
    .slice(0, 5);

  return (
    <Panel testId="dash-priorities" className={`overflow-hidden ${className}`}>
      <PanelTitle
        className="px-5 pt-4 pb-2.5"
        title={t(`dashboard.priorities.${variant}`)}
        aside={<HeaderLink to="/action-center">{t('dashboard.priorities.link')}</HeaderLink>}
      />
      {isLoading ? (
        <div className="px-5 pb-4">
          <BlockSkeleton lines={5} height={30} />
        </div>
      ) : isError ? (
        <div className="px-5">
          <BlockError onRetry={refetch} />
        </div>
      ) : shown.length === 0 ? (
        <div className="px-5 pb-3 border-t border-border-subtle" data-testid="dash-priorities-empty">
          <BlockEmpty>{t('dashboard.priorities.empty')}</BlockEmpty>
        </div>
      ) : (
        shown.map(({ item: a, href }) => {
          const Icon = iconFor(a.type);
          return (
            <Link
              key={a.id}
              to={href}
              data-testid="dash-priority-item"
              data-action-type={a.type}
              className="flex items-center gap-3 px-5 py-[11px] border-t border-border-subtle hover:bg-surface-2 transition-colors"
            >
              <Icon size={16} strokeWidth={1.75} className="text-ink-muted w-[18px] shrink-0" aria-hidden="true" />
              <span className="flex-1 min-w-0">
                <span className="block text-[13px] font-medium text-ink truncate">{a.title}</span>
                <span className="block text-[11.5px] text-ink-muted truncate">
                  {kindOf(a.type)
                    ? t(`actionCenter.kind.${a.type}`)
                    : t('actionCenter.types.unknown')}
                </span>
              </span>
              {a.due_at && <DuePill iso={a.due_at} />}
            </Link>
          );
        })
      )}
    </Panel>
  );
}
