// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, Server, History } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useI18n } from '../../hooks/useI18n';
import { useToast } from '../../hooks/useToast';
import { useAssets } from './useAssets';
import { ASSET_CRITICALITIES, ASSET_TYPES, type Asset } from '../../types/asset';

const schema = z.object({
  name: z.string().min(2),
  type: z.enum(ASSET_TYPES),
  criticality: z.enum(['LOW', 'MEDIUM', 'HIGH', 'CRITICAL']),
  owner: z.string().optional(),
});
type FormValues = z.infer<typeof schema>;

interface EditAssetModalProps {
  asset: Asset | undefined;
  onClose: () => void;
  // Opens the asset's change-history drawer (snapshots taken before each
  // update/delete). Optional so callers that don't surface history can omit it.
  onShowHistory?: (assetId: string) => void;
}

export const EditAssetModal = ({ asset, onClose, onShowHistory }: EditAssetModalProps) => {
  const { t } = useI18n();
  const toast = useToast();
  const { updateAsset } = useAssets();
  const isOpen = !!asset;

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) });

  useEffect(() => {
    if (asset) {
      reset({
        name: asset.name ?? '',
        type: asset.type ?? ASSET_TYPES[0],
        criticality: (asset.criticality as FormValues['criticality']) ?? 'MEDIUM',
        owner: asset.owner ?? '',
      });
    }
  }, [asset, reset]);

  const handleClose = () => {
    reset();
    onClose();
  };

  const onSubmit = async (values: FormValues) => {
    if (!asset?.id) return;
    try {
      await updateAsset.mutateAsync({ id: asset.id, payload: values });
      toast.success(t('assets.updateSuccess'));
      handleClose();
    } catch {
      toast.error(t('errors.failedToUpdateAsset'));
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
            onClick={handleClose}
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
          />
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 40 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 40 }}
            transition={{ duration: 0.22, type: 'spring', stiffness: 240 }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4"
          >
            <div className="flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden rounded-3xl border border-zinc-800 bg-zinc-950/95 shadow-2xl shadow-black/40">
              <div className="flex shrink-0 items-center justify-between gap-4 border-b border-zinc-800 px-6 py-5">
                <div className="flex items-center gap-3">
                  <div className="rounded-2xl bg-primary/10 p-2 text-primary">
                    <Server size={20} />
                  </div>
                  <h2 className="text-xl font-semibold">{t('assets.editAsset')}</h2>
                </div>
                <div className="flex items-center gap-1">
                  {onShowHistory && asset?.id && (
                    <button
                      type="button"
                      onClick={() => onShowHistory(asset.id as string)}
                      className="flex items-center gap-1.5 rounded-full px-3 py-2 text-xs font-medium text-zinc-300 hover:bg-white/10 hover:text-white transition-colors"
                      title={t('assets.history')}
                    >
                      <History size={16} />
                      <span className="hidden sm:inline">{t('assets.history')}</span>
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={handleClose}
                    className="rounded-full p-2 text-zinc-400 hover:bg-white/10 hover:text-white transition-colors"
                  >
                    <X size={20} />
                  </button>
                </div>
              </div>

              <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
                <div className="flex-1 space-y-5 overflow-y-auto px-6 py-6 scrollbar-thin">
                <Input
                  label={t('assets.form.name')}
                  {...register('name')}
                  error={errors.name?.message}
                  disabled={isSubmitting}
                  autoFocus
                />

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">
                      {t('assets.form.type')}
                    </label>
                    <select
                      {...register('type')}
                      disabled={isSubmitting}
                      className="w-full h-10 rounded-lg border border-zinc-800 bg-zinc-900/50 px-3 text-sm text-white outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-all"
                    >
                      {ASSET_TYPES.map((type) => (
                        <option key={type} value={type}>
                          {type}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">
                      {t('assets.form.criticality')}
                    </label>
                    <select
                      {...register('criticality')}
                      disabled={isSubmitting}
                      className="w-full h-10 rounded-lg border border-zinc-800 bg-zinc-900/50 px-3 text-sm text-white outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-all"
                    >
                      {ASSET_CRITICALITIES.map((level) => (
                        <option key={level} value={level}>
                          {t(`assets.criticality.${level}`)}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                <Input
                  label={t('assets.form.owner')}
                  {...register('owner')}
                  disabled={isSubmitting}
                />

                </div>

                <div className="flex shrink-0 justify-end gap-3 border-t border-zinc-800 bg-zinc-950/95 px-6 py-4">
                  <Button type="button" variant="ghost" onClick={handleClose}>
                    {t('common.cancel', 'Cancel')}
                  </Button>
                  <Button type="submit" variant="primary" isLoading={isSubmitting}>
                    {t('common.save', 'Save')}
                  </Button>
                </div>
              </form>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};
