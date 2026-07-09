// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect } from 'react';
import { motion } from 'framer-motion';
import { ClipboardList, FileDown, Library, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { SkeletonTable } from '../../components/shared/SkeletonTable';
import { EmptyState } from '../../components/shared/EmptyState';
import { useI18n } from '../../hooks/useI18n';
import { useToast } from '../../hooks/useToast';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useComplianceUIStore } from './store';
import { useComplianceReport, useControls, useFrameworks } from './useCompliance';
import { computeComplianceProgress } from './utils';
import { ComplianceGauge } from './ComplianceGauge';
import { ControlTable } from './ControlTable';
import { ControlDrawer } from './ControlDrawer';
import { CreateFrameworkModal } from './CreateFrameworkModal';
import { CreateControlModal } from './CreateControlModal';
import { ImportCatalogModal } from './ImportCatalogModal';
import type { ControlStatus } from '../../types/compliance';

export const CompliancePage = () => {
  const { t, locale } = useI18n();
  const toast = useToast();
  const isAdmin = useAuthStore((s) => s.hasRole('admin'));
  const report = useComplianceReport();

  const {
    selectedFrameworkId,
    selectFramework,
    isCreateFrameworkModalOpen,
    openCreateFrameworkModal,
    closeCreateFrameworkModal,
    isCreateControlModalOpen,
    openCreateControlModal,
    closeCreateControlModal,
    isImportCatalogModalOpen,
    openImportCatalogModal,
    closeImportCatalogModal,
    openControlDrawer,
  } = useComplianceUIStore();

  const { frameworks, isLoading: frameworksLoading, error: frameworksError, deleteFramework } = useFrameworks();
  const { controls, isLoading: controlsLoading, error: controlsError, updateControl } =
    useControls(selectedFrameworkId ?? undefined);

  const handleDeleteFramework = (id: string, name: string) => {
    if (!window.confirm(t('compliance.deleteFrameworkConfirm').replace('{name}', name))) return;
    deleteFramework.mutate(id, {
      onSuccess: () => {
        if (selectedFrameworkId === id) selectFramework(null);
        toast.success(t('compliance.deleteFrameworkSuccess'));
      },
      onError: () => toast.error(t('compliance.deleteFrameworkError')),
    });
  };

  // Default to the first framework once the list has loaded.
  useEffect(() => {
    if (!selectedFrameworkId && frameworks.length > 0 && frameworks[0].id) {
      selectFramework(frameworks[0].id);
    }
  }, [frameworks, selectedFrameworkId, selectFramework]);

  const handleStatusChange = (controlId: string, status: ControlStatus) => {
    updateControl.mutate(
      { id: controlId, payload: { status } },
      { onError: () => toast.error(t('errors.failedToUpdateControl')) }
    );
  };

  const handleDownloadReport = () => {
    if (!selectedFrameworkId) return;
    report.mutate(
      { frameworkId: selectedFrameworkId, locale },
      {
        onSuccess: () => toast.success(t('compliance.report.success')),
        onError: () => toast.error(t('compliance.report.error')),
      }
    );
  };

  return (
    // h-full + overflow-y-auto: the page lives inside an overflow-hidden layout
    // main, so without its own scroll container a tall controls table (e.g. 198
    // controls) would be clipped with no way to scroll down.
    <div className="h-full overflow-y-auto">
      <div className="p-6 max-w-7xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-white">{t('compliance.title')}</h1>
          <p className="text-sm text-zinc-500">{t('compliance.description')}</p>
        </div>
        {isAdmin && (
          <div className="flex items-center gap-2">
            <Button onClick={openImportCatalogModal} className="gap-2">
              <Library size={16} />
              {t('compliance.catalog.buttonLabel')}
            </Button>
            <Button variant="ghost" onClick={openCreateFrameworkModal} className="gap-2">
              <Plus size={16} />
              {t('compliance.createFramework')}
            </Button>
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[260px_1fr]">
        {/* Framework rail */}
        <div className="space-y-2">
          {frameworksLoading ? (
            <div className="space-y-2">
              {[0, 1, 2].map((i) => (
                <div key={i} className="h-11 animate-pulse rounded-xl bg-zinc-900" />
              ))}
            </div>
          ) : frameworksError ? (
            <p className="text-sm text-red-400">{t('errors.networkError')}</p>
          ) : frameworks.length === 0 ? (
            <EmptyState
              icon={<ShieldCheck size={24} />}
              title={t('compliance.noFrameworks')}
              description={t('compliance.noFrameworksDescription')}
              action={isAdmin ? { label: t('compliance.catalog.buttonLabel'), onClick: openImportCatalogModal } : undefined}
            />
          ) : (
            frameworks.map((framework, index) => (
              <motion.div
                key={framework.id}
                initial={{ opacity: 0, x: -8 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: Math.min(index * 0.03, 0.3) }}
                className={`group relative rounded-xl border transition-all ${
                  selectedFrameworkId === framework.id
                    ? 'border-primary bg-primary/10'
                    : 'border-zinc-800 bg-zinc-950/40 hover:border-zinc-700'
                }`}
              >
                <button
                  type="button"
                  onClick={() => framework.id && selectFramework(framework.id)}
                  className={`w-full rounded-xl px-4 py-2.5 pr-10 text-left text-sm transition-colors ${
                    selectedFrameworkId === framework.id ? 'text-white' : 'text-zinc-400 group-hover:text-zinc-200'
                  }`}
                >
                  <div className="font-medium">{framework.name}</div>
                  {framework.version && <div className="text-xs text-zinc-500">{framework.version}</div>}
                </button>
                {isAdmin && framework.id && (
                  <button
                    type="button"
                    aria-label={t('compliance.deleteFramework')}
                    title={t('compliance.deleteFramework')}
                    onClick={() => handleDeleteFramework(framework.id as string, framework.name)}
                    className="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1.5 text-zinc-500 opacity-0 transition-all hover:bg-red-500/10 hover:text-red-400 focus:opacity-100 group-hover:opacity-100"
                  >
                    <Trash2 size={15} />
                  </button>
                )}
              </motion.div>
            ))
          )}
        </div>

        {/* Controls panel */}
        <div className="space-y-4">
          {!selectedFrameworkId ? (
            <EmptyState icon={<ShieldCheck size={28} />} title={t('compliance.selectFramework')} />
          ) : (
            <>
              <ComplianceGauge progress={computeComplianceProgress(selectedFrameworkId, controls)} />

              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-500">
                  {t('compliance.controls')}
                </h2>
                <div className="flex items-center gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleDownloadReport}
                    disabled={report.isPending}
                    className="gap-2"
                    title={t('compliance.report.hint')}
                  >
                    <FileDown size={14} />
                    {report.isPending ? t('compliance.report.generating') : t('compliance.report.buttonLabel')}
                  </Button>
                  <Button variant="secondary" size="sm" onClick={openCreateControlModal} className="gap-2">
                    <Plus size={14} />
                    {t('compliance.addControl')}
                  </Button>
                </div>
              </div>

              {controlsLoading ? (
                <SkeletonTable rows={5} columns={4} />
              ) : controlsError ? (
                <div className="rounded-2xl border border-red-900/40 bg-red-950/20 p-6 text-center">
                  <p className="text-sm text-red-400">{t('errors.networkError')}</p>
                </div>
              ) : controls.length === 0 ? (
                <EmptyState
                  icon={<ClipboardList size={28} />}
                  title={t('compliance.noControls')}
                  description={t('compliance.noControlsDescription')}
                  action={{ label: t('compliance.addControl'), onClick: openCreateControlModal }}
                />
              ) : (
                <ControlTable controls={controls} onOpenControl={openControlDrawer} onStatusChange={handleStatusChange} />
              )}
            </>
          )}
        </div>
      </div>

      <CreateFrameworkModal isOpen={isCreateFrameworkModalOpen} onClose={closeCreateFrameworkModal} />
      {/* Catalog import is page-level: it creates/reuses its own framework, so it must
          work even when no framework is selected (or none exist yet). On success we
          select the imported framework so its controls show immediately. */}
      <ImportCatalogModal
        isOpen={isImportCatalogModalOpen}
        onClose={closeImportCatalogModal}
        onImported={(id) => selectFramework(id)}
      />
      {selectedFrameworkId && (
        <>
          <CreateControlModal
            isOpen={isCreateControlModalOpen}
            onClose={closeCreateControlModal}
            frameworkId={selectedFrameworkId}
          />
          <ControlDrawer frameworkId={selectedFrameworkId} />
        </>
      )}
      </div>
    </div>
  );
};
