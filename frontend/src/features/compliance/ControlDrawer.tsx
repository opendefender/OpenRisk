// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Download, FileText, Trash2, Upload } from 'lucide-react';
import { Button, Drawer, Field } from '../../shared/ds';
import { EmptyState } from '../../shared/EmptyState';
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
    <Drawer open={isControlDrawerOpen} onClose={closeControlDrawer} title={control.name}>
      <div className="flex gap-2 border-b border-border-subtle pb-3 mb-6">
        {(['details', 'evidence'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveDrawerTab(tab)}
            className={`rounded-full px-4 py-1.5 text-sm font-medium transition-colors ${
              activeDrawerTab === tab ? 'bg-primary text-text-primary' : 'text-text-secondary hover:bg-surface-1/5'
            }`}
          >
            {t(`compliance.tabs.${tab}`)}
          </button>
        ))}
      </div>

      {activeDrawerTab === 'details' && (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-5">
          <div>
            <label className="text-xs font-semibold uppercase tracking-[0.18em] text-text-muted">
              {t('compliance.referenceCode')}
            </label>
            <p className="mt-1 font-mono text-sm text-text-secondary">{control.reference_code || '—'}</p>
          </div>
          <div>
            <label className="text-xs font-semibold uppercase tracking-[0.18em] text-text-muted">
              {t('compliance.form.description')}
            </label>
            <p className="mt-1 text-sm text-text-secondary">{control.description || '—'}</p>
          </div>
          {control.source_reference && (
            <div>
              <label className="text-xs font-semibold uppercase tracking-[0.18em] text-text-muted">
                {t('compliance.form.sourceReference')}
              </label>
              <p className="mt-1 text-sm italic text-text-secondary">{control.source_reference}</p>
            </div>
          )}
          <div>
            <label className="text-xs font-semibold uppercase tracking-[0.18em] text-text-muted">
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
                      ? 'border-primary bg-primary/20 text-text-primary'
                      : 'border-border-subtle bg-surface-1 text-text-secondary hover:border-border-default'
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
          <div className="rounded-2xl border border-dashed border-border-default p-5">
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
              className="mb-3 w-full rounded-xl border border-border-subtle bg-surface-0 px-3 py-2 text-sm text-text-primary outline-none focus:ring-2 focus:ring-primary/40"
            />
            <Button
              type="button"
              variant="secondary"
              size="sm"
              loading={createEvidence.isPending}
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
                <div key={i} className="h-14 animate-pulse rounded-xl bg-surface-1" />
              ))}
            </div>
          ) : evidences.length === 0 ? (
            <EmptyState icon={FileText} title={t('compliance.evidence.noEvidence')} />
          ) : (
            <ul className="space-y-2">
              <AnimatePresence initial={false}>
                {evidences.map((evidence) => (
                  <motion.li
                    key={evidence.id}
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                    className="flex items-center justify-between gap-3 rounded-xl border border-border-subtle bg-surface-0/60 px-4 py-3"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm text-text-primary">{evidence.filename}</p>
                      {evidence.description && (
                        <p className="truncate text-xs text-text-muted">{evidence.description}</p>
                      )}
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      <button
                        title={t('compliance.evidence.download')}
                        onClick={() =>
                          evidence.id &&
                          downloadEvidence.mutate({ id: evidence.id, filename: evidence.filename ?? 'evidence' })
                        }
                        className="rounded-lg p-2 text-text-secondary hover:bg-surface-1/10 hover:text-text-primary transition-colors"
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
                          className="rounded-lg p-2 text-text-secondary hover:bg-danger/10 hover:text-danger-text transition-colors"
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
