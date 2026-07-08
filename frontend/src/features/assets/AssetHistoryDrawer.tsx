// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { Clock, History } from 'lucide-react';
import { Drawer } from '../../components/ui/Drawer';
import { EmptyState } from '../../components/shared/EmptyState';
import { useI18n } from '../../hooks/useI18n';
import { useAssets, useAssetHistory } from './useAssets';
import { CriticalityBadge } from './CriticalityBadge';

interface AssetHistoryDrawerProps {
  assetId: string | null;
  onClose: () => void;
}

export const AssetHistoryDrawer = ({ assetId, onClose }: AssetHistoryDrawerProps) => {
  const { t } = useI18n();
  const { assets } = useAssets();
  const asset = assets.find((a) => a.id === assetId);
  const { data: history, isLoading, error } = useAssetHistory(assetId ?? undefined);

  return (
    <Drawer
      isOpen={!!assetId}
      onClose={onClose}
      title={asset ? t('assets.historyTitle').replace('{name}', asset.name ?? '') : t('assets.history')}
    >
      {isLoading ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-16 animate-pulse rounded-xl bg-zinc-900" />
          ))}
        </div>
      ) : error ? (
        <p className="text-sm text-red-400">{t('errors.networkError')}</p>
      ) : !history || history.length === 0 ? (
        <EmptyState
          icon={<History size={28} />}
          title={t('assets.noHistory')}
          description={t('assets.noHistoryDescription')}
        />
      ) : (
        <div className="space-y-3">
          {history.map((snapshot) => (
            <div
              key={snapshot.id}
              className="rounded-xl border border-zinc-800 bg-zinc-950/40 p-4"
            >
              <div className="flex items-center justify-between mb-2">
                <span className="flex items-center gap-1.5 text-xs text-zinc-500">
                  <Clock size={12} />
                  {snapshot.created_at ? new Date(snapshot.created_at).toLocaleString() : '—'}
                </span>
                <span className="rounded-full bg-white/5 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                  {t(`assets.historyReason.${snapshot.reason ?? 'update'}`)}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-white">{snapshot.name}</p>
                  <p className="text-xs text-zinc-500">{snapshot.type}</p>
                </div>
                {snapshot.criticality && <CriticalityBadge level={snapshot.criticality} />}
              </div>
            </div>
          ))}
        </div>
      )}
    </Drawer>
  );
};
