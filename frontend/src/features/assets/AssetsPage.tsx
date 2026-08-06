// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { Database, Edit2, History, HardDrive, Laptop, Plus, Server, Trash2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Button } from '../../components/ui/Button';
import { SkeletonTable } from '../../components/shared/SkeletonTable';
import { EmptyState } from '../../shared/EmptyState';
import { Btn } from '../../shared/ui';
import { ViewToggle } from '../../components/ViewToggle';
import { useI18n } from '../../hooks/useI18n';
import { useToast } from '../../hooks/useToast';
import { useAssetUIStore } from './store';
import { useAssets } from './useAssets';
import { CriticalityBadge } from './CriticalityBadge';
import { CreateAssetModal } from './CreateAssetModal';
import { EditAssetModal } from './EditAssetModal';
import { AssetHistoryDrawer } from './AssetHistoryDrawer';
import type { Asset } from '../../types/asset';

const TypeIcon = ({ type }: { type: string }) => {
  switch ((type || '').toLowerCase()) {
    case 'server':
      return <Server size={16} className="text-info-text" />;
    case 'database':
      return <Database size={16} className="text-success-text" />;
    case 'laptop':
      return <Laptop size={16} className="text-text-secondary" />;
    default:
      return <HardDrive size={16} className="text-purple-400" />;
  }
};

