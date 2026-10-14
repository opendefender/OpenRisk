// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Inventaire logiciel" in the asset drawer (#910): what the OpenRisk agent
// last reported for the hosts attached to this asset, and the state of the
// vulnerability check on it. Every line is a stored fact; an error is a code
// worded here, never hidden.

import { useQuery } from '@tanstack/react-query';

import { useI18n } from '../../../hooks/useI18n';
import { formatDate, formatNumber } from '../../../i18n/format';
import { BlockEmpty, BlockError, BlockSkeleton } from '../../dashboard/console/Panel';
import {
  vulnerabilityService,
  type HostInventory,
} from '../../vulnerabilities/vulnerabilityService';

const KNOWN_ERRORS = ['osquery_missing', 'source_unreachable', 'lookup_disabled'] as const;

export function InventorySection({ assetId, enabled }: { assetId: string; enabled: boolean }) {
  const { t } = useI18n();
  const q = useQuery({
    queryKey: ['assets', assetId, 'inventories'],
    queryFn: () => vulnerabilityService.assetInventories(assetId),
    enabled,
  });

  return (
    <section data-testid="inventory-section">
      <h3 className="m-0 mb-2 text-[11px] uppercase tracking-[.06em] font-semibold text-ink-muted">
        {t('hostInventory.title')}
      </h3>
      {q.isLoading ? (
        <BlockSkeleton lines={2} height={18} />
      ) : q.isError ? (
        <BlockError onRetry={() => void q.refetch()} />
      ) : !q.data || q.data.length === 0 ? (
        <BlockEmpty>{t('hostInventory.empty')}</BlockEmpty>
      ) : (
        <ul className="m-0 p-0 list-none grid gap-2">
          {q.data.map((inv) => (
            <InventoryLine key={inv.id} inv={inv} />
          ))}
        </ul>
      )}
    </section>
  );
}

function InventoryLine({ inv }: { inv: HostInventory }) {
  const { t, locale } = useI18n();
  const when = (d: string) =>
    formatDate(locale, d, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
  const error = (KNOWN_ERRORS as readonly string[]).includes(inv.match_error)
    ? t(`hostInventory.error.${inv.match_error}`)
    : inv.match_error
      ? t('hostInventory.error.unknown')
      : '';
  return (
    <li
      className="rounded-[10px] border border-border-subtle px-3 py-2.5 text-[12.5px]"
      data-testid={`inventory-${inv.host_key}`}
    >
      <div className="flex items-baseline justify-between gap-3">
        <span className="mono text-ink font-semibold truncate">{inv.hostname || inv.host_key}</span>
        <span className="text-ink-muted whitespace-nowrap">
          {inv.os} {inv.os_version}
        </span>
      </div>
      <div className="mt-1 text-ink-soft">
        {t('hostInventory.packages', {
          count: inv.package_count,
          n: formatNumber(locale, inv.package_count),
        })}
        {inv.uncheckable_count > 0 &&
          ` · ${t('hostInventory.uncheckable', {
            count: inv.uncheckable_count,
            n: formatNumber(locale, inv.uncheckable_count),
          })}`}
      </div>
      <div className="mt-0.5 text-ink-muted">
        {t('hostInventory.collected', { date: when(inv.collected_at) })}
        {inv.matched_at && ` · ${t('hostInventory.matched', { date: when(inv.matched_at) })}`}
      </div>
      {error && (
        <p className="m-0 mt-1 text-warning-text" role="status">
          {error}
        </p>
      )}
    </li>
  );
}
