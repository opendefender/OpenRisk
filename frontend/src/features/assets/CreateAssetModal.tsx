// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, Server } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useI18n } from '../../hooks/useI18n';
import { useToast } from '../../hooks/useToast';
import { useAssets } from './useAssets';
import { ASSET_CRITICALITIES, ASSET_TYPES } from '../../types/asset';

const schema = z.object({
  name: z.string().min(2),
  type: z.enum(ASSET_TYPES),
  criticality: z.enum(['LOW', 'MEDIUM', 'HIGH', 'CRITICAL']),
  owner: z.string().optional(),
});
type FormValues = z.infer<typeof schema>;

interface CreateAssetModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const CreateAssetModal = ({ isOpen, onClose }: CreateAssetModalProps) => {
  const { t } = useI18n();
  const toast = useToast();
  const { createAsset } = useAssets();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', type: ASSET_TYPES[0], criticality: 'MEDIUM', owner: '' },
  });

  const handleClose = () => {
    reset();
    onClose();
  };

  const onSubmit = async (values: FormValues) => {
    try {
      await createAsset.mutateAsync(values);
      toast.success(t('assets.createSuccess'));
      handleClose();
    } catch {
      toast.error(t('errors.failedToCreateAsset'));
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
                  <h2 className="text-xl font-semibold">{t('assets.createAsset')}</h2>
                </div>
                <button
                  type="button"
                  onClick={handleClose}
                  className="rounded-full p-2 text-zinc-400 hover:bg-white/10 hover:text-white transition-colors"
                >
                  <X size={20} />
                </button>
              </div>

              <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
                <div className="flex-1 space-y-5 overflow-y-auto px-6 py-6 scrollbar-thin">
                <Input
                  label={t('assets.form.name')}
                  {...register('name')}
                  error={errors.name?.message}
                  disabled={isSubmitting}
                  placeholder="Production-DB-01"
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
                  placeholder="IT Dept"
                />

                </div>

                <div className="flex shrink-0 justify-end gap-3 border-t border-zinc-800 bg-zinc-950/95 px-6 py-4">
                  <Button type="button" variant="ghost" onClick={handleClose}>
                    {t('common.cancel', 'Cancel')}
                  </Button>
                  <Button type="submit" variant="primary" isLoading={isSubmitting}>
                    {t('assets.createAsset')}
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