export const AssetsPage = () => {
  const { t } = useI18n();
  const toast = useToast();
  const { assets, isLoading, error, deleteAsset } = useAssets();
  const {
    view,
    setView,
    isCreateModalOpen,
    openCreateModal,
    closeCreateModal,
    editingAssetId,
    openEditModal,
    closeEditModal,
    historyAssetId,
    openHistoryDrawer,
    closeHistoryDrawer,
  } = useAssetUIStore();

  const editingAsset = assets.find((a) => a.id === editingAssetId);

  const handleDelete = (asset: Asset) => {
    if (!asset.id) return;
    if (!confirm(t('assets.confirmDelete').replace('{name}', asset.name ?? ''))) return;
    deleteAsset.mutate(asset.id, {
      onSuccess: () => toast.success(t('assets.deleteSuccess')),
      onError: () => toast.error(t('errors.failedToDeleteAsset')),
    });
  };

  return (
    <div className="p-8 space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">{t('assets.title')}</h1>
          <p className="text-text-secondary text-sm">{t('assets.description')}</p>
        </div>
        <div className="flex items-center gap-4">
          <ViewToggle view={view} onViewChange={setView} />
          <Button onClick={openCreateModal}>
            <Plus size={16} className="mr-2" /> {t('assets.createAsset')}
          </Button>
        </div>
      </div>

      {isLoading ? (
        <SkeletonTable rows={5} columns={5} />
      ) : error ? (
        <div className="rounded-2xl border border-red-900/40 bg-danger-surface p-6 text-center">
          <p className="text-sm text-danger-text">{t('errors.networkError')}</p>
        </div>
      ) : assets.length === 0 ? (
        <EmptyState
          variant="first-use"
          icon={Server}
          title={t('assets.noAssets')}
          description={t('assets.noAssetsDescription')}
          primaryAction={<Btn label={t('assets.createAsset')} icon={Plus} primary onClick={openCreateModal} />}
        />
      ) : view === 'table' ? (
        <div className="bg-surface border border-border rounded-xl overflow-x-auto scrollbar-thin shadow-sm">
          <table className="w-full min-w-[640px] text-left text-sm">
            <thead className="bg-surface-1/5 text-text-secondary font-medium uppercase text-xs">
              <tr>
                <th className="px-6 py-4">{t('assets.form.name')}</th>
                <th className="px-6 py-4">{t('assets.form.type')}</th>
                <th className="px-6 py-4">{t('assets.form.criticality')}</th>
                <th className="px-6 py-4">{t('assets.activeRisks')}</th>
                <th className="px-6 py-4 text-right">{t('common.actions', 'Actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-subtle">
              {assets.map((asset) => (
                <tr key={asset.id} className="hover:bg-surface-1/5 transition-colors group">
                  <td className="px-6 py-4 font-medium text-text-primary">{asset.name}</td>
                  <td className="px-6 py-4 text-text-secondary flex items-center gap-2">
                    <TypeIcon type={asset.type ?? ''} /> {asset.type}
                  </td>
                  <td className="px-6 py-4">
                    <CriticalityBadge level={asset.criticality ?? 'MEDIUM'} />
                  </td>
                  <td className="px-6 py-4">
                    {asset.risks && asset.risks.length > 0 ? (
                      <span className="text-danger-text font-bold flex items-center gap-1">
                        {asset.risks.length} <span className="w-2 h-2 rounded-full bg-danger animate-pulse" />
                      </span>
                    ) : (
                      <span className="text-text-muted">-</span>
                    )}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        title={t('assets.history')}
                        onClick={() => asset.id && openHistoryDrawer(asset.id)}
                        className="p-1.5 rounded-lg text-text-secondary hover:bg-surface-1/10 hover:text-text-primary transition-colors"
                      >
                        <History size={14} />
                      </button>
                      <button
                        title={t('common.edit', 'Edit')}
                        onClick={() => asset.id && openEditModal(asset.id)}
                        className="p-1.5 rounded-lg text-text-secondary hover:bg-surface-1/10 hover:text-text-primary transition-colors"
                      >
                        <Edit2 size={14} />
                      </button>
                      <button
                        title={t('common.delete', 'Delete')}
                        onClick={() => handleDelete(asset)}
                        className="p-1.5 rounded-lg text-text-secondary hover:bg-danger/10 hover:text-danger-text transition-colors"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {assets.map((asset) => (
            <motion.div
              key={asset.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              whileHover={{ y: -4 }}
              className="bg-surface border border-border rounded-lg p-6 hover:border-primary/50 transition-all group"
            >
              <div className="flex items-start justify-between mb-4">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-primary/10">
                    <TypeIcon type={asset.type ?? ''} />
                  </div>
                  <div>
                    <h3 className="font-semibold text-text-primary group-hover:text-primary transition-colors">
                      {asset.name}
                    </h3>
                    <p className="text-xs text-text-muted">{asset.type}</p>
                  </div>
                </div>
              </div>

              <div className="space-y-3 mb-4 border-t border-border pt-4">
                <div className="flex items-center justify-between">
                  <span className="text-xs text-text-secondary">{t('assets.form.criticality')}</span>
                  <CriticalityBadge level={asset.criticality ?? 'MEDIUM'} />
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-xs text-text-secondary">{t('assets.activeRisks')}</span>
                  {asset.risks && asset.risks.length > 0 ? (
                    <span className="text-danger-text font-bold flex items-center gap-1">
                      {asset.risks.length} <span className="w-2 h-2 rounded-full bg-danger animate-pulse" />
                    </span>
                  ) : (
                    <span className="text-text-muted">-</span>
                  )}
                </div>
              </div>

              <div className="flex gap-2 pt-4 border-t border-border">
                <Button
                  variant="ghost"
                  className="flex-1 text-xs flex items-center justify-center gap-1"
                  onClick={() => asset.id && openHistoryDrawer(asset.id)}
                >
                  <History size={14} /> {t('assets.history')}
                </Button>
                <Button
                  variant="ghost"
                  className="flex-1 text-xs flex items-center justify-center gap-1"
                  onClick={() => asset.id && openEditModal(asset.id)}
                >
                  <Edit2 size={14} /> {t('common.edit', 'Edit')}
                </Button>
                <Button
                  variant="ghost"
                  className="flex-1 text-xs flex items-center justify-center gap-1"
                  onClick={() => handleDelete(asset)}
                >
                  <Trash2 size={14} /> {t('common.delete', 'Delete')}
                </Button>
              </div>
            </motion.div>
          ))}
        </div>
      )}

      <CreateAssetModal isOpen={isCreateModalOpen} onClose={closeCreateModal} />
      <EditAssetModal asset={editingAsset} onClose={closeEditModal} />
      <AssetHistoryDrawer assetId={historyAssetId} onClose={closeHistoryDrawer} />
    </div>
  );
};
