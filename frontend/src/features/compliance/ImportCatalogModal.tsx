// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { AnimatePresence, motion } from 'framer-motion';
import { X, Library, Clock, Loader2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { useI18n } from '../../hooks/useI18n';
import { useToast } from '../../hooks/useToast';
import { useCatalogs, useImportCatalogAsFramework } from './useCompliance';
import type { ComplianceCatalogSummary } from '../../types/compliance';

interface ImportCatalogModalProps {
  isOpen: boolean;
  onClose: () => void;
  // Called with the (created or reused) framework id once a catalog is imported,
  // so the page can select it in the rail.
  onImported?: (frameworkId: string) => void;
}

export const ImportCatalogModal = ({ isOpen, onClose, onImported }: ImportCatalogModalProps) => {
  const { t } = useI18n();
  const toast = useToast();
  const { data: catalogs, isLoading, error } = useCatalogs();
  const importCatalog = useImportCatalogAsFramework();

  const handleImport = async (catalog: ComplianceCatalogSummary) => {
    try {
      const { framework, result } = await importCatalog.mutateAsync(catalog);
      toast.success(
        t('compliance.catalog.importSuccess')
          .replace('{imported}', String(result.imported))
          .replace('{skipped}', String(result.skipped))
      );
      onImported?.(framework.id);
      onClose();
    } catch {
      toast.error(t('compliance.catalog.importError'));
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
          />
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 40 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 40 }}
            transition={{ duration: 0.22, type: 'spring', stiffness: 240 }}
            className="fixed inset-x-0 top-1/2 z-50 mx-auto w-full max-w-lg -translate-y-1/2 transform px-4"
          >
            <div className="rounded-3xl border border-zinc-800 bg-zinc-950/95 p-6 shadow-2xl shadow-black/40">
              <div className="flex items-center justify-between gap-4 mb-2">
                <div className="flex items-center gap-3">
                  <div className="rounded-2xl bg-primary/10 p-2 text-primary">
                    <Library size={20} />
                  </div>
                  <h2 className="text-xl font-semibold">{t('compliance.catalog.title')}</h2>
                </div>
                <button
                  type="button"
                  onClick={onClose}
                  className="rounded-full p-2 text-zinc-400 hover:bg-white/10 hover:text-white transition-colors"
                >
                  <X size={20} />
                </button>
              </div>
              <p className="text-sm text-zinc-500 mb-6">{t('compliance.catalog.description')}</p>

              {isLoading ? (
                <div className="space-y-2">
                  {[0, 1, 2].map((i) => (
                    <div key={i} className="h-16 animate-pulse rounded-xl bg-zinc-900" />
                  ))}
                </div>
              ) : error ? (
                <p className="text-sm text-red-400">{t('errors.networkError')}</p>
              ) : !catalogs || catalogs.length === 0 ? (
                <p className="text-sm text-zinc-500">{t('compliance.catalog.empty')}</p>
              ) : (
                <div className="space-y-2 max-h-96 overflow-y-auto">
                  {catalogs.map((catalog) => (
                    <div
                      key={catalog.key}
                      className={`flex items-center justify-between gap-4 rounded-xl border p-4 ${
                        catalog.available
                          ? 'border-zinc-800 bg-zinc-900/40'
                          : 'border-zinc-900 bg-zinc-950/40 opacity-60'
                      }`}
                    >
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-white">
                            {catalog.name}
                            {catalog.version ? ` ${catalog.version}` : ''}
                          </span>
                          {!catalog.available && (
                            <span className="flex items-center gap-1 rounded-full bg-zinc-800 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                              <Clock size={10} />
                              {t('compliance.catalog.comingSoon')}
                            </span>
                          )}
                        </div>
                        <p className="mt-0.5 truncate text-xs text-zinc-500">
                          {catalog.available
                            ? t('compliance.catalog.controlCount').replace('{count}', String(catalog.control_count))
                            : t('compliance.catalog.comingSoonDescription')}
                        </p>
                      </div>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        disabled={!catalog.available || importCatalog.isPending}
                        onClick={() => handleImport(catalog)}
                        className="shrink-0"
                      >
                        {importCatalog.isPending && importCatalog.variables?.key === catalog.key ? (
                          <Loader2 size={14} className="animate-spin" />
                        ) : (
                          t('compliance.catalog.import')
                        )}
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};
