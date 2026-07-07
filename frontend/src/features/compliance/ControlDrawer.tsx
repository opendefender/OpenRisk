// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Download, FileText, Trash2, Upload } from 'lucide-react';
import { Drawer } from '../../components/ui/Drawer';
import { Button } from '../../components/ui/Button';
import { EmptyState } from '../../components/shared/EmptyState';
import { useI18n } from '../../hooks/useI18n';
import { useToast } from '../../hooks/useToast';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useComplianceUIStore } from './store';
import { useControls, useEvidences } from './useCompliance';
import { CONTROL_STATUSES, type ControlStatus } from '../../types/compliance';

interface ControlDrawerProps {
  frameworkId: string;
}

export const ControlDrawer = ({ frameworkId }: ControlDrawerProps) => {
  const { t } = useI18n();
  const toast = useToast();
  const hasRole = useAuthStore((s) => s.hasRole);
  const isAdmin = hasRole('admin');

  const { isControlDrawerOpen, activeControlId, activeDrawerTab, closeControlDrawer, setActiveDrawerTab } =
    useComplianceUIStore();

  const { controls, updateControl } = useControls(frameworkId);
  const control = controls.find((c) => c.id === activeControlId);

  const { evidences, isLoading: evidencesLoading, createEvidence, deleteEvidence, downloadEvidence } =
    useEvidences(activeControlId ?? undefined);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const [description, setDescription] = useState('');

  if (!control) return null;

  const handleStatusChange = (status: ControlStatus) => {
    if (!control.id) return;
    updateControl.mutate(
      { id: control.id, payload: { status } },
      {
        onError: () => toast.error(t('errors.failedToUpdateControl')),
      }
    );
  };

  const handleUpload = (file: File) => {
    createEvidence.mutate(
      { file, description: description || undefined },
      {
        onSuccess: () => {
          toast.success(t('compliance.evidence.upload'));
          setDescription('');
          if (fileInputRef.current) fileInputRef.current.value = '';
        },
        onError: () => toast.error(t('errors.failedToUploadEvidence')),
      }
    );
  };

  return (
    <Drawer isOpen={isControlDrawerOpen} onClose={closeControlDrawer} title={control.name}>
      <div className="flex gap-2 border-b border-zinc-800 pb-3 mb-6">
        {(['details', 'evidence'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveDrawerTab(tab)}
            className={`rounded-full px-4 py-1.5 text-sm font-medium transition-colors ${
              activeDrawerTab === tab ? 'bg-primary text-white' : 'text-zinc-400 hover:bg-white/5'
            }`}
          >
            {t(`compliance.tabs.${tab}`)}
          </button>
        ))}
      </div>

      {activeDrawerTab === 'details' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-5">
          <div>
            <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              {t('compliance.referenceCode')}
            </label>
            <p className="mt-1 font-mono text-sm text-zinc-300">{control.reference_code || '—'}</p>
          </div>
          <div>
            <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              {t('compliance.form.description')}
            </label>
            <p className="mt-1 text-sm text-zinc-300">{control.description || '—'}</p>
          </div>
          <div>
            <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              {t('common.status')}
            </label>
            <div className="mt-2 flex flex-wrap gap-2">
              {CONTROL_STATUSES.map((status) => (
                <button
                  key={status}
                  onClick={() => handleStatusChange(status)}
                  disabled={updateControl.isPending}
                  className={`rounded-full border px-3 py-1.5 text-xs font-medium transition-all disabled:opacity-50 ${
                    control.status === status
                      ? 'border-primary bg-primary/20 text-white'
                      : 'border-zinc-800 bg-zinc-900 text-zinc-400 hover:border-zinc-600'
                  }`}
                >
                  {t(`compliance.status.${status}`)}
                </button>
              ))}
            </div>
          </div>
        </motion.div>
      )}

      {activeDrawerTab === 'evidence' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-6">
          <div className="rounded-2xl border border-dashed border-zinc-700 p-5">
            <input
              ref={fileInputRef}
              type="file"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) handleUpload(file);
              }}
            />
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('compliance.evidence.description')}
              rows={2}
              className="mb-3 w-full rounded-xl border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-white outline-none focus:ring-2 focus:ring-primary/40"
            />
            <Button
              type="button"
              variant="secondary"
              size="sm"
              isLoading={createEvidence.isPending}
              onClick={() => fileInputRef.current?.click()}
              className="gap-2"
            >
              <Upload size={14} />
              {t('compliance.evidence.upload')}
            </Button>
          </div>

          {evidencesLoading ? (
            <div className="space-y-2">
              {[0, 1].map((i) => (
                <div key={i} className="h-14 animate-pulse rounded-xl bg-zinc-900" />
              ))}
            </div>
          ) : evidences.length === 0 ? (
            <EmptyState icon={<FileText size={28} />} title={t('compliance.evidence.noEvidence')} />
          ) : (
            <ul className="space-y-2">
              <AnimatePresence initial={false}>
                {evidences.map((evidence) => (
                  <motion.li
                    key={evidence.id}
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                    className="flex items-center justify-between gap-3 rounded-xl border border-zinc-800 bg-zinc-950/60 px-4 py-3"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm text-zinc-100">{evidence.filename}</p>
                      {evidence.description && (
                        <p className="truncate text-xs text-zinc-500">{evidence.description}</p>
                      )}
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      <button
                        title={t('compliance.evidence.download')}
                        onClick={() =>
                          evidence.id &&
                          downloadEvidence.mutate({ id: evidence.id, filename: evidence.filename ?? 'evidence' })
                        }
                        className="rounded-lg p-2 text-zinc-400 hover:bg-white/10 hover:text-white transition-colors"
                      >
                        <Download size={14} />
                      </button>
                      {isAdmin && (
                        <button
                          title={t('compliance.evidence.delete')}
                          onClick={() =>
                            evidence.id &&
                            deleteEvidence.mutate(evidence.id, {
                              onSuccess: () => toast.success(t('compliance.evidence.delete')),
                            })
                          }
                          className="rounded-lg p-2 text-zinc-400 hover:bg-red-500/10 hover:text-red-400 transition-colors"
                        >
                          <Trash2 size={14} />
                        </button>
                      )}
                    </div>
                  </motion.li>
                ))}
              </AnimatePresence>
            </ul>
          )}
        </motion.div>
      )}
    </Drawer>
  );
};
